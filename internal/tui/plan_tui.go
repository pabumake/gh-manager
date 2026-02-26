package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gh-manager/internal/planfile"
)

type planModel struct {
	table      repoTable
	showDetail bool
	width      int
	height     int
	quitting   bool
	saved      bool
	theme      UITheme
}

func SelectRepos(repos []planfile.RepoRecord) ([]planfile.RepoRecord, error) {
	return SelectReposWithTheme(repos, UITheme{})
}

func SelectReposWithTheme(repos []planfile.RepoRecord, theme UITheme) ([]planfile.RepoRecord, error) {
	m := planModel{table: newRepoTable(repos), theme: theme.withDefaults()}
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm, ok := finalModel.(planModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}
	if !fm.saved {
		return nil, fmt.Errorf("plan canceled")
	}
	return fm.table.selectedReposSorted(), nil
}

func (m planModel) Init() tea.Cmd { return nil }

func (m planModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.setHeight(m.height)
		m.table.ensureVisible(m.detailsHeight())
		return m, nil
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			m.table.moveCursor(-1, m.detailsHeight())
		case "down", "j":
			m.table.moveCursor(1, m.detailsHeight())
		case "pgup":
			m.table.pageMove(-1, m.detailsHeight())
		case "pgdown":
			m.table.pageMove(1, m.detailsHeight())
		case " ":
			m.table.toggleCurrent()
		case "a":
			m.table.selectAllFiltered()
		case "x":
			m.table.clearAllFiltered()
		case "n":
			m.table.setSortField(sortFieldName)
		case "u":
			m.table.setSortField(sortFieldUpdated)
		case "v":
			m.table.setSortField(sortFieldVisibility)
		case "enter":
			m.showDetail = !m.showDetail
			m.table.ensureVisible(m.detailsHeight())
		case "backspace":
			m.table.backspaceFilter()
		case "s":
			m.saved = true
			return m, tea.Quit
		default:
			m.table.appendFilterChar(s)
		}
	}
	return m, nil
}

func (m planModel) View() string {
	if m.quitting && !m.saved {
		return "Canceled.\n"
	}
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 32
	}
	m.table.setHeight(m.height)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.HeaderText)).Render("gh-manager plan")
	help := "Keys: j/k move, pgup/pgdown page, space toggle, a select filtered, x clear filtered, n/u/v sort+toggle dir, enter details, s save, q quit"
	status := fmt.Sprintf("Filter: %s | Sort: %s | Selected: %d | Visible: %d/%d", m.table.filter, sortLabel(m.table.sortBy, m.table.sortDir), len(m.table.selected), len(m.table.filtered), len(m.table.repos))
	help = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.HelpText)).Render(help)
	status = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusText)).Render(status)

	head := []string{title, help, status}
	table := m.table.renderTableWithTheme(m.width, true, m.detailsHeight(), m.theme)
	out := append(head, table)
	if m.showDetail {
		out = append(out, m.renderDetail(m.width))
	}
	return strings.Join(out, "\n") + "\n"
}

func (m planModel) detailsHeight() int {
	if !m.showDetail {
		return 0
	}
	h := m.height / 4
	if h < 7 {
		h = 7
	}
	if h > 11 {
		h = 11
	}
	return h
}

func (m planModel) renderDetail(totalWidth int) string {
	repo, ok := m.table.currentRepo()
	if !ok {
		return ""
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DetailsLabel)).Render("Repo Details"),
		colorizeDetailLine(fmt.Sprintf("fullName: %s", repo.FullName), m.theme),
		colorizeDetailLine(fmt.Sprintf("owner: %s", repo.Owner), m.theme),
		colorizeDetailLine(fmt.Sprintf("name: %s", repo.Name), m.theme),
		colorizeDetailLine(fmt.Sprintf("visibility: %s", visibilityLabel(repo)), m.theme),
		colorizeDetailLine(fmt.Sprintf("fork: %t | archived: %t", repo.IsFork, repo.IsArchived), m.theme),
		colorizeDetailLine(fmt.Sprintf("updatedAt: %s", repo.UpdatedAt), m.theme),
		colorizeDetailLine(fmt.Sprintf("description: %s", repo.Description), m.theme),
	}
	if m.height < 18 {
		lines = []string{
			"Repo Details",
			fmt.Sprintf("%s | %s | fork=%t archived=%t", repo.FullName, visibilityLabel(repo), repo.IsFork, repo.IsArchived),
			fmt.Sprintf("updatedAt: %s", repo.UpdatedAt),
		}
	}
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.PaneBorderActive)).
		Padding(0, 1).
		Width(max(40, totalWidth-2)).
		Render(strings.Join(lines, "\n"))
}
