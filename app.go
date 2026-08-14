package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	nodeVersion    = "22.19.0"
	harnessVersion = "0.1.0-rc.6"
	servicePort    = 3080
	maxLogEntries  = 800
)

var registryPresets = []RegistryPreset{
	{ID: "npmmirror", Name: "NPMirror", URL: "https://registry.npmmirror.com/", Description: "中国大陆推荐，通常延迟最低", Recommended: true},
	{ID: "official", Name: "npm 官方", URL: "https://registry.npmjs.org/", Description: "官方源，包同步最及时"},
	{ID: "tencent", Name: "腾讯云", URL: "https://mirrors.cloud.tencent.com/npm/", Description: "腾讯云开源镜像站"},
	{ID: "huawei", Name: "华为云", URL: "https://repo.huaweicloud.com/repository/npm/", Description: "华为云开源镜像站"},
}

type App struct {
	ctx         context.Context
	mu          sync.RWMutex
	config      Config
	configFile  string
	installRoot string
	logs        []LogEntry
	logSequence int64
	job         JobState
	serviceCmd  *exec.Cmd
	taskCmd     *exec.Cmd
	taskCancel  context.CancelFunc
}

func NewApp() *App {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		configRoot = os.TempDir()
	}
	root := filepath.Join(configRoot, "HarnessStudio")
	app := &App{
		configFile:  filepath.Join(root, "config.json"),
		installRoot: filepath.Join(root, "engine"),
		job:         JobState{Phase: "idle", Title: "准备就绪", Message: "等待部署或启动操作"},
	}
	app.config = app.loadConfig()
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = os.MkdirAll(a.installRoot, 0o755)
	a.log("Studio", "info", fmt.Sprintf("控制中心已启动 · %s/%s", runtime.GOOS, runtime.GOARCH))
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	if a.taskCancel != nil {
		a.taskCancel()
	}
	service := a.serviceCmd
	a.mu.Unlock()
	if service != nil && service.Process != nil {
		_ = service.Process.Kill()
	}
}

func defaultConfig() Config {
	return Config{
		RegistryID: "npmmirror", RegistryURL: registryPresets[0].URL,
		ProxyMode: "system", AutoOpen: true, AutoStart: true, InstallChannel: "stable",
	}
}

func (a *App) loadConfig() Config {
	value := defaultConfig()
	data, err := os.ReadFile(a.configFile)
	if err == nil {
		_ = json.Unmarshal(data, &value)
	}
	if normalized, err := normalizeConfig(value); err == nil {
		return normalized
	}
	return defaultConfig()
}

func normalizeHTTPURL(raw string, label string, credentialsAllowed bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%s格式无效，仅支持 HTTP 或 HTTPS", label)
	}
	if !credentialsAllowed && parsed.User != nil {
		return "", fmt.Errorf("%s不能包含账号密码", label)
	}
	return parsed.String(), nil
}

func normalizeConfig(input Config) (Config, error) {
	matched := false
	for _, preset := range registryPresets {
		if input.RegistryID == preset.ID {
			input.RegistryURL = preset.URL
			matched = true
			break
		}
	}
	if !matched {
		input.RegistryID = "custom"
		value, err := normalizeHTTPURL(input.RegistryURL, "镜像地址", false)
		if err != nil {
			return Config{}, err
		}
		input.RegistryURL = value
	}
	if input.ProxyMode != "direct" && input.ProxyMode != "system" && input.ProxyMode != "custom" {
		return Config{}, errors.New("代理模式无效")
	}
	if input.ProxyMode == "custom" {
		value, err := normalizeHTTPURL(input.ProxyURL, "代理地址", true)
		if err != nil {
			return Config{}, err
		}
		input.ProxyURL = value
	} else {
		input.ProxyURL = ""
	}
	if input.InstallChannel == "" {
		input.InstallChannel = "stable"
	}
	return input, nil
}

func (a *App) SaveConfig(input Config) (Config, error) {
	next, err := normalizeConfig(input)
	if err != nil {
		return Config{}, err
	}
	if err := a.persistConfig(next); err != nil {
		return Config{}, err
	}
	a.log("网络", "success", "网络设置已保存")
	a.emitStatus()
	return next, nil
}

func (a *App) persistConfig(next Config) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.configFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(a.configFile, data, 0o600); err != nil {
		return err
	}
	a.mu.Lock()
	a.config = next
	a.mu.Unlock()
	return nil
}

