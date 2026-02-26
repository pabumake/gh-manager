package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"gh-manager/internal/planfile"
)

type sortField int

type sortDirection int

const (
	sortFieldName sortField = iota
	sortFieldUpdated
	sortFieldVisibility
)

const (
	sortAsc sortDirection = iota
	sortDesc
)

type columnSpec struct {
	title  string
	min    int
	max    int
	weight int
}

type repoTable struct {
	repos    []planfile.RepoRecord
	selected map[string]bool
	filtered []int
	cursor   int
	scroll   int
	filter   string
	sortBy   sortField
	sortDir  sortDirection
	height   int
}

func newRepoTable(repos []planfile.RepoRecord) repoTable {
	t := repoTable{
		repos:    append([]planfile.RepoRecord(nil), repos...),
		selected: map[string]bool{},
		sortBy:   sortFieldName,
		sortDir:  sortAsc,
		height:   32,
	}
	t.recompute()
	return t
}

func (t *repoTable) replaceRepos(repos []planfile.RepoRecord) {
	prevSelected := t.selected
	t.repos = append([]planfile.RepoRecord(nil), repos...)
	t.selected = make(map[string]bool, len(prevSelected))
	for _, r := range t.repos {
		if prevSelected[r.FullName] {
			t.selected[r.FullName] = true
		}
	}
	t.recompute()
}

func (t *repoTable) setHeight(h int) {
	t.height = h
	t.ensureVisible(0)
}

func (t *repoTable) moveCursor(delta int, detailsHeight int) {
	if len(t.filtered) == 0 {
		return
	}
	t.cursor += delta
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor > len(t.filtered)-1 {
		t.cursor = len(t.filtered) - 1
	}
	t.ensureVisible(detailsHeight)
}

func (t *repoTable) pageMove(delta int, detailsHeight int) {
	rows := t.tableBodyRows(detailsHeight)
	t.moveCursor(delta*rows, detailsHeight)
}

func (t *repoTable) toggleCurrent() {
	if len(t.filtered) == 0 || t.cursor < 0 || t.cursor >= len(t.filtered) {
		return
	}
	r := t.repos[t.filtered[t.cursor]]
	if t.selected[r.FullName] {
		delete(t.selected, r.FullName)
	} else {
		t.selected[r.FullName] = true
	}
}

func (t *repoTable) selectAllFiltered() {
	for _, idx := range t.filtered {
		t.selected[t.repos[idx].FullName] = true
	}
}

func (t *repoTable) clearAllFiltered() {
	for _, idx := range t.filtered {
		delete(t.selected, t.repos[idx].FullName)
	}
}

func (t *repoTable) backspaceFilter() {
	if len(t.filter) == 0 {
		return
	}
	t.filter = t.filter[:len(t.filter)-1]
	t.recompute()
}

func (t *repoTable) appendFilterChar(ch string) {
	if len(ch) != 1 {
		return
	}
	if ch[0] < 32 || ch[0] > 126 {
		return
	}
	t.filter += ch
	t.recompute()
}

func (t *repoTable) setSortField(field sortField) {
	if t.sortBy == field {
		if t.sortDir == sortAsc {
			t.sortDir = sortDesc
		} else {
			t.sortDir = sortAsc
		}
	} else {
		t.sortBy = field
		t.sortDir = sortAsc
		if field == sortFieldUpdated {
			t.sortDir = sortDesc
		}
	}
	t.recompute()
}

func (t *repoTable) selectedReposSorted() []planfile.RepoRecord {
	out := make([]planfile.RepoRecord, 0)
	for _, r := range t.repos {
		if t.selected[r.FullName] {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName < out[j].FullName })
	return out
}

func (t *repoTable) currentRepo() (planfile.RepoRecord, bool) {
	if len(t.filtered) == 0 || t.cursor < 0 || t.cursor >= len(t.filtered) {
		return planfile.RepoRecord{}, false
	}
	return t.repos[t.filtered[t.cursor]], true
}

