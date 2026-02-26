package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	restorepkg "gh-manager/internal/restore"
)

type restoreStage int

const (
	restoreStageNone restoreStage = iota
	restoreStageBrowseArchive
	restoreStageSelectRepo
	restoreStageAskUseOriginal
	restoreStageInputNewName
)

type restoreState struct {
	active bool
	stage  restoreStage

	browserDir     string
	browserCursor  int
	browserItems   []browserItem
	browserHScroll int

	archiveRoot string
	repoCursor  int
	repos       []restoreRepoItem
	selected    restoreRepoItem
	repoHScroll int

	promptInput string
}

type browserItem struct {
	label      string
	path       string
	isDir      bool
	selectRoot bool
}

type restoreRepoItem struct {
	fullName   string
	sourceKind string
	sourcePath string
}

func (m *appModel) startRestoreFlow() tea.Cmd {
	m.restoreState = restoreState{
		active: true,
		stage:  restoreStageBrowseArchive,
	}
	base := strings.TrimSpace(m.callbacks.RestoreDefaultArchiveDir)
	if base == "" {
		base = filepath.Join(userHomeDirOr("."), "Documents")
	}
	if _, err := os.Stat(base); err != nil {
		base = "."
	}
	m.restoreState.browserDir = base
	m.loadBrowserItems()
	m.status = "Restore: select archive root"
	return m.openRestoreBrowseModal()
}

func userHomeDirOr(fallback string) string {
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return fallback
	}
	return h
}

func (m *appModel) loadBrowserItems() {
	dir := m.restoreState.browserDir
	items := make([]browserItem, 0, 64)
	if restorepkg.IsArchiveRoot(dir) {
		items = append(items, browserItem{label: "[Use this archive]", path: dir, selectRoot: true})
	}
	parent := filepath.Dir(dir)
	if parent != dir {
		items = append(items, browserItem{label: "..", path: parent, isDir: true})
	}
	ents, err := os.ReadDir(dir)
	if err == nil {
		for _, ent := range ents {
			if !ent.IsDir() {
				continue
			}
			p := filepath.Join(dir, ent.Name())
			label := ent.Name()
			if restorepkg.IsArchiveRoot(p) {
				label += " [archive]"
			}
			items = append(items, browserItem{label: label, path: p, isDir: true})
		}
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].label) < strings.ToLower(items[j].label) })
	m.restoreState.browserItems = items
	if m.restoreState.browserCursor >= len(items) {
		m.restoreState.browserCursor = len(items) - 1
	}
	if m.restoreState.browserCursor < 0 {
		m.restoreState.browserCursor = 0
	}
}

func (m appModel) updateRestoreFlow(key string) (tea.Model, tea.Cmd) {
	s := m.restoreState
	switch s.stage {
	case restoreStageBrowseArchive:
		switch key {
		case "esc":
			m.restoreState = restoreState{}
			m.closeModal()
			m.status = "Restore canceled"
			return m, nil
		case "up", "k":
			if s.browserCursor > 0 {
				s.browserCursor--
			}
		case "down", "j":
			if s.browserCursor < len(s.browserItems)-1 {
				s.browserCursor++
			}
		case "left", "h":
			if s.browserHScroll > 0 {
				s.browserHScroll--
			}
		case "right", "l":
			s.browserHScroll++
		case "backspace":
			s.browserDir = filepath.Dir(s.browserDir)
			m.restoreState = s
			m.loadBrowserItems()
			m.status = "Restore: browse archive folders"
			return m, nil
		case "enter":
			if len(s.browserItems) == 0 {
				m.status = "No directories found"
				return m, nil
			}
			it := s.browserItems[s.browserCursor]
			if it.selectRoot || restorepkg.IsArchiveRoot(it.path) {
				entries, err := restorepkg.LoadIndex(it.path)
				if err != nil {
					m.status = "Error: " + err.Error()
					return m, nil
				}
				repos := make([]restoreRepoItem, 0, len(entries))
				for _, e := range entries {
					src, ok := restorepkg.PreferredSource(e)
					if !ok {
						continue
					}
					repos = append(repos, restoreRepoItem{fullName: e.FullName, sourceKind: src.Kind, sourcePath: src.Path})
				}
				if len(repos) == 0 {
					m.status = "No restorable repos found in archive"
					return m, nil
				}
				s.archiveRoot = it.path
				s.repos = repos
				s.repoCursor = 0
				s.stage = restoreStageSelectRepo
				m.status = "Restore: select repository"
				m.restoreState = s
				return m, m.openRestoreSelectRepoModal()
			}
			if it.isDir {
				s.browserDir = it.path
				m.restoreState = s
				m.loadBrowserItems()
				m.status = "Restore: browse archive folders"
				return m, nil
			}
		}
		m.restoreState = s
		return m, nil
	case restoreStageSelectRepo:
		switch key {
		case "esc":
			s.stage = restoreStageBrowseArchive
			m.status = "Restore: select archive root"
			m.restoreState = s
			return m, m.openRestoreBrowseModal()
		case "up", "k":
			if s.repoCursor > 0 {
				s.repoCursor--
			}
		case "down", "j":
			if s.repoCursor < len(s.repos)-1 {
				s.repoCursor++
			}
		case "left", "h":
			if s.repoHScroll > 0 {
				s.repoHScroll--
			}
		case "right", "l":
			s.repoHScroll++
		case "enter":
			if len(s.repos) == 0 {
				break
			}
			s.selected = s.repos[s.repoCursor]
			s.promptInput = ""
			s.stage = restoreStageAskUseOriginal
			m.status = "Use original name? (yes/no)"
			m.restoreState = s
			return m, m.openRestoreYesNoModal()
		}
		m.restoreState = s
		return m, nil
	case restoreStageAskUseOriginal:
		// Input is handled in modal key routing.
		m.restoreState = s
		return m, nil
	case restoreStageInputNewName:
		// Input is handled in modal key routing.
		m.restoreState = s
		return m, nil
	default:
		return m, nil
	}
}

