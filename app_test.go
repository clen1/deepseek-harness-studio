package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizePresetConfig(t *testing.T) {
	value := defaultConfig()
	value.RegistryID = "official"
	value.RegistryURL = "https://invalid.example/"
	normalized, err := normalizeConfig(value)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.RegistryURL != "https://registry.npmjs.org/" {
		t.Fatalf("preset URL was not enforced: %s", normalized.RegistryURL)
	}
}

func TestNormalizeCustomConfig(t *testing.T) {
	value := defaultConfig()
	value.RegistryID = "custom"
	value.RegistryURL = "file:///tmp/registry"
	if _, err := normalizeConfig(value); err == nil {
		t.Fatal("expected non-http registry to be rejected")
	}

	value.RegistryURL = "https://packages.example.com/npm"
	value.ProxyMode = "custom"
	value.ProxyURL = "http://127.0.0.1:7890"
	normalized, err := normalizeConfig(value)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.RegistryURL != "https://packages.example.com/npm" {
		t.Fatalf("unexpected registry URL: %s", normalized.RegistryURL)
	}
}

func TestSafeArchivePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "extract")
	if _, err := safeArchivePath(root, "../escape.txt"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	valid, err := safeArchivePath(root, "node/bin/node")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(valid, root) {
		t.Fatalf("valid path escaped root: %s", valid)
	}
}

func TestSupportedNodeAsset(t *testing.T) {
	asset := supportedNodeAsset()
	if (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") &&
		(runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "linux") && asset == "" {
		t.Fatalf("expected asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if asset != "" && !strings.Contains(asset, nodeVersion) {
		t.Fatalf("asset does not contain pinned Node version: %s", asset)
	}
	if asset != "" && nodeChecksums[asset] == "" {
		t.Fatalf("asset does not have a pinned checksum: %s", asset)
	}
}

func TestPreflightLocalStorage(t *testing.T) {
	app := NewApp()
	app.installRoot = filepath.Join(t.TempDir(), "engine")
	if err := app.preflightLocalStorage(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(app.installRoot, ".write-test")); !os.IsNotExist(err) {
		t.Fatal("preflight marker was not cleaned up")
	}
}

func TestInstallAutomaticallySwitchesRegistry(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.installRoot = filepath.Join(root, "engine")
	app.configFile = filepath.Join(root, "config.json")
	config := defaultConfig()
	candidates := []RegistryPreset{
		{ID: "broken", Name: "不可用线路", URL: "https://broken.example/"},
		{ID: "working", Name: "可用线路", URL: "https://working.example/"},
	}
	var calls []string
	attempt := func(_ context.Context, value Config) error {
		calls = append(calls, value.RegistryID)
		if value.RegistryID == "broken" {
			return errors.New("simulated network failure")
		}
		return nil
	}
	if err := app.installHarnessWithFallbackUsing(context.Background(), config, candidates, attempt); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "broken,working" {
		t.Fatalf("unexpected attempt order: %v", calls)
	}
	if app.config.RegistryID != "working" {
		t.Fatalf("successful registry was not persisted: %s", app.config.RegistryID)
	}
}

func TestDirectEnvironmentRemovesProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9999")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9999")
	app := NewApp()
	config := defaultConfig()
	config.ProxyMode = "direct"
	environment := app.commandEnv(config)
	for _, item := range environment {
		upper := strings.ToUpper(item)
		if strings.HasPrefix(upper, "HTTP_PROXY=") || strings.HasPrefix(upper, "HTTPS_PROXY=") {
			t.Fatalf("proxy leaked into direct environment: %s", item)
		}
	}
}

func TestManagedDirectoriesStayUnderConfigRoot(t *testing.T) {
	app := NewApp()
	configRoot, err := os.UserConfigDir()
	if err != nil {
		t.Skip("user config directory unavailable")
	}
	if !strings.HasPrefix(filepath.Clean(app.installRoot), filepath.Clean(configRoot)) {
		t.Fatalf("install root is outside user config: %s", app.installRoot)
	}
}