func (a *App) GetStatus() Status {
	a.mu.RLock()
	config := a.config
	job := a.job
	logs := append([]LogEntry(nil), a.logs...)
	service := a.serviceCmd
	a.mu.RUnlock()

	serviceState := "stopped"
	servicePID := 0
	if service != nil && service.Process != nil {
		serviceState = "running"
		servicePID = service.Process.Pid
	} else if isPortOpen(servicePort, 250*time.Millisecond) {
		serviceState = "external"
	}
	version := a.installedVersion()
	return Status{
		Installed: version != "", Version: version,
		NodeReady: a.nodePath() != "", NodeVersion: nodeVersion,
		Service: serviceState, ServicePID: servicePID,
		Platform: platformName(), Architecture: runtime.GOARCH,
		InstallPath: a.installRoot, ServiceURL: fmt.Sprintf("http://127.0.0.1:%d", servicePort),
		Job: job, Config: config, Registries: registryPresets, Logs: logs,
		DownloadSupport: supportedNodeAsset() != "",
	}
}

func (a *App) log(source, level, text string) {
	text = stripANSI(strings.TrimSpace(text))
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		a.mu.Lock()
		a.logSequence++
		entry := LogEntry{ID: a.logSequence, Time: time.Now().Format("15:04:05"), Source: source, Level: level, Text: strings.TrimSpace(line)}
		a.logs = append(a.logs, entry)
		if len(a.logs) > maxLogEntries {
			a.logs = append([]LogEntry(nil), a.logs[len(a.logs)-maxLogEntries:]...)
		}
		a.mu.Unlock()
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "studio:log", entry)
		}
	}
}

func stripANSI(value string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range value {
		if r == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func (a *App) setJob(update JobState) {
	a.mu.Lock()
	a.job = update
	a.mu.Unlock()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "studio:job", update)
	}
}

func (a *App) emitStatus() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "studio:status", a.GetStatus())
	}
}

func (a *App) attachOutput(command *exec.Cmd, source string) {
	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()
	consume := func(scanner *bufio.Scanner, level string) {
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			a.log(source, level, scanner.Text())
		}
	}
	go consume(bufio.NewScanner(stdout), "info")
	go consume(bufio.NewScanner(stderr), "warning")
}

func (a *App) httpClient(config Config, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch config.ProxyMode {
	case "direct":
		transport.Proxy = nil
	case "system":
		transport.Proxy = http.ProxyFromEnvironment
	case "custom":
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func (a *App) BenchmarkRegistries() []RegistryResult {
	a.mu.RLock()
	config := a.config
	a.mu.RUnlock()
	results := make([]RegistryResult, len(registryPresets))
	var wait sync.WaitGroup
	for index, preset := range registryPresets {
		wait.Add(1)
		go func(i int, item RegistryPreset) {
			defer wait.Done()
			results[i] = a.testRegistry(config, item)
		}(index, preset)
	}
	wait.Wait()
	return results
}

func (a *App) TestCurrentRegistry() RegistryResult {
	a.mu.RLock()
	config := a.config
	a.mu.RUnlock()
	return a.testRegistry(config, RegistryPreset{ID: config.RegistryID, URL: config.RegistryURL})
}

func (a *App) testRegistry(config Config, preset RegistryPreset) RegistryResult {
	client, err := a.httpClient(config, 8*time.Second)
	if err != nil {
		return RegistryResult{ID: preset.ID, Message: err.Error()}
	}
	started := time.Now()
	request, _ := http.NewRequest(http.MethodGet, strings.TrimRight(preset.URL, "/")+"/-/ping", nil)
	request.Header.Set("User-Agent", "HarnessStudio/2.1")
	response, err := client.Do(request)
	if err != nil {
		return RegistryResult{ID: preset.ID, LatencyMS: time.Since(started).Milliseconds(), Message: err.Error()}
	}
	_ = response.Body.Close()
	ok := response.StatusCode >= 200 && response.StatusCode < 400
	message := response.Status
	return RegistryResult{ID: preset.ID, OK: ok, LatencyMS: time.Since(started).Milliseconds(), Message: message}
}

func (a *App) OpenService() error {
	if !isPortOpen(servicePort, 350*time.Millisecond) {
		return errors.New("服务尚未启动")
	}
	wailsruntime.BrowserOpenURL(a.ctx, fmt.Sprintf("http://127.0.0.1:%d", servicePort))
	return nil
}

func (a *App) OpenInstallDirectory() error {
	if err := os.MkdirAll(a.installRoot, 0o755); err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("explorer.exe", a.installRoot)
	case "darwin":
		command = exec.Command("open", a.installRoot)
	default:
		command = exec.Command("xdg-open", a.installRoot)
	}
	return command.Start()
}

// Uninstall removes only the runtime and Harness package managed by Studio.
// Network preferences are stored outside installRoot and remain available for
// a later reinstall.
func (a *App) Uninstall() error {
	a.mu.Lock()
	if a.job.Phase == "running" {
		a.mu.Unlock()
		return errors.New("已有任务正在运行，请等待完成后再卸载")
	}
	service := a.serviceCmd
	a.mu.Unlock()

	if service == nil && isPortOpen(servicePort, 300*time.Millisecond) {
		return errors.New("检测到 Harness 正在由其他进程运行，请先关闭后再卸载")
	}
	if !a.hasManagedInstallFiles() {
		return errors.New("当前没有可卸载的 Harness 文件")
	}

	startedAt := time.Now().UnixMilli()
	initialJob := JobState{Type: "uninstall", Phase: "running", Title: "一键卸载", Message: "正在停止服务并清理安装文件", Progress: 12, StartedAt: startedAt}
	a.mu.Lock()
	if a.job.Phase == "running" {
		a.mu.Unlock()
		return errors.New("已有任务正在运行，请等待完成后再卸载")
	}
	a.job = initialJob
	a.mu.Unlock()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "studio:job", initialJob)
	}
	go a.uninstallWorker(service, startedAt)
	return nil
}

