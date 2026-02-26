package tui

type activeMode int

type activePane int

const (
	modeBrowse activeMode = iota
	modeCommands
	modeDetails
)

const (
	paneTable activePane = iota
	paneCommands
)

func modeLabel(m activeMode) string {
	switch m {
	case modeCommands:
		return "Commands"
	case modeDetails:
		return "Details"
	default:
		return "Browse/Select"
	}
}

func paneLabel(p activePane) string {
	if p == paneCommands {
		return "commands"
	}
	return "table"
}

func globalHelp() string {
	return "Global: 1 browse, 2 commands, 3 details, tab switch pane, q quit"
}

func browseHelp() string {
	return "Browse: j/k move, pgup/pgdown page, space toggle, a select filtered, x clear filtered, type filter, backspace delete, n/u/v sort+toggle dir"
}

func commandHelp() string {
	return "Commands: j/k move, enter open/run. Popup forms suspend shortcuts until Enter/Esc."
}
