package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"gh-manager/internal/app"
)

const CurrentVersion = 1

type Config struct {
	Version int         `json:"version"`
	Theme   ThemeConfig `json:"theme"`
}

type ThemeConfig struct {
	Active          string `json:"active"`
	IndexURL        string `json:"index_url"`
	AutoUpdateIndex bool   `json:"auto_update_index"`
}

func Default() Config {
	return Config{
		Version: CurrentVersion,
		Theme: ThemeConfig{
			Active:          "default",
			IndexURL:        "https://raw.githubusercontent.com/pabumake/gh-manager/main/themes/index.json",
			AutoUpdateIndex: true,
		},
	}
}

func EnsureDefaults(cfg *Config) {
	if cfg.Version <= 0 {
		cfg.Version = CurrentVersion
	}
	if cfg.Theme.Active == "" {
		cfg.Theme.Active = "default"
	}
	if cfg.Theme.IndexURL == "" {
		cfg.Theme.IndexURL = Default().Theme.IndexURL
	}
}

func Dir() (string, error) {
	return app.ConfigDir()
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func ThemesDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "themes"), nil
}

func Load() (Config, error) {
	cfgPath, err := Path()
	if err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := Save(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	EnsureDefaults(&cfg)
	return cfg, nil
}

func Save(cfg Config) error {
	EnsureDefaults(&cfg)
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "config.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "config.json"))
}