func isPrintableKey(k string) bool {
	return len(k) == 1 && k[0] >= 32 && k[0] <= 126
}

func parseYesNo(v string) (bool, bool) {
	n := strings.ToLower(strings.TrimSpace(v))
	switch n {
	case "y", "yes":
		return true, true
	case "n", "no":
		return false, true
	default:
		return false, false
	}
}

func originalRepoName(fullName string) string {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullName
}

func (m appModel) submitRestore(targetName string) (tea.Cmd, error) {
	if m.callbacks.Restore == nil {
		return nil, fmt.Errorf("restore callback unavailable")
	}
	s := m.restoreState
	owner := strings.TrimSpace(m.callbacks.RestoreDefaultOwner)
	if owner == "" {
		return nil, fmt.Errorf("restore owner is empty")
	}
	req := RestoreRequest{
		ArchiveRoot:      s.archiveRoot,
		RepoFullName:     s.selected.fullName,
		SourceKind:       s.selected.sourceKind,
		SourcePath:       s.selected.sourcePath,
		TargetOwner:      owner,
		TargetName:       targetName,
		TargetVisibility: "private",
	}
	return func() tea.Msg {
		out, err := m.callbacks.Restore(req)
		return commandResultMsg{output: out, err: err, refreshRepos: err == nil}
	}, nil
}

func (m appModel) renderRestorePanel(width, height int) string {
	s := m.restoreState
	lines := []string{"Restore"}
	switch s.stage {
	case restoreStageBrowseArchive:
		lines = append(lines,
			"Select archive root:",
			"dir: "+s.browserDir,
		)
		for i, it := range s.browserItems {
			prefix := "  "
			if i == s.browserCursor {
				prefix = "> "
			}
			lines = append(lines, prefix+it.label)
		}
		lines = append(lines, "", "Keys: j/k move, enter select/open, backspace parent, esc cancel")
	case restoreStageSelectRepo:
		lines = append(lines,
			"Archive: "+s.archiveRoot,
			"Select repository:",
		)
		for i, r := range s.repos {
			prefix := "  "
			if i == s.repoCursor {
				prefix = "> "
			}
			label := fmt.Sprintf("%s (%s)", r.fullName, r.sourceKind)
			lines = append(lines, prefix+label)
		}
		lines = append(lines, "", "Keys: j/k move, enter choose, esc back")
	case restoreStageAskUseOriginal, restoreStageInputNewName:
		lines = append(lines,
			"Archive: "+s.archiveRoot,
			"Repo: "+s.selected.fullName,
			"Source: "+s.selected.sourceKind,
			"",
			"Popup input is active.",
		)
	}
	lines = append(lines, "", "Status: "+m.status)
	lines = fitAndWrapLines(lines, innerHeight(height), panelInnerWidth(width))

	style := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).Padding(0, 1)
	if m.activePane == paneCommands {
		style = style.BorderForeground(lipgloss.Color(m.theme.PaneBorderActive))
	} else {
		style = style.BorderForeground(lipgloss.Color(m.theme.PaneBorderInactive))
	}
	return style.Width(width).Render(strings.Join(lines, "\n"))
}
