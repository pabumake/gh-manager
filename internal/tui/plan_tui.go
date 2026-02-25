package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gh-manager/internal/planfile"
)

type sortMode int

const (
	sortByName sortMode = iota
	sortByUpdated
	sortByVisibility
)

type columnSpec struct {
	title  string
	min    int
	max    int
	weight int
}

type model struct {
	repos      []planfile.RepoRecord
	selected   map[string]bool
	filtered   []int
	cursor     int
	scroll     int
	filter     string
	sort       sortMode
	showDetail bool
	width      int
	height     int
	quitting   bool
	saved      bool
}

func SelectRepos(repos []planfile.RepoRecord) ([]planfile.RepoRecord, error) {
	m := model{repos: append([]planfile.RepoRecord(nil), repos...), selected: map[string]bool{}}
	m.recompute()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm, ok := finalModel.(model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}
	if !fm.saved {
		return nil, fmt.Errorf("plan canceled")
	}
	out := make([]planfile.RepoRecord, 0)
	for _, r := range fm.repos {
		if fm.selected[r.FullName] {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName < out[j].FullName })
	return out, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
		return m, nil
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureVisible()
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			m.ensureVisible()
		case "pgup":
			m.cursor -= m.tableBodyRows()
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureVisible()
		case "pgdown":
			m.cursor += m.tableBodyRows()
			if m.cursor > len(m.filtered)-1 {
				m.cursor = len(m.filtered) - 1
			}
			m.ensureVisible()
		case " ":
			m.toggleCurrent()
		case "a":
			for _, idx := range m.filtered {
				m.selected[m.repos[idx].FullName] = true
			}
		case "x":
			for _, idx := range m.filtered {
				delete(m.selected, m.repos[idx].FullName)
			}
		case "tab":
			m.sort = (m.sort + 1) % 3
			m.recompute()
		case "enter":
			m.showDetail = !m.showDetail
			m.ensureVisible()
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.recompute()
			}
		case "s":
			m.saved = true
			return m, tea.Quit
		default:
			if len(s) == 1 && s[0] >= 32 && s[0] <= 126 {
				m.filter += s
				m.recompute()
			}
		}
	}
	return m, nil
}

func (m *model) toggleCurrent() {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return
	}
	r := m.repos[m.filtered[m.cursor]]
	if m.selected[r.FullName] {
		delete(m.selected, r.FullName)
	} else {
		m.selected[r.FullName] = true
	}
}

func (m *model) recompute() {
	indexes := make([]int, 0, len(m.repos))
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	for i, r := range m.repos {
		hay := strings.ToLower(strings.Join([]string{r.FullName, r.Name, r.Description, visibilityLabel(r), r.UpdatedAt}, " "))
		if needle == "" || strings.Contains(hay, needle) {
			indexes = append(indexes, i)
		}
	}
	sort.Slice(indexes, func(i, j int) bool {
		a := m.repos[indexes[i]]
		b := m.repos[indexes[j]]
		switch m.sort {
		case sortByUpdated:
			at, _ := time.Parse(time.RFC3339, a.UpdatedAt)
			bt, _ := time.Parse(time.RFC3339, b.UpdatedAt)
			if at.Equal(bt) {
				return a.FullName < b.FullName
			}
			return at.After(bt)
		case sortByVisibility:
			av := visibilityLabel(a)
			bv := visibilityLabel(b)
			if av == bv {
				return a.FullName < b.FullName
			}
			return av < bv
		default:
			return a.FullName < b.FullName
		}
	})
	m.filtered = indexes
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

func (m *model) ensureVisible() {
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.scroll = 0
		return
	}
	rows := m.tableBodyRows()
	if rows < 1 {
		rows = 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+rows {
		m.scroll = m.cursor - rows + 1
	}
	maxScroll := len(m.filtered) - rows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m model) View() string {
	if m.quitting && !m.saved {
		return "Canceled.\n"
	}

	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 32
	}

	title := lipgloss.NewStyle().Bold(true).Render("gh-manager plan")
	help := "Keys: j/k move, pgup/pgdown page, space toggle, a select filtered, x clear filtered, tab sort, enter details, s save, q quit"
	status := fmt.Sprintf("Filter: %s | Sort: %s | Selected: %d | Visible: %d/%d", m.filter, m.sortLabel(), len(m.selected), len(m.filtered), len(m.repos))

	head := []string{title, help, status}
	table := m.renderTable(m.width)

	out := append(head, table)
	if m.showDetail {
		out = append(out, m.renderDetail(m.width))
	}
	return strings.Join(out, "\n") + "\n"
}

