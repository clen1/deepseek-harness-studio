package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var nodeChecksums = map[string]string{
	"node-v22.19.0-win-x64.zip":         "ea3fad0e67a991d8477d8c01344b56e69c676ccb733f065b22436994b1253f86",
	"node-v22.19.0-win-arm64.zip":       "e4a7336010d58ff35b53d9dd5869095c56089c70913cf22508cf8183593e56b2",
	"node-v22.19.0-darwin-x64.tar.gz":   "3cfed4795cd97277559763c5f56e711852d2cc2420bda1cea30c8aa9ac77ce0c",
	"node-v22.19.0-darwin-arm64.tar.gz": "c59006db713c770d6ec63ae16cb3edc11f49ee093b5c415d667bb4f436c6526d",
	"node-v22.19.0-linux-x64.tar.gz":    "d36e56998220085782c0ca965f9d51b7726335aed2f5fc7321c6c0ad233aa96d",
	"node-v22.19.0-linux-arm64.tar.gz":  "d32817b937219b8f131a28546035183d79e7fd17a86e38ccb8772901a7cd9009",
}

type nodeDownloadSource struct {
	Name    string
	BaseURL string
}

func supportedNodeAsset() string {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("node-v%s-win-%s.zip", nodeVersion, arch)
	case "darwin":
		return fmt.Sprintf("node-v%s-darwin-%s.tar.gz", nodeVersion, arch)
	case "linux":
		return fmt.Sprintf("node-v%s-linux-%s.tar.gz", nodeVersion, arch)
	default:
		return ""
	}
}

func (a *App) nodeRoot() string {
	return filepath.Join(a.installRoot, "node")
}

func (a *App) nodePath() string {
	pathValue := filepath.Join(a.nodeRoot(), "bin", "node")
	if runtime.GOOS == "windows" {
		pathValue = filepath.Join(a.nodeRoot(), "node.exe")
	}
	if info, err := os.Stat(pathValue); err == nil && !info.IsDir() {
		return pathValue
	}
	return ""
}

func (a *App) npmCLIPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(a.nodeRoot(), "node_modules", "npm", "bin", "npm-cli.js")
	}
	return filepath.Join(a.nodeRoot(), "lib", "node_modules", "npm", "bin", "npm-cli.js")
}

func (a *App) packageRoot() string {
	return filepath.Join(a.installRoot, "package")
}

func (a *App) harnessBin() string {
	return filepath.Join(a.packageRoot(), "node_modules", "@deepseek-ai", "dsh", "lib", "bin.js")
}

