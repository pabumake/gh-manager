package theme

import "testing"

func TestPaletteHexValidate(t *testing.T) {
	p := DefaultPaletteHex()
	if p.PaneBorderActive != "#fff67d" || p.PopupBorder != "#fff67d" || p.SelectionBg != "#fff67d" {
		t.Fatalf("default accent colors should be #fff67d")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid default palette: %v", err)
	}
	p.Danger = "red"
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid hex error")
	}
}

func TestResolveForTerminal(t *testing.T) {
	p := DefaultPaletteHex()
	resolvedTrue := ResolveForTerminal(p, true)
	if resolvedTrue.Danger != string(p.Danger) {
		t.Fatalf("expected truecolor to keep hex")
	}
	resolved256 := ResolveForTerminal(p, false)
	if resolved256.Danger == "" || resolved256.Danger[0] == '#' {
		t.Fatalf("expected numeric terminal color for 256 fallback, got %q", resolved256.Danger)
	}
	if resolved256.ColName == "" || resolved256.LogoLine1 == "" {
		t.Fatalf("expected extended fields to resolve")
	}
}

func TestParseThemeFileBackwardCompatibleDefaults(t *testing.T) {
	raw := []byte(`{
		"id":"legacy",
		"name":"Legacy",
		"version":1,
		"colors":{
			"pane_border_active":"#89b4fa",
			"pane_border_inactive":"#585b70",
			"popup_border":"#89b4fa",
			"popup_outer_border":"#11111b",
			"danger":"#f38ba8",
			"danger_text":"#f38ba8",
			"text_primary":"#cdd6f4",
			"text_muted":"#a6adc8",
			"selection_bg":"#89b4fa",
			"selection_fg":"#11111b"
		}
	}`)
	tf, err := ParseThemeFile(raw)
	if err != nil {
		t.Fatalf("expected legacy theme to parse: %v", err)
	}
	if tf.Colors.ColName == "" || tf.Colors.LogoLine1 == "" {
		t.Fatalf("expected missing extended fields to be default-filled")
	}
}
