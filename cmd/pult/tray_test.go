package main

import (
	"os"
	"path/filepath"
	"testing"

	"pult/internal/launcher"
)

func TestEnsureConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "pult", "tools.json")
	if err := ensureConfig(path); err != nil {
		t.Fatal(err)
	}
	// The created skeleton must be a valid, empty config.
	cfg, err := launcher.LoadConfig(path)
	if err != nil {
		t.Fatalf("created config does not load: %v", err)
	}
	if len(cfg.Tools) != 0 || len(cfg.Links) != 0 {
		t.Errorf("config not empty: %+v", cfg)
	}

	// An existing file is left alone.
	os.WriteFile(path, []byte(`{"links":[{"name":"x","url":"https://example.com"}]}`), 0o600)
	if err := ensureConfig(path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) == emptyConfig {
		t.Error("existing config was overwritten")
	}
}