func (a *App) installedVersion() string {
	data, err := os.ReadFile(filepath.Join(a.packageRoot(), "node_modules", "@deepseek-ai", "dsh", "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	return pkg.Version
}

func (a *App) preflightLocalStorage() error {
	if err := os.MkdirAll(a.installRoot, 0o755); err != nil {
		return fmt.Errorf("无法创建安装目录: %w", err)
	}
	marker := filepath.Join(a.installRoot, ".write-test")
	if err := os.WriteFile(marker, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("安装目录不可写: %w", err)
	}
	_ = os.Remove(marker)
	return nil
}

func (a *App) rankedRegistryCandidates(config Config) []RegistryPreset {
	results := a.BenchmarkRegistries()
	type scored struct {
		Preset RegistryPreset
		Result RegistryResult
	}
	values := make([]scored, 0, len(registryPresets)+1)
	for _, preset := range registryPresets {
		for _, result := range results {
			if result.ID == preset.ID && result.OK {
				values = append(values, scored{Preset: preset, Result: result})
				break
			}
		}
	}
	if config.RegistryID == "custom" {
		custom := RegistryPreset{ID: "custom", Name: "自定义线路", URL: config.RegistryURL, Description: "用户设置的下载线路"}
		result := a.testRegistry(config, custom)
		if result.OK {
			values = append(values, scored{Preset: custom, Result: result})
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].Result.LatencyMS < values[j].Result.LatencyMS })

	candidates := make([]RegistryPreset, 0, len(registryPresets)+1)
	seen := make(map[string]bool)
	for _, value := range values {
		if !seen[value.Preset.URL] {
			candidates = append(candidates, value.Preset)
			seen[value.Preset.URL] = true
			a.log("网络", "success", fmt.Sprintf("%s 可用 · %d ms", value.Preset.Name, value.Result.LatencyMS))
		}
	}
	// Even when the quick ping is blocked, npm may still be reachable. Keep every
	// untested source as a later recovery candidate.
	fallbacks := append([]RegistryPreset{}, registryPresets...)
	if config.RegistryID == "custom" {
		fallbacks = append([]RegistryPreset{{ID: "custom", Name: "自定义线路", URL: config.RegistryURL}}, fallbacks...)
	}
	for _, preset := range fallbacks {
		if !seen[preset.URL] {
			candidates = append(candidates, preset)
			seen[preset.URL] = true
		}
	}
	return candidates
}

func (a *App) Deploy() error {
	a.mu.Lock()
	if a.job.Phase == "running" {
		a.mu.Unlock()
		return errors.New("已有任务正在运行")
	}
	if supportedNodeAsset() == "" {
		a.mu.Unlock()
		return fmt.Errorf("暂不支持 %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.taskCancel = cancel
	config := a.config
	a.mu.Unlock()

	a.setJob(JobState{Type: "deploy", Phase: "running", Title: "一键部署", Message: "正在准备运行环境", Progress: 2, StartedAt: time.Now().UnixMilli()})
	go a.deployWorker(ctx, config)
	return nil
}

func (a *App) deployWorker(ctx context.Context, config Config) {
	var deployError error
	defer func() {
		a.mu.Lock()
		a.taskCancel = nil
		a.taskCmd = nil
		a.mu.Unlock()
		if deployError != nil {
			phase := "error"
			message := deployError.Error()
			if errors.Is(deployError, context.Canceled) {
				phase = "cancelled"
				message = "部署任务已取消"
			}
			a.setJob(JobState{Type: "deploy", Phase: phase, Title: "部署未完成", Message: message, Progress: 0, FinishedAt: time.Now().UnixMilli()})
			a.log("部署", "error", message)
		} else {
			a.setJob(JobState{Type: "deploy", Phase: "success", Title: "部署完成", Message: "DeepSeek Harness 已可使用", Progress: 100, FinishedAt: time.Now().UnixMilli()})
			a.log("部署", "success", "全部组件部署完成")
			if config.AutoStart {
				if err := a.StartService(config.AutoOpen); err != nil {
					a.log("服务", "error", err.Error())
				}
			}
		}
		a.emitStatus()
	}()
	if err := a.preflightLocalStorage(); err != nil {
		deployError = err
		return
	}
	a.setJob(JobState{Type: "deploy", Phase: "running", Title: "检查下载线路", Message: "正在选择可用且响应最快的线路", Progress: 3, StartedAt: time.Now().UnixMilli()})
	a.log("网络", "info", "安装前自动检查下载线路")
	candidates := a.rankedRegistryCandidates(config)
	if err := ctx.Err(); err != nil {
		deployError = err
		return
	}
	if len(candidates) > 0 {
		config.RegistryID = candidates[0].ID
		config.RegistryURL = candidates[0].URL
		if err := a.persistConfig(config); err != nil {
			a.log("网络", "warning", "最快线路已选择，但保存设置失败: "+err.Error())
		}
		a.log("网络", "success", "自动选择 "+candidates[0].Name)
	}

	if a.nodePath() == "" {
		if err := a.installNode(ctx, config); err != nil {
			deployError = err
			return
		}
	} else {
		a.log("运行时", "success", fmt.Sprintf("复用 Node.js v%s", nodeVersion))
	}
	if err := a.installHarnessWithFallback(ctx, config, candidates); err != nil {
		deployError = err
		return
	}
}

func (a *App) CancelTask() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.job.Phase != "running" || a.taskCancel == nil {
		return errors.New("当前没有可取消的任务")
	}
	a.taskCancel()
	if a.taskCmd != nil && a.taskCmd.Process != nil {
		_ = a.taskCmd.Process.Kill()
	}
	return nil
}

func (a *App) installNode(ctx context.Context, config Config) error {
	asset := supportedNodeAsset()
	a.setJob(JobState{Type: "deploy", Phase: "running", Title: "下载运行时", Message: asset, Progress: 5, StartedAt: time.Now().UnixMilli()})
	a.log("运行时", "info", fmt.Sprintf("下载 Node.js v%s · %s/%s", nodeVersion, runtime.GOOS, runtime.GOARCH))

	client, err := a.httpClient(config, 20*time.Minute)
	if err != nil {
		return err
	}
	checksum := nodeChecksums[asset]
	if checksum == "" {
		return errors.New("当前平台缺少内置运行时校验值")
	}
	downloadPath := filepath.Join(a.installRoot, asset+".download")
	sources := []nodeDownloadSource{
		{Name: "NPMirror", BaseURL: fmt.Sprintf("https://cdn.npmmirror.com/binaries/node/v%s/", nodeVersion)},
		{Name: "Node.js 官方", BaseURL: fmt.Sprintf("https://nodejs.org/dist/v%s/", nodeVersion)},
	}
	if config.RegistryID == "official" {
		sources[0], sources[1] = sources[1], sources[0]
	}
	var failures []string
	for index, source := range sources {
		if index > 0 {
			a.log("运行时", "warning", "上一下载地址失败，自动切换到 "+source.Name)
		}
		a.log("运行时", "info", "尝试使用 "+source.Name)
		downloadErr := downloadFile(ctx, client, source.BaseURL+asset, downloadPath, func(current, total int64) {
			progress := 8.0
			if total > 0 {
				progress = 8 + float64(current)/float64(total)*36
			}
			a.setJob(JobState{Type: "deploy", Phase: "running", Title: "下载运行时", Message: formatBytes(current) + " / " + formatBytes(total), Progress: progress, StartedAt: time.Now().UnixMilli()})
		})
		if downloadErr != nil {
			_ = os.Remove(downloadPath)
			if errors.Is(ctx.Err(), context.Canceled) {
				return context.Canceled
			}
			failures = append(failures, source.Name+": "+downloadErr.Error())
			continue
		}
		actual, hashErr := sha256File(downloadPath)
		if hashErr != nil || !strings.EqualFold(actual, checksum) {
			_ = os.Remove(downloadPath)
			failures = append(failures, source.Name+": SHA-256 校验失败")
			continue
		}
		a.log("运行时", "success", source.Name+" 下载完成，SHA-256 校验通过")
		break
	}
	if _, err := os.Stat(downloadPath); err != nil {
		return fmt.Errorf("所有运行时下载地址均失败: %s", strings.Join(failures, "；"))
	}
	a.setJob(JobState{Type: "deploy", Phase: "running", Title: "解压运行时", Message: "正在写入本地运行环境", Progress: 48, StartedAt: time.Now().UnixMilli()})
	if err := a.extractNode(downloadPath, asset); err != nil {
		_ = os.Remove(downloadPath)
		return fmt.Errorf("解压 Node.js 失败: %w", err)
	}
	_ = os.Remove(downloadPath)
	a.log("运行时", "success", "Node.js 运行时已就绪")
	return nil
}

func fetchChecksum(ctx context.Context, client *http.Client, address, filename string) (string, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 2*1024*1024))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == filename {
			return fields[0], nil
		}
	}
	return "", errors.New("校验文件中没有当前平台记录")
}

