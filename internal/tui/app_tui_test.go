package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gh-manager/internal/planfile"
)

func TestModeSwitching(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})

	updated, _ := m.Update(key("2"))
	m2 := updated.(appModel)
	if m2.activeMode != modeCommands {
		t.Fatalf("expected commands mode, got %v", m2.activeMode)
	}
	if m2.activePane != paneCommands {
		t.Fatalf("expected command pane focus")
	}

	updated, _ = m2.Update(key("3"))
	m3 := updated.(appModel)
	if m3.activeMode != modeDetails {
		t.Fatalf("expected details mode, got %v", m3.activeMode)
	}
}

func TestModalSuppressesGlobalShortcuts(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.activeMode = modeCommands
	m.activePane = paneCommands
	m.cmdCursor = 1 // Inspect
	_ = m.openFormForCurrentCommand()
	if !m.modalActive || m.modalKind != modalCommandForm {
		t.Fatalf("expected command form modal open")
	}

	updated, _ := m.Update(key("1"))
	m2 := updated.(appModel)
	if m2.activeMode != modeCommands {
		t.Fatalf("mode should not change while modal active")
	}
	if m2.activePane != paneCommands {
		t.Fatalf("pane should not change while modal active")
	}
	if m2.table.sortBy != sortFieldName || m2.table.sortDir != sortAsc {
		t.Fatalf("sort changed unexpectedly while modal active")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(appModel)
	if m3.activePane != paneCommands {
		t.Fatalf("tab should not switch pane while modal active")
	}
}

func TestModalBlinkAndCancel(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.activeMode = modeCommands
	m.activePane = paneCommands
	m.cmdCursor = 1
	_ = m.openFormForCurrentCommand()
	if !m.cursorVisible {
		t.Fatalf("cursor should start visible")
	}

	updated, _ := m.Update(cursorBlinkMsg{})
	m2 := updated.(appModel)
	if m2.cursorVisible {
		t.Fatalf("cursor should toggle off on blink")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(appModel)
	if m3.modalActive || m3.formOpen {
		t.Fatalf("modal/form should close on esc")
	}
}

func TestModalTextFieldAcceptsJKCharacters(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.activeMode = modeCommands
	m.activePane = paneCommands
	m.cmdCursor = 1 // Inspect
	_ = m.openFormForCurrentCommand()
	if !m.modalActive {
		t.Fatalf("expected modal active")
	}

	updated, _ := m.Update(key("j"))
	m2 := updated.(appModel)
	if got := m2.formFields[m2.formFieldIdx].value; got != "j" {
		t.Fatalf("expected input to contain j, got %q", got)
	}

	updated, _ = m2.Update(key("k"))
	m3 := updated.(appModel)
	if got := m3.formFields[m3.formFieldIdx].value; got != "jk" {
		t.Fatalf("expected input to contain jk, got %q", got)
	}
}

func TestSubmitBackupAllowsBlankPlan(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	var def commandDef
	for _, c := range m.commands {
		if c.name == "Backup" {
			def = c
			break
		}
	}
	m.formCommand = "Backup"
	m.formFields = append([]formField(nil), def.fields...)
	for i := range m.formFields {
		if m.formFields[i].key == "confirm" {
			m.formFields[i].value = "CONFIRM"
		}
	}

	_, err := m.submitCommandForm()
	if err == nil {
		t.Fatal("expected callback unavailable error")
	}
	if err.Error() != "backup callback unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitExecuteAllowsBlankPlan(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	var def commandDef
	for _, c := range m.commands {
		if c.name == "Execute" {
			def = c
			break
		}
	}
	m.formCommand = "Execute"
	m.formFields = append([]formField(nil), def.fields...)
	for i := range m.formFields {
		if m.formFields[i].key == "confirm" {
			m.formFields[i].value = "CONFIRM"
		}
	}

	_, err := m.submitCommandForm()
	if err == nil {
		t.Fatal("expected callback unavailable error")
	}
	if err.Error() != "execute callback unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandResultUsesModalAndDoesNotPersistInBaseView(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.width = 120
	m.height = 36

	updated, _ := m.Update(commandResultMsg{output: "zz_result_token"})
	m2 := updated.(appModel)
	if !m2.modalActive || m2.modalKind != modalResult {
		t.Fatalf("expected result modal to open")
	}
	if !strings.Contains(m2.View(), "zz_result_token") {
		t.Fatalf("expected result token to be visible in modal view")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(appModel)
	if m3.modalActive {
		t.Fatalf("expected result modal to close on enter")
	}
	if strings.Contains(m3.View(), "zz_result_token") {
		t.Fatalf("result token should not remain rendered in base view")
	}
}

func TestModalViewAppliesBackdropScrim(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.width = 120
	m.height = 36
	m.activeMode = modeCommands
	m.activePane = paneCommands
	m.cmdCursor = 1 // Inspect
	_ = m.openFormForCurrentCommand()
	if !m.modalActive {
		t.Fatalf("expected modal open")
	}

	view := m.View()
	if !strings.Contains(view, "Form: Inspect") {
		t.Fatalf("expected modal content in view")
	}
	if !strings.Contains(view, "Mode:") {
		t.Fatalf("expected base UI to remain visible behind modal")
	}
}

func TestCommandResultTriggersRepoRefresh(t *testing.T) {
	initial := []planfile.RepoRecord{{FullName: "alice/one"}}
	refreshed := []planfile.RepoRecord{{FullName: "alice/two"}}
	m := newAppModel(initial, AppCallbacks{
		RefreshRepos: func() ([]planfile.RepoRecord, error) {
			return refreshed, nil
		},
	})
	m.width = 120
	m.height = 36

	updated, cmd := m.Update(commandResultMsg{output: "done", refreshRepos: true})
	m2 := updated.(appModel)
	if cmd == nil {
		t.Fatalf("expected refresh command")
	}
	if m2.status != "Command complete (refreshing repositories...)" {
		t.Fatalf("unexpected status: %q", m2.status)
	}

	msg := cmd()
	updated, _ = m2.Update(msg)
	m3 := updated.(appModel)
	if m3.status != "Command complete (repositories refreshed)" {
		t.Fatalf("unexpected refreshed status: %q", m3.status)
	}
	if len(m3.table.repos) != 1 || m3.table.repos[0].FullName != "alice/two" {
		t.Fatalf("expected refreshed repos in table")
	}
}

func TestDeleteModalRequiresExactRepoName(t *testing.T) {
	repo := planfile.RepoRecord{Owner: "alice", Name: "demo", FullName: "alice/demo"}
	m := newAppModel([]planfile.RepoRecord{repo}, AppCallbacks{
		Delete: func(got planfile.RepoRecord) (string, error) {
			if got.FullName != repo.FullName {
				t.Fatalf("unexpected repo passed to delete callback: %s", got.FullName)
			}
			return "ok", nil
		},
	})
	m.activeMode = modeCommands
	m.activePane = paneCommands
	for i, c := range m.commands {
		if c.name == "Delete" {
			m.cmdCursor = i
			break
		}
	}
	_ = m.openFormForCurrentCommand()
	if !m.modalActive || m.modalKind != modalDeleteConfirm {
		t.Fatalf("expected delete modal")
	}

	updated, _ := m.Update(key("x"))
	m2 := updated.(appModel)
	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(appModel)
	if cmd != nil {
		t.Fatalf("did not expect delete command with wrong confirmation")
	}
	if m3.status == "" || m3.status[:6] != "Error:" {
		t.Fatalf("expected error status for mismatch, got: %q", m3.status)
	}

	m3.deleteInput = "demo"
	updated, cmd = m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m4 := updated.(appModel)
	if cmd == nil {
		t.Fatalf("expected delete command after correct confirmation")
	}
	if m4.busy != true {
		t.Fatalf("expected busy state after submitting delete")
	}
}

func TestSortToggleBySameKey(t *testing.T) {
	repos := []planfile.RepoRecord{{FullName: "b/repo"}, {FullName: "a/repo"}}
	tb := newRepoTable(repos)
	if tb.sortBy != sortFieldName || tb.sortDir != sortAsc {
		t.Fatalf("unexpected initial sort: %v %v", tb.sortBy, tb.sortDir)
	}
	tb.setSortField(sortFieldName)
	if tb.sortDir != sortDesc {
		t.Fatalf("expected desc after toggle, got %v", tb.sortDir)
	}
	tb.setSortField(sortFieldName)
	if tb.sortDir != sortAsc {
		t.Fatalf("expected asc after second toggle, got %v", tb.sortDir)
	}
}

func TestSettingsCommandOpensModal(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.activeMode = modeCommands
	m.activePane = paneCommands
	for i, c := range m.commands {
		if c.name == "Settings" {
			m.cmdCursor = i
			break
		}
	}
	cmd := m.openFormForCurrentCommand()
	if !m.modalActive || m.modalKind != modalSettings {
		t.Fatalf("expected settings modal to open")
	}
	if cmd == nil {
		t.Fatalf("expected settings init command")
	}
	if m.settings.stage != settingsStageConfigHome {
		t.Fatalf("expected configuration home stage")
	}
}

func TestSettingsApplyUpdatesThemeLive(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{
		ThemeApply: func(id string) (UITheme, string, error) {
			return UITheme{PaneBorderActive: "#ff00ff"}, "applied theme: " + id, nil
		},
	})
	m.modalActive = true
	m.modalKind = modalSettings
	m.settings = settingsState{
		stage:       settingsStageThemeLocalList,
		mode:        settingsModeApply,
		localThemes: []string{"catppuccin-mocha"},
		cursor:      0,
	}

	updated, cmd := m.updateSettingsModal("enter")
	if cmd == nil {
		t.Fatalf("expected apply command")
	}
	m2 := updated.(appModel)
	msg := cmd()
	updated, _ = m2.Update(msg)
	m3 := updated.(appModel)
	if m3.theme.PaneBorderActive != "#ff00ff" {
		t.Fatalf("expected live-updated theme, got %q", m3.theme.PaneBorderActive)
	}
}

func TestSettingsUninstallUpdatesThemeLive(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{
		ThemeUninstall: func(id string) (UITheme, string, error) {
			if id != "catppuccin-mocha" {
				t.Fatalf("unexpected theme id for uninstall: %s", id)
			}
			return UITheme{PaneBorderActive: "#fff67d"}, "uninstalled theme: " + id + "; applied theme: default", nil
		},
	})
	m.modalActive = true
	m.modalKind = modalSettings
	m.settings = settingsState{
		stage:       settingsStageThemeLocalList,
		mode:        settingsModeUninstall,
		localThemes: []string{"catppuccin-mocha"},
		cursor:      0,
	}

	updated, cmd := m.updateSettingsModal("enter")
	if cmd == nil {
		t.Fatalf("expected uninstall command")
	}
	m2 := updated.(appModel)
	msg := cmd()
	updated, _ = m2.Update(msg)
	m3 := updated.(appModel)
	if m3.theme.PaneBorderActive != "#fff67d" {
		t.Fatalf("expected live-updated theme, got %q", m3.theme.PaneBorderActive)
	}
}

func TestInitDispatchesUpdateCheck(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{
		UpdateCheck: func() (UpdateInfo, error) {
			return UpdateInfo{CurrentVersion: "v0.1.0", LatestVersion: "v0.1.1", UpdateAvailable: true}, nil
		},
	})
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected update check cmd on init")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	m2 := updated.(appModel)
	if !m2.settings.updateInfo.UpdateAvailable {
		t.Fatalf("expected update available state after init check")
	}
}