func (a *App) uninstallWorker(service *exec.Cmd, startedAt int64) {
	if service != nil && service.Process != nil {
		a.log("卸载", "info", "正在停止 Harness 服务")
		if err := service.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			a.finishUninstall(startedAt, fmt.Errorf("无法停止 Harness 服务: %w", err))
			return
		}
		a.mu.Lock()
		if a.serviceCmd == service {
			a.serviceCmd = nil
		}
		a.mu.Unlock()
		deadline := time.Now().Add(5 * time.Second)
		for isPortOpen(servicePort, 120*time.Millisecond) && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if isPortOpen(servicePort, 120*time.Millisecond) {
			a.finishUninstall(startedAt, errors.New("Harness 服务仍在运行，请关闭后重试"))
			return
		}
	}

	a.setJob(JobState{Type: "uninstall", Phase: "running", Title: "清理安装文件", Message: "正在删除运行环境和 Harness 程序文件", Progress: 55, StartedAt: startedAt})
	a.log("卸载", "info", "开始清理 Studio 管理的安装目录")
	if err := removeManagedInstallRoot(a.installRoot); err != nil {
		a.finishUninstall(startedAt, err)
		return
	}
	if err := os.MkdirAll(a.installRoot, 0o755); err != nil {
		a.finishUninstall(startedAt, fmt.Errorf("安装文件已删除，但无法重新创建空目录: %w", err))
		return
	}
	a.finishUninstall(startedAt, nil)
}

func (a *App) finishUninstall(startedAt int64, uninstallError error) {
	if uninstallError != nil {
		a.setJob(JobState{Type: "uninstall", Phase: "error", Title: "卸载未完成", Message: uninstallError.Error(), Progress: 0, StartedAt: startedAt, FinishedAt: time.Now().UnixMilli()})
		a.log("卸载", "error", uninstallError.Error())
	} else {
		a.setJob(JobState{Type: "uninstall", Phase: "success", Title: "卸载完成", Message: "Harness 和运行环境已从这台电脑移除", Progress: 100, StartedAt: startedAt, FinishedAt: time.Now().UnixMilli()})
		a.log("卸载", "success", "Harness 与内置运行环境已安全移除，网络设置已保留")
	}
	a.emitStatus()
}

func (a *App) hasManagedInstallFiles() bool {
	entries, err := os.ReadDir(a.installRoot)
	return err == nil && len(entries) > 0
}

func removeManagedInstallRoot(root string) error {
	clean := filepath.Clean(root)
	absolute, err := filepath.Abs(clean)
	if err != nil {
		return fmt.Errorf("无法确认安装目录: %w", err)
	}
	volumeRoot := filepath.Clean(filepath.VolumeName(absolute) + string(os.PathSeparator))
	if filepath.Base(absolute) != "engine" || absolute == volumeRoot || filepath.Dir(absolute) == volumeRoot {
		return fmt.Errorf("拒绝清理非托管目录: %s", absolute)
	}

	var removeError error
	for attempt := 0; attempt < 4; attempt++ {
		removeError = os.RemoveAll(absolute)
		if removeError == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return fmt.Errorf("部分文件无法删除，请关闭占用文件的程序后重试: %w", removeError)
}

func isPortOpen(port int, timeout time.Duration) bool {
	connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), timeout)
	if err != nil {
		return false
	}
	_ = connection.Close()
	return true
}

func platformName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}