func downloadFile(ctx context.Context, client *http.Client, address, destination string, progress func(int64, int64)) error {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 256*1024)
	var written int64
	lastUpdate := time.Now()
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if _, err := file.Write(buffer[:count]); err != nil {
				return err
			}
			written += int64(count)
			if time.Since(lastUpdate) > 120*time.Millisecond {
				progress(written, response.ContentLength)
				lastUpdate = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	progress(written, response.ContentLength)
	return file.Sync()
}

func (a *App) extractNode(archivePath, asset string) error {
	tempRoot := filepath.Join(a.installRoot, ".node-extract")
	_ = os.RemoveAll(tempRoot)
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)
	var err error
	if strings.HasSuffix(asset, ".zip") {
		err = extractZip(archivePath, tempRoot)
	} else {
		err = extractTarGz(archivePath, tempRoot)
	}
	if err != nil {
		return err
	}
	topName := strings.TrimSuffix(strings.TrimSuffix(asset, ".zip"), ".tar.gz")
	source := filepath.Join(tempRoot, topName)
	if _, err := os.Stat(source); err != nil {
		return errors.New("运行时压缩包目录结构无效")
	}
	_ = os.RemoveAll(a.nodeRoot())
	var renameErr error
	for attempt := 0; attempt < 12; attempt++ {
		renameErr = os.Rename(source, a.nodeRoot())
		if renameErr == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return renameErr
}

