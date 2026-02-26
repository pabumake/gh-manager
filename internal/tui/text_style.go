package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func colorizeDetailLine(line string, theme UITheme) string {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.DetailsValue)).Render(line)
	}
	label := line[:idx+1]
	value := strings.TrimSpace(line[idx+1:])
	labelStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.DetailsLabel)).Render(label)
	valueStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.DetailsValue)).Render(value)
	if value == "" {
		return labelStyled
	}
	return labelStyled + " " + valueStyled
}