func TestDetailPanelBorderIsAlwaysInactive(t *testing.T) {
	repos := []planfile.RepoRecord{{FullName: "alice/repo", Name: "repo", Owner: "alice"}}
	m := newAppModel(repos, AppCallbacks{
		Theme: UITheme{PaneBorderActive: "196", PaneBorderInactive: "240"},
	})
	m.width = 120
	m.height = 36
	m.activePane = paneTable
	outTable := m.renderDetailPanel(40, 12)
	m.activePane = paneCommands
	outCmd := m.renderDetailPanel(40, 12)
	if outTable != outCmd {
		t.Fatalf("expected details panel border to remain inactive regardless of active pane")
	}
}

func TestFormatVersionLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "vdev"},
		{"0.1.0", "v0.1.0"},
		{"v0.1.0", "v0.1.0"},
		{" dev ", "vdev"},
	}
	for _, tc := range cases {
		if got := formatVersionLabel(tc.in); got != tc.want {
			t.Fatalf("formatVersionLabel(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSettingsConfigHomeContainsThemeAndUpdate(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.modalActive = true
	m.modalKind = modalSettings
	m.settings = settingsState{stage: settingsStageConfigHome}
	out := m.renderModalOverlay()
	if !strings.Contains(out, "Theme") || !strings.Contains(out, "Update") {
		t.Fatalf("expected Theme and Update in config home")
	}
}

func TestBannerShowsUpdateIndicator(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{Version: "0.1.0"})
	m.settings.updateInfo = UpdateInfo{
		CurrentVersion:  "v0.1.0",
		LatestVersion:   "v0.1.1",
		UpdateAvailable: true,
	}
	lines := m.renderTopBanner(200)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Update available!") {
		t.Fatalf("expected update indicator in banner")
	}
}

func key(v string) tea.KeyMsg {
	r := []rune(v)
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}
}