func safeArchivePath(root, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	target := filepath.Join(root, clean)
	rootWithSep := filepath.Clean(root) + string(os.PathSeparator)
	if target != filepath.Clean(root) && !strings.HasPrefix(target, rootWithSep) {
		return "", errors.New("压缩包包含不安全路径")
	}
	return target, nil
}

func extractZip(source, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		target, err := safeArchivePath(destination, entry.Name)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractTarGz(source, destination string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeArchivePath(destination, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(header.Linkname) || strings.Contains(filepath.Clean(header.Linkname), ".."+string(os.PathSeparator)) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Symlink(filepath.FromSlash(header.Linkname), target)
		}
	}
	return nil
}

func sha256File(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "--"
	}
	const mb = 1024 * 1024
	return strconv.FormatFloat(float64(value)/mb, 'f', 1, 64) + " MB"
}

func (a *App) installHarness(ctx context.Context, config Config) error {
	preset := RegistryPreset{ID: config.RegistryID, Name: config.RegistryID, URL: config.RegistryURL}
	for _, item := range registryPresets {
		if item.ID == config.RegistryID {
			preset = item
			break
		}
	}
	return a.installHarnessWithFallback(ctx, config, []RegistryPreset{preset})
}

func (a *App) installHarnessWithFallback(ctx context.Context, config Config, candidates []RegistryPreset) error {
	return a.installHarnessWithFallbackUsing(ctx, config, candidates, a.installHarnessAttempt)
}

func (a *App) installHarnessWithFallbackUsing(ctx context.Context, config Config, candidates []RegistryPreset, attempt func(context.Context, Config) error) error {
	if len(candidates) == 0 {
		candidates = []RegistryPreset{{ID: config.RegistryID, Name: config.RegistryID, URL: config.RegistryURL}}
	}
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	var failures []string
	for index, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptConfig := config
		attemptConfig.RegistryID = candidate.ID
		attemptConfig.RegistryURL = candidate.URL
		if index > 0 {
			a.setJob(JobState{Type: "deploy", Phase: "running", Title: "自动更换下载线路", Message: "正在改用 " + candidate.Name + " 重试", Progress: 58, StartedAt: time.Now().UnixMilli()})
			a.log("部署", "warning", "上一线路安装失败，自动切换到 "+candidate.Name)
		}
		attemptContext, cancel := context.WithTimeout(ctx, 15*time.Minute)
		err := attempt(attemptContext, attemptConfig)
		cancel()
		if err == nil {
			if persistErr := a.persistConfig(attemptConfig); persistErr != nil {
				a.log("网络", "warning", "保存成功线路失败: "+persistErr.Error())
			}
			if index > 0 {
				a.log("部署", "success", candidate.Name+" 重试成功")
			}
			return nil
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		failures = append(failures, candidate.Name+": "+err.Error())
		a.log("部署", "warning", candidate.Name+" 安装失败，准备尝试下一条线路")
	}
	return fmt.Errorf("所有下载线路均未完成安装，请检查网络或代理设置。详细原因：%s", strings.Join(failures, "；"))
}

func (a *App) installHarnessAttempt(ctx context.Context, config Config) error {
	a.setJob(JobState{Type: "deploy", Phase: "running", Title: "安装 Harness", Message: "正在解析并安装依赖", Progress: 56, StartedAt: time.Now().UnixMilli()})
	if err := os.MkdirAll(a.packageRoot(), 0o755); err != nil {
		return err
	}
	packageJSON := fmt.Sprintf("{\n  \"name\": \"harness-studio-engine\",\n  \"private\": true,\n  \"version\": \"1.0.0\",\n  \"dependencies\": {\n    \"@deepseek-ai/dsh\": \"%s\"\n  }\n}\n", harnessVersion)
	if err := os.WriteFile(filepath.Join(a.packageRoot(), "package.json"), []byte(packageJSON), 0o644); err != nil {
		return err
	}

	env := a.commandEnv(config)
	args := []string{
		a.npmCLIPath(), "install", "--omit=dev", "--no-audit", "--no-fund", "--progress=false", "--loglevel=notice",
		"--replace-registry-host=always", "--fetch-retries=3", "--fetch-retry-factor=2", "--fetch-retry-mintimeout=1000",
		"--fetch-retry-maxtimeout=10000", "--fetch-timeout=120000", "--registry=" + config.RegistryURL,
	}
	command := exec.CommandContext(ctx, a.nodePath(), args...)
	command.Dir = a.packageRoot()
	command.Env = env
	a.attachOutput(command, "npm")
	a.mu.Lock()
	a.taskCmd = command
	a.mu.Unlock()
	a.log("部署", "info", "开始安装 @deepseek-ai/dsh@"+harnessVersion)
	if err := command.Start(); err != nil {
		return err
	}
	if err := command.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("npm 安装失败: %w", err)
	}
	a.setJob(JobState{Type: "deploy", Phase: "running", Title: "完成配置", Message: "正在验证安装结果", Progress: 94, StartedAt: time.Now().UnixMilli()})
	if _, err := os.Stat(a.harnessBin()); a.installedVersion() == "" || err != nil {
		return errors.New("安装完成，但未找到 Harness 启动文件")
	}
	return nil
}

