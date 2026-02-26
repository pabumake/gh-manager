package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gh-manager/internal/config"
)

func LoadActivePaletteHex(cfg config.Config) (PaletteHex, string, error) {
	if cfg.Theme.Active == "" || cfg.Theme.Active == "default" {
		return DefaultPaletteHex(), "default", nil
	}
	themesDir, err := config.ThemesDir()
	if err != nil {
		return DefaultPaletteHex(), "default", err
	}
	path := filepath.Join(themesDir, cfg.Theme.Active+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return DefaultPaletteHex(), "default", err
	}
	themeFile, err := ParseThemeFile(b)
	if err != nil {
		return DefaultPaletteHex(), "default", err
	}
	if themeFile.ID != cfg.Theme.Active {
		return DefaultPaletteHex(), "default", fmt.Errorf("theme id mismatch: expected %q got %q", cfg.Theme.Active, themeFile.ID)
	}
	return themeFile.Colors, themeFile.ID, nil
}

func SaveThemeFile(theme ThemeFile) error {
	if err := theme.Colors.Validate(); err != nil {
		return err
	}
	themesDir, err := config.ThemesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(themesDir, theme.ID+".json"), out, 0o644)
}

func ListLocalThemeIDs() ([]string, error) {
	themesDir, err := config.ThemesDir()
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(themesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(ents))
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		ids = append(ids, name[:len(name)-5])
	}
	sort.Strings(ids)
	return ids, nil
}

func RemoveLocalTheme(id string) error {
	if id == "" {
		return fmt.Errorf("theme id is required")
	}
	themesDir, err := config.ThemesDir()
	if err != nil {
		return err
	}
	path := filepath.Join(themesDir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("theme not installed: %s", id)
		}
		return err
	}
	return nil
}
