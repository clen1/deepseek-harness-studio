package main

import (
	"context"
	"os"
	"testing"
)

func TestRealDeployment(t *testing.T) {
	if os.Getenv("HARNESS_STUDIO_INTEGRATION") != "1" {
		t.Skip("set HARNESS_STUDIO_INTEGRATION=1 to run the real download and npm install")
	}
	app := NewApp()
	app.installRoot = t.TempDir()
	config := defaultConfig()
	config.AutoStart = false
	if archive := os.Getenv("HARNESS_STUDIO_NODE_ARCHIVE"); archive != "" {
		t.Log("extracting pre-downloaded Node.js runtime")
		if err := app.extractNode(archive, supportedNodeAsset()); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Log("downloading Node.js runtime")
		if err := app.installNode(context.Background(), config); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("installing DeepSeek Harness")
	if err := app.installHarness(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if got := app.installedVersion(); got != harnessVersion {
		t.Fatalf("installed version %q, expected %q", got, harnessVersion)
	}
}

func TestLiveRegistryRanking(t *testing.T) {
	if os.Getenv("HARNESS_STUDIO_NETWORK_TEST") != "1" {
		t.Skip("set HARNESS_STUDIO_NETWORK_TEST=1 to test live registry ranking")
	}
	app := NewApp()
	app.config = defaultConfig()
	candidates := app.rankedRegistryCandidates(app.config)
	if len(candidates) < 2 {
		t.Fatalf("expected multiple recovery candidates, got %d", len(candidates))
	}
	for index, candidate := range candidates {
		t.Logf("%d. %s %s", index+1, candidate.Name, candidate.URL)
	}
}