func (a *App) commandEnv(config Config) []string {
	envMap := make(map[string]string)
	for _, item := range os.Environ() {
		if key, value, ok := strings.Cut(item, "="); ok {
			envMap[strings.ToUpper(key)] = value
		}
	}
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NPM_CONFIG_PROXY", "NPM_CONFIG_HTTPS_PROXY"}
	if config.ProxyMode == "direct" {
		for _, key := range proxyKeys {
			delete(envMap, key)
		}
	} else if config.ProxyMode == "custom" {
		envMap["HTTP_PROXY"] = config.ProxyURL
		envMap["HTTPS_PROXY"] = config.ProxyURL
		envMap["NPM_CONFIG_PROXY"] = config.ProxyURL
		envMap["NPM_CONFIG_HTTPS_PROXY"] = config.ProxyURL
	}
	envMap["NPM_CONFIG_REGISTRY"] = config.RegistryURL
	envMap["NPM_CONFIG_REPLACE_REGISTRY_HOST"] = "always"
	values := make([]string, 0, len(envMap))
	for key, value := range envMap {
		values = append(values, key+"="+value)
	}
	return values
}

func (a *App) StartService(openAfterStart bool) error {
	a.mu.Lock()
	if a.serviceCmd != nil && a.serviceCmd.Process != nil {
		a.mu.Unlock()
		if openAfterStart {
			return a.OpenService()
		}
		return nil
	}
	config := a.config
	a.mu.Unlock()
	if a.installedVersion() == "" {
		return errors.New("请先完成一键部署")
	}
	if isPortOpen(servicePort, 300*time.Millisecond) {
		a.log("服务", "warning", fmt.Sprintf("端口 %d 已有服务运行", servicePort))
		if openAfterStart {
			return a.OpenService()
		}
		return nil
	}
	command := exec.Command(a.nodePath(), a.harnessBin(), "web")
	command.Dir = a.packageRoot()
	command.Env = a.commandEnv(config)
	a.attachOutput(command, "Harness")
	if err := command.Start(); err != nil {
		return err
	}
	a.mu.Lock()
	a.serviceCmd = command
	a.mu.Unlock()
	a.log("服务", "info", "DeepSeek Harness 正在启动")
	go func() {
		err := command.Wait()
		a.mu.Lock()
		if a.serviceCmd == command {
			a.serviceCmd = nil
		}
		a.mu.Unlock()
		if err != nil {
			a.log("服务", "warning", "服务进程已退出: "+err.Error())
		} else {
			a.log("服务", "info", "服务已停止")
		}
		a.emitStatus()
	}()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if isPortOpen(servicePort, 250*time.Millisecond) {
			a.log("服务", "success", fmt.Sprintf("服务已就绪 · 127.0.0.1:%d", servicePort))
			a.emitStatus()
			if openAfterStart {
				return a.OpenService()
			}
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("服务启动超时，请查看实时日志")
}

func (a *App) StopService() error {
	a.mu.Lock()
	command := a.serviceCmd
	a.mu.Unlock()
	if command == nil || command.Process == nil {
		return errors.New("当前服务并非由 Studio 启动，无法安全停止")
	}
	a.log("服务", "info", "正在停止服务")
	return command.Process.Kill()
}
