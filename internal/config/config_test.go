package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Theme.Active != "default" {
		t.Fatalf("unexpected active theme: %q", cfg.Theme.Active)
	}
	p := filepath.Join(home, ".config", "gh-manager", "config.json")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := Default()
	cfg.Theme.Active = "catppuccin-mocha"
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Theme.Active != "catppuccin-mocha" {
		t.Fatalf("active theme mismatch: %q", loaded.Theme.Active)
	}
}