func (m model) renderTable(totalWidth int) string {
	cols := []columnSpec{
		{title: "Sel", min: 3, max: 3, weight: 0},
		{title: "Name", min: 16, max: 34, weight: 2},
		{title: "Vis", min: 7, max: 8, weight: 1},
		{title: "Fork", min: 4, max: 5, weight: 1},
		{title: "Arch", min: 4, max: 5, weight: 1},
		{title: "Updated", min: 10, max: 20, weight: 2},
		{title: "Description", min: 16, max: 48, weight: 5},
	}
	widths := allocateColumnWidths(totalWidth-2, cols)

	rowLimit := m.tableBodyRows()
	if rowLimit < 1 {
		rowLimit = 1
	}
	start := m.scroll
	end := start + rowLimit
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	lines := make([]string, 0, rowLimit+6)
	lines = append(lines, drawBorder("┌", "┬", "┐", widths))
	lines = append(lines, drawRow([]string{"Sel", "Name", "Vis", "Fork", "Arch", "Updated", "Description"}, widths, false))
	lines = append(lines, drawBorder("├", "┼", "┤", widths))
	for i := start; i < end; i++ {
		repo := m.repos[m.filtered[i]]
		mark := " "
		if m.selected[repo.FullName] {
			mark = "x"
		}
		line := drawRow([]string{
			mark,
			repo.FullName,
			visibilityLabel(repo),
			fmt.Sprintf("%t", repo.IsFork),
			fmt.Sprintf("%t", repo.IsArchived),
			repo.UpdatedAt,
			repo.Description,
		}, widths, i == m.cursor)
		lines = append(lines, line)
	}
	for i := end; i < start+rowLimit; i++ {
		lines = append(lines, drawRow([]string{"", "", "", "", "", "", ""}, widths, false))
	}
	lines = append(lines, drawBorder("└", "┴", "┘", widths))
	return strings.Join(lines, "\n")
}

func (m model) tableBodyRows() int {
	height := m.height
	if height <= 0 {
		height = 30
	}
	baseOverhead := 7 // title/help/status + table borders/header
	detail := 0
	if m.showDetail {
		detail = m.detailHeight()
	}
	rows := height - baseOverhead - detail
	if rows < 4 {
		rows = 4
	}
	return rows
}

func (m model) detailHeight() int {
	h := m.height / 4
	if h < 7 {
		h = 7
	}
	if h > 11 {
		h = 11
	}
	return h
}

func (m model) renderDetail(totalWidth int) string {
	if len(m.filtered) == 0 {
		return ""
	}
	repo := m.repos[m.filtered[m.cursor]]
	lines := []string{
		"Repo Details",
		fmt.Sprintf("fullName: %s", repo.FullName),
		fmt.Sprintf("owner: %s", repo.Owner),
		fmt.Sprintf("name: %s", repo.Name),
		fmt.Sprintf("visibility: %s", visibilityLabel(repo)),
		fmt.Sprintf("fork: %t | archived: %t", repo.IsFork, repo.IsArchived),
		fmt.Sprintf("updatedAt: %s", repo.UpdatedAt),
		fmt.Sprintf("description: %s", repo.Description),
	}
	compact := m.height < 18
	if compact {
		lines = []string{
			"Repo Details",
			fmt.Sprintf("%s | %s | fork=%t archived=%t", repo.FullName, visibilityLabel(repo), repo.IsFork, repo.IsArchived),
			fmt.Sprintf("updatedAt: %s", repo.UpdatedAt),
		}
	}
	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Width(max(40, totalWidth-2)).
		Render(strings.Join(lines, "\n"))
	return panel
}

func (m model) sortLabel() string {
	switch m.sort {
	case sortByUpdated:
		return "updatedAt desc"
	case sortByVisibility:
		return "visibility"
	default:
		return "name"
	}
}

func visibilityLabel(r planfile.RepoRecord) string {
	if r.IsPrivate {
		return "private"
	}
	return "public"
}

func drawBorder(left, mid, right string, widths []int) string {
	parts := make([]string, 0, len(widths)+2)
	parts = append(parts, left)
	for i, w := range widths {
		parts = append(parts, strings.Repeat("─", w))
		if i != len(widths)-1 {
			parts = append(parts, mid)
		}
	}
	parts = append(parts, right)
	return strings.Join(parts, "")
}

func drawRow(values []string, widths []int, selected bool) string {
	parts := make([]string, 0, len(widths)+2)
	parts = append(parts, "│")
	for i := range widths {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		cell := pad(truncate(v, widths[i]), widths[i])
		if selected {
			cell = lipgloss.NewStyle().Reverse(true).Render(cell)
		}
		parts = append(parts, cell)
		if i != len(widths)-1 {
			parts = append(parts, "│")
		}
	}
	parts = append(parts, "│")
	return strings.Join(parts, "")
}

func allocateColumnWidths(total int, cols []columnSpec) []int {
	if total < 10 {
		total = 10
	}
	sep := len(cols) - 1
	available := total - sep
	widths := make([]int, len(cols))
	used := 0
	for i, c := range cols {
		widths[i] = c.min
		used += c.min
	}
	remaining := available - used
	for remaining > 0 {
		changed := false
		for i, c := range cols {
			if remaining == 0 {
				break
			}
			if widths[i] >= c.max {
				continue
			}
			if c.weight == 0 {
				continue
			}
			widths[i]++
			remaining--
			changed = true
		}
		if !changed {
			break
		}
	}
	return widths
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func pad(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