func (t *repoTable) recompute() {
	indexes := make([]int, 0, len(t.repos))
	needle := strings.ToLower(strings.TrimSpace(t.filter))
	for i, r := range t.repos {
		hay := strings.ToLower(strings.Join([]string{r.FullName, r.Name, r.Description, visibilitySortValue(r), visibilityLabel(r), r.UpdatedAt}, " "))
		if needle == "" || strings.Contains(hay, needle) {
			indexes = append(indexes, i)
		}
	}
	sort.Slice(indexes, func(i, j int) bool {
		a := t.repos[indexes[i]]
		b := t.repos[indexes[j]]
		cmp := compareRepos(a, b, t.sortBy)
		if cmp == 0 {
			return a.FullName < b.FullName
		}
		if t.sortDir == sortAsc {
			return cmp < 0
		}
		return cmp > 0
	})
	t.filtered = indexes
	if t.cursor >= len(t.filtered) {
		t.cursor = len(t.filtered) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.ensureVisible(0)
}

func compareRepos(a, b planfile.RepoRecord, field sortField) int {
	switch field {
	case sortFieldUpdated:
		at, aok := parseUpdatedAt(a.UpdatedAt)
		bt, bok := parseUpdatedAt(b.UpdatedAt)
		if !aok && !bok {
			return strings.Compare(a.FullName, b.FullName)
		}
		if !aok {
			return 1
		}
		if !bok {
			return -1
		}
		if at.Equal(bt) {
			return strings.Compare(a.FullName, b.FullName)
		}
		if at.Before(bt) {
			return -1
		}
		return 1
	case sortFieldVisibility:
		av := visibilitySortValue(a)
		bv := visibilitySortValue(b)
		if av == bv {
			return strings.Compare(a.FullName, b.FullName)
		}
		return strings.Compare(av, bv)
	default:
		return strings.Compare(a.FullName, b.FullName)
	}
}

func parseUpdatedAt(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (t *repoTable) ensureVisible(detailsHeight int) {
	if len(t.filtered) == 0 {
		t.cursor = 0
		t.scroll = 0
		return
	}
	rows := t.tableBodyRows(detailsHeight)
	if rows < 1 {
		rows = 1
	}
	if t.cursor < t.scroll {
		t.scroll = t.cursor
	}
	if t.cursor >= t.scroll+rows {
		t.scroll = t.cursor - rows + 1
	}
	maxScroll := len(t.filtered) - rows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scroll > maxScroll {
		t.scroll = maxScroll
	}
	if t.scroll < 0 {
		t.scroll = 0
	}
}

func (t repoTable) tableBodyRows(detailsHeight int) int {
	height := t.height
	if height <= 0 {
		height = 30
	}
	// Table now renders without an outer wrapper. Non-body lines are:
	// top border, header row, header separator, bottom border.
	baseOverhead := 4
	rows := height - baseOverhead - detailsHeight
	if rows < 4 {
		rows = 4
	}
	return rows
}

func (t repoTable) renderTableWithTheme(totalWidth int, focused bool, detailsHeight int, theme UITheme) string {
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
	rowLimit := t.tableBodyRows(detailsHeight)
	if rowLimit < 1 {
		rowLimit = 1
	}
	start := t.scroll
	end := start + rowLimit
	if end > len(t.filtered) {
		end = len(t.filtered)
	}

	lines := make([]string, 0, rowLimit+6)
	lines = append(lines, drawBorder("┌", "┬", "┐", widths))
	lines = append(lines, drawRow(
		[]string{"Sel", "Name", "Vis", "Fork", "Arch", "Updated", "Description"},
		widths,
		false,
		theme,
		true,
	))
	lines = append(lines, drawBorder("├", "┼", "┤", widths))
	for i := start; i < end; i++ {
		repo := t.repos[t.filtered[i]]
		mark := "[ ]"
		if t.selected[repo.FullName] {
			mark = "[x]"
		}
		line := drawRow([]string{
			mark,
			repo.FullName,
			visibilityGlyph(repo),
			forkGlyph(repo.IsFork),
			archiveGlyph(repo.IsArchived),
			repo.UpdatedAt,
			repo.Description,
		}, widths, i == t.cursor, theme, false)
		lines = append(lines, line)
	}
	for i := end; i < start+rowLimit; i++ {
		lines = append(lines, drawRow([]string{"", "", "", "", "", "", ""}, widths, false, theme, false))
	}
	lines = append(lines, drawBorder("└", "┴", "┘", widths))

	_ = focused
	return strings.Join(lines, "\n")
}

func visibilityLabel(r planfile.RepoRecord) string {
	if r.IsPrivate {
		return " private"
	}
	return " public"
}

func visibilityGlyph(r planfile.RepoRecord) string {
	if r.IsPrivate {
		return ""
	}
	return ""
}

func visibilitySortValue(r planfile.RepoRecord) string {
	if r.IsPrivate {
		return "private"
	}
	return "public"
}

func forkGlyph(v bool) string {
	if v {
		return ""
	}
	return ""
}

func archiveGlyph(v bool) string {
	if v {
		return ""
	}
	return ""
}

func sortLabel(field sortField, dir sortDirection) string {
	name := "name"
	switch field {
	case sortFieldUpdated:
		name = "updatedAt"
	case sortFieldVisibility:
		name = "visibility"
	}
	direction := "asc"
	if dir == sortDesc {
		direction = "desc"
	}
	return name + " " + direction
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

func drawRow(values []string, widths []int, selected bool, theme UITheme, isHeader bool) string {
	parts := make([]string, 0, len(widths)+2)
	parts = append(parts, "│")
	columnColors := []string{
		theme.ColSel,
		theme.ColName,
		theme.ColVisibility,
		theme.ColFork,
		theme.ColArchived,
		theme.ColUpdated,
		theme.ColDescription,
	}
	for i := range widths {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		cellText := truncate(v, widths[i])
		cell := pad(cellText, widths[i])
		if i == 0 || i == 2 || i == 3 || i == 4 {
			cell = center(cellText, widths[i])
		}
		cellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(columnColors[i]))
		if isHeader {
			cellStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TableHeader)).Bold(true)
		}
		if selected {
			cellStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.SelectionFg)).
				Background(lipgloss.Color(theme.SelectionBg))
		}
		cell = cellStyle.Render(cell)
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
		return "~"
	}
	return string(r[:max-1]) + "~"
}

func pad(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func center(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	left := (width - len(r)) / 2
	right := width - len(r) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
