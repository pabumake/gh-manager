package theme

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/muesli/termenv"
)

type rgb struct {
	r int
	g int
	b int
}

func DetectTrueColor() bool {
	profile := termenv.EnvColorProfile()
	if profile == termenv.TrueColor {
		return true
	}
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if strings.Contains(ct, "truecolor") || strings.Contains(ct, "24bit") {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "direct") || strings.Contains(term, "truecolor") {
		return true
	}
	return false
}

func ResolveForTerminal(p PaletteHex, trueColor bool) PaletteResolved {
	resolve := func(h Hex) string {
		if trueColor {
			return string(h)
		}
		rgbVal, err := parseHex(h)
		if err != nil {
			return "7"
		}
		return strconv.Itoa(nearestXterm256(rgbVal))
	}
	return PaletteResolved{
		PaneBorderActive:   resolve(p.PaneBorderActive),
		PaneBorderInactive: resolve(p.PaneBorderInactive),
		PopupBorder:        resolve(p.PopupBorder),
		PopupOuterBorder:   resolve(p.PopupOuterBorder),
		Danger:             resolve(p.Danger),
		DangerText:         resolve(p.DangerText),
		TextPrimary:        resolve(p.TextPrimary),
		TextMuted:          resolve(p.TextMuted),
		SelectionBg:        resolve(p.SelectionBg),
		SelectionFg:        resolve(p.SelectionFg),
		LogoLine1:          resolve(p.LogoLine1),
		LogoLine2:          resolve(p.LogoLine2),
		LogoLine3:          resolve(p.LogoLine3),
		LogoLine4:          resolve(p.LogoLine4),
		LogoLine5:          resolve(p.LogoLine5),
		LogoLine6:          resolve(p.LogoLine6),
		HeaderText:         resolve(p.HeaderText),
		HelpText:           resolve(p.HelpText),
		StatusText:         resolve(p.StatusText),
		TableHeader:        resolve(p.TableHeader),
		ColSel:             resolve(p.ColSel),
		ColName:            resolve(p.ColName),
		ColVisibility:      resolve(p.ColVisibility),
		ColFork:            resolve(p.ColFork),
		ColArchived:        resolve(p.ColArchived),
		ColUpdated:         resolve(p.ColUpdated),
		ColDescription:     resolve(p.ColDescription),
		DetailsLabel:       resolve(p.DetailsLabel),
		DetailsValue:       resolve(p.DetailsValue),
	}
}

func parseHex(h Hex) (rgb, error) {
	s := string(h)
	if !hexRe.MatchString(s) {
		return rgb{}, fmt.Errorf("invalid hex")
	}
	r, _ := strconv.ParseInt(s[1:3], 16, 64)
	g, _ := strconv.ParseInt(s[3:5], 16, 64)
	b, _ := strconv.ParseInt(s[5:7], 16, 64)
	return rgb{r: int(r), g: int(g), b: int(b)}, nil
}

func nearestXterm256(c rgb) int {
	bestIdx := 0
	bestDist := int(^uint(0) >> 1)
	for i := 0; i < 256; i++ {
		p := xterm256RGB(i)
		d := sq(c.r-p.r) + sq(c.g-p.g) + sq(c.b-p.b)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

func sq(v int) int { return v * v }

func xterm256RGB(idx int) rgb {
	base16 := []rgb{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	if idx < 16 {
		return base16[idx]
	}
	if idx >= 232 {
		v := 8 + (idx-232)*10
		return rgb{v, v, v}
	}
	i := idx - 16
	r := i / 36
	g := (i / 6) % 6
	b := i % 6
	levels := []int{0, 95, 135, 175, 215, 255}
	return rgb{levels[r], levels[g], levels[b]}
}
