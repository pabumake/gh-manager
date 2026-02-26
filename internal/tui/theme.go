package tui

import (
	"gh-manager/internal/theme"
)

type UITheme struct {
	PaneBorderActive   string
	PaneBorderInactive string
	PopupBorder        string
	PopupOuterBorder   string
	Danger             string
	DangerText         string
	Success            string
	SuccessText        string
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

type ThemeOption struct {
	ID          string
	Name        string
	Description string
}

func defaultUITheme() UITheme {
	resolved := theme.ResolveForTerminal(theme.DefaultPaletteHex(), theme.DetectTrueColor())
	return uiThemeFromResolved(resolved)
}

func uiThemeFromResolved(r theme.PaletteResolved) UITheme {
	return UITheme{
		PaneBorderActive:   r.PaneBorderActive,
		PaneBorderInactive: r.PaneBorderInactive,
		PopupBorder:        r.PopupBorder,
		PopupOuterBorder:   r.PopupOuterBorder,
		Danger:             r.Danger,
		DangerText:         r.DangerText,
		Success:            r.Success,
		SuccessText:        r.SuccessText,
		TextPrimary:        r.TextPrimary,
		TextMuted:          r.TextMuted,
		SelectionBg:        r.SelectionBg,
		SelectionFg:        r.SelectionFg,
		LogoLine1:          r.LogoLine1,
		LogoLine2:          r.LogoLine2,
		LogoLine3:          r.LogoLine3,
		LogoLine4:          r.LogoLine4,
		LogoLine5:          r.LogoLine5,
		LogoLine6:          r.LogoLine6,
		HeaderText:         r.HeaderText,
		HelpText:           r.HelpText,
		StatusText:         r.StatusText,
		TableHeader:        r.TableHeader,
		ColSel:             r.ColSel,
		ColName:            r.ColName,
		ColVisibility:      r.ColVisibility,
		ColFork:            r.ColFork,
		ColArchived:        r.ColArchived,
		ColUpdated:         r.ColUpdated,
		ColDescription:     r.ColDescription,
		DetailsLabel:       r.DetailsLabel,
		DetailsValue:       r.DetailsValue,
	}
}

func (t UITheme) withDefaults() UITheme {
	d := defaultUITheme()
	if t.PaneBorderActive == "" {
		t.PaneBorderActive = d.PaneBorderActive
	}
	if t.PaneBorderInactive == "" {
		t.PaneBorderInactive = d.PaneBorderInactive
	}
	if t.PopupBorder == "" {
		t.PopupBorder = d.PopupBorder
	}
	if t.PopupOuterBorder == "" {
		t.PopupOuterBorder = d.PopupOuterBorder
	}
	if t.Danger == "" {
		t.Danger = d.Danger
	}
	if t.DangerText == "" {
		t.DangerText = d.DangerText
	}
	if t.Success == "" {
		t.Success = d.Success
	}
	if t.SuccessText == "" {
		t.SuccessText = d.SuccessText
	}
	if t.TextPrimary == "" {
		t.TextPrimary = d.TextPrimary
	}
	if t.TextMuted == "" {
		t.TextMuted = d.TextMuted
	}
	if t.SelectionBg == "" {
		t.SelectionBg = d.SelectionBg
	}
	if t.SelectionFg == "" {
		t.SelectionFg = d.SelectionFg
	}
	if t.LogoLine1 == "" {
		t.LogoLine1 = d.LogoLine1
	}
	if t.LogoLine2 == "" {
		t.LogoLine2 = d.LogoLine2
	}
	if t.LogoLine3 == "" {
		t.LogoLine3 = d.LogoLine3
	}
	if t.LogoLine4 == "" {
		t.LogoLine4 = d.LogoLine4
	}
	if t.LogoLine5 == "" {
		t.LogoLine5 = d.LogoLine5
	}
	if t.LogoLine6 == "" {
		t.LogoLine6 = d.LogoLine6
	}
	if t.HeaderText == "" {
		t.HeaderText = d.HeaderText
	}
	if t.HelpText == "" {
		t.HelpText = d.HelpText
	}
	if t.StatusText == "" {
		t.StatusText = d.StatusText
	}
	if t.TableHeader == "" {
		t.TableHeader = d.TableHeader
	}
	if t.ColSel == "" {
		t.ColSel = d.ColSel
	}
	if t.ColName == "" {
		t.ColName = d.ColName
	}
	if t.ColVisibility == "" {
		t.ColVisibility = d.ColVisibility
	}
	if t.ColFork == "" {
		t.ColFork = d.ColFork
	}
	if t.ColArchived == "" {
		t.ColArchived = d.ColArchived
	}
	if t.ColUpdated == "" {
		t.ColUpdated = d.ColUpdated
	}
	if t.ColDescription == "" {
		t.ColDescription = d.ColDescription
	}
	if t.DetailsLabel == "" {
		t.DetailsLabel = d.DetailsLabel
	}
	if t.DetailsValue == "" {
		t.DetailsValue = d.DetailsValue
	}
	return t
}
