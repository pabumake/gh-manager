package theme

import (
	"encoding/json"
	"fmt"
	"regexp"
)

type Hex string

type PaletteHex struct {
	PaneBorderActive   Hex `json:"pane_border_active"`
	PaneBorderInactive Hex `json:"pane_border_inactive"`
	PopupBorder        Hex `json:"popup_border"`
	PopupOuterBorder   Hex `json:"popup_outer_border"`
	Danger             Hex `json:"danger"`
	DangerText         Hex `json:"danger_text"`
	TextPrimary        Hex `json:"text_primary"`
	TextMuted          Hex `json:"text_muted"`
	SelectionBg        Hex `json:"selection_bg"`
	SelectionFg        Hex `json:"selection_fg"`
	LogoLine1          Hex `json:"logo_line_1"`
	LogoLine2          Hex `json:"logo_line_2"`
	LogoLine3          Hex `json:"logo_line_3"`
	LogoLine4          Hex `json:"logo_line_4"`
	LogoLine5          Hex `json:"logo_line_5"`
	LogoLine6          Hex `json:"logo_line_6"`
	HeaderText         Hex `json:"header_text"`
	HelpText           Hex `json:"help_text"`
	StatusText         Hex `json:"status_text"`
	TableHeader        Hex `json:"table_header"`
	ColSel             Hex `json:"col_sel"`
	ColName            Hex `json:"col_name"`
	ColVisibility      Hex `json:"col_visibility"`
	ColFork            Hex `json:"col_fork"`
	ColArchived        Hex `json:"col_archived"`
	ColUpdated         Hex `json:"col_updated"`
	ColDescription     Hex `json:"col_description"`
	DetailsLabel       Hex `json:"details_label"`
	DetailsValue       Hex `json:"details_value"`
}

type ThemeFile struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Version int        `json:"version"`
	Colors  PaletteHex `json:"colors"`
}

type PaletteResolved struct {
	PaneBorderActive   string
	PaneBorderInactive string
	PopupBorder        string
	PopupOuterBorder   string
	Danger             string
	DangerText         string
	TextPrimary        string
	TextMuted          string
	SelectionBg        string
	SelectionFg        string
	LogoLine1          string
	LogoLine2          string
	LogoLine3          string
	LogoLine4          string
	LogoLine5          string
	LogoLine6          string
	HeaderText         string
	HelpText           string
	StatusText         string
	TableHeader        string
	ColSel             string
	ColName            string
	ColVisibility      string
	ColFork            string
	ColArchived        string
	ColUpdated         string
	ColDescription     string
	DetailsLabel       string
	DetailsValue       string
}

type ThemeIndex struct {
	Version int               `json:"version"`
	Themes  []ThemeIndexEntry `json:"themes"`
}

type ThemeIndexEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (p PaletteHex) Validate() error {
	fields := map[string]Hex{
		"pane_border_active":   p.PaneBorderActive,
		"pane_border_inactive": p.PaneBorderInactive,
		"popup_border":         p.PopupBorder,
		"popup_outer_border":   p.PopupOuterBorder,
		"danger":               p.Danger,
		"danger_text":          p.DangerText,
		"text_primary":         p.TextPrimary,
		"text_muted":           p.TextMuted,
		"selection_bg":         p.SelectionBg,
		"selection_fg":         p.SelectionFg,
		"logo_line_1":          p.LogoLine1,
		"logo_line_2":          p.LogoLine2,
		"logo_line_3":          p.LogoLine3,
		"logo_line_4":          p.LogoLine4,
		"logo_line_5":          p.LogoLine5,
		"logo_line_6":          p.LogoLine6,
		"header_text":          p.HeaderText,
		"help_text":            p.HelpText,
		"status_text":          p.StatusText,
		"table_header":         p.TableHeader,
		"col_sel":              p.ColSel,
		"col_name":             p.ColName,
		"col_visibility":       p.ColVisibility,
		"col_fork":             p.ColFork,
		"col_archived":         p.ColArchived,
		"col_updated":          p.ColUpdated,
		"col_description":      p.ColDescription,
		"details_label":        p.DetailsLabel,
		"details_value":        p.DetailsValue,
	}
	for key, val := range fields {
		if !hexRe.MatchString(string(val)) {
			return fmt.Errorf("invalid hex color for %s: %q", key, string(val))
		}
	}
	return nil
}

func ParseThemeFile(b []byte) (ThemeFile, error) {
	t := ThemeFile{
		Version: 1,
		Colors:  DefaultPaletteHex(),
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return ThemeFile{}, err
	}
	if t.ID == "" {
		return ThemeFile{}, fmt.Errorf("theme id is required")
	}
	if t.Version == 0 {
		t.Version = 1
	}
	if err := t.Colors.Validate(); err != nil {
		return ThemeFile{}, err
	}
	return t, nil
}

func DefaultPaletteHex() PaletteHex {
	return PaletteHex{
		PaneBorderActive:   "#fff67d",
		PaneBorderInactive: "#585858",
		PopupBorder:        "#fff67d",
		PopupOuterBorder:   "#000000",
		Danger:             "#d70000",
		DangerText:         "#d70000",
		TextPrimary:        "#ddd7c1",
		TextMuted:          "#9e9987",
		SelectionBg:        "#fff67d",
		SelectionFg:        "#000000",
		LogoLine1:          "#fff67d",
		LogoLine2:          "#f6e8a6",
		LogoLine3:          "#e7d896",
		LogoLine4:          "#d6c580",
		LogoLine5:          "#c8b56f",
		LogoLine6:          "#baa260",
		HeaderText:         "#efe8ca",
		HelpText:           "#d8cfaa",
		StatusText:         "#fff67d",
		TableHeader:        "#fff1a6",
		ColSel:             "#fff67d",
		ColName:            "#f7e4a3",
		ColVisibility:      "#9fd0d0",
		ColFork:            "#c7b3e6",
		ColArchived:        "#c9a88f",
		ColUpdated:         "#d6d1b3",
		ColDescription:     "#bdb79f",
		DetailsLabel:       "#dcca91",
		DetailsValue:       "#d8d1b2",
	}
}
