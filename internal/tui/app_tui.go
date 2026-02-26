package tui

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gh-manager/internal/planfile"
	restorepkg "gh-manager/internal/restore"
)

type AppCallbacks struct {
	Plan            func(selected []planfile.RepoRecord, outPath string) (string, error)
	Inspect         func(planPath string) (string, error)
	Backup          func(planPath, backupLocation string, dryRun bool, confirmation string, selected []planfile.RepoRecord) (string, error)
	Execute         func(planPath, backupLocation string, dryRun bool, confirmation string, selected []planfile.RepoRecord) (string, error)
	Restore         func(req RestoreRequest) (string, error)
	Delete          func(repo planfile.RepoRecord) (string, error)
	ThemeCurrent    func() (string, error)
	ThemeListLocal  func() ([]string, string, error)
	ThemeListRemote func() ([]ThemeOption, string, error)
	ThemeInstall    func(id string) (string, error)
	ThemeApply      func(id string) (UITheme, string, error)
	ThemeUninstall  func(id string) (UITheme, string, error)
	UpdateCheck     func() (UpdateInfo, error)
	UpdateRun       func() (string, error)
	// RefreshRepos reloads the repo list after mutating operations (execute/restore/delete).
	RefreshRepos func() ([]planfile.RepoRecord, error)

	RestoreDefaultOwner      string
	RestoreDefaultArchiveDir string
	Version                  string
	Theme                    UITheme
}

type RestoreRequest struct {
	ArchiveRoot      string
	RepoFullName     string
	SourceKind       string
	SourcePath       string
	TargetOwner      string
	TargetName       string
	TargetVisibility string
}

type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
	CheckedAt       time.Time
	Source          string
	Error           string
}

type commandDef struct {
	name   string
	icon   string
	desc   string
	fields []formField
}

type fieldKind int

const (
	fieldText fieldKind = iota
	fieldBool
)

type formField struct {
	key         string
	label       string
	kind        fieldKind
	value       string
	boolValue   bool
	required    bool
	placeholder string
}

type modalKind int

const (
	modalNone modalKind = iota
	modalCommandForm
	modalRestoreBrowse
	modalRestoreSelectRepo
	modalRestoreYesNo
	modalRestoreRename
	modalDeleteConfirm
	modalSettings
	modalResult
)

type settingsStage int

const (
	settingsStageConfigHome settingsStage = iota
	settingsStageThemeHome
	settingsStageThemeLocalList
	settingsStageThemeRemoteList
	settingsStageUpdateHome
)

type settingsMode int

const (
	settingsModeView settingsMode = iota
	settingsModeApply
	settingsModeInstall
	settingsModeUninstall
)

type appModel struct {
	table        repoTable
	callbacks    AppCallbacks
	width        int
	height       int
	activeMode   activeMode
	activePane   activePane
	commands     []commandDef
	cmdCursor    int
	formOpen     bool
	formFields   []formField
	formFieldIdx int
	formCommand  string
	status       string
	appVersion   string
	busy         bool
	quitting     bool
	showDetails  bool
	theme        UITheme

	restoreState  restoreState
	modalActive   bool
	modalKind     modalKind
	cursorVisible bool
	resultText    string
	resultScroll  int
	deleteRepo    planfile.RepoRecord
	deleteInput   string
	settings      settingsState
}

type settingsState struct {
	stage            settingsStage
	mode             settingsMode
	cursor           int
	homeCursor       int
	themeHomeCursor  int
	updateHomeCursor int
	activeTheme      string
	currentLabel     string
	currentSource    string
	localThemes      []string
	remoteThemes     []ThemeOption
	status           string
	updateInfo       UpdateInfo
	updateBusy       bool
	updateStatus     string
}

type commandResultMsg struct {
	output       string
	err          error
	refreshRepos bool
}

type cursorBlinkMsg struct{}

type reposRefreshedMsg struct {
	repos []planfile.RepoRecord
	err   error
}

type settingsCurrentMsg struct {
	label string
	err   error
}

type settingsLocalMsg struct {
	themes []string
	active string
	err    error
}

type settingsRemoteMsg struct {
	themes []ThemeOption
	source string
	err    error
}

type settingsInstallMsg struct {
	output string
	err    error
}

type settingsApplyMsg struct {
	theme  UITheme
	output string
	err    error
}

type settingsUninstallMsg struct {
	theme  UITheme
	output string
	err    error
}

type updateCheckedMsg struct {
	info UpdateInfo
	err  error
}

type updateRunMsg struct {
	output string
	err    error
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func RunApp(repos []planfile.RepoRecord, callbacks AppCallbacks) error {
	m := newAppModel(repos, callbacks)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newAppModel(repos []planfile.RepoRecord, callbacks AppCallbacks) appModel {
	return appModel{
		table:      newRepoTable(repos),
		callbacks:  callbacks,
		activeMode: modeBrowse,
		activePane: paneTable,
		commands: []commandDef{
			{name: "Plan", icon: "󰦨", desc: "Save signed plan from current selection", fields: []formField{{key: "out", label: "Output path", kind: fieldText, placeholder: "./deletion-plan-YYYYMMDD-HHMMSS.json"}}},
			{name: "Inspect", icon: "󰈞", desc: "Inspect a plan file", fields: []formField{{key: "plan", label: "Plan path", kind: fieldText, required: true, placeholder: "./plan.json"}}},
			{name: "Backup", icon: "󰁯", desc: "Run backup workflow", fields: []formField{{key: "plan", label: "Plan path", kind: fieldText, placeholder: "(auto from current selection)"}, {key: "backup_location", label: "Backup location", kind: fieldText, placeholder: "(auto timestamp folder)"}, {key: "dry_run", label: "Dry run", kind: fieldBool, boolValue: true}, {key: "confirm", label: "Type ACCEPT or CONFIRM", kind: fieldText, required: true, placeholder: "CONFIRM"}}},
			{name: "Execute", icon: "󰐊", desc: "Run execute workflow", fields: []formField{{key: "plan", label: "Plan path", kind: fieldText, placeholder: "(auto from current selection)"}, {key: "backup_location", label: "Backup location", kind: fieldText, placeholder: "(auto timestamp folder)"}, {key: "dry_run", label: "Dry run", kind: fieldBool, boolValue: true}, {key: "confirm", label: "Type ACCEPT or CONFIRM", kind: fieldText, required: true, placeholder: "CONFIRM"}}},
			{name: "Restore", icon: "󰑐", desc: "Restore from local archive to GitHub"},
			{name: "Delete", icon: "󰆴", desc: "Delete highlighted repository (no backup)"},
			{name: "Settings", icon: "󰒓", desc: "Manage configuration, theme, and updates"},
		},
		status:     "Ready",
		appVersion: callbacks.Version,
		theme:      callbacks.Theme.withDefaults(),
		settings: settingsState{
			updateInfo: UpdateInfo{CurrentVersion: formatVersionLabel(callbacks.Version)},
		},
	}
}

func blinkCursorCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

func (m appModel) Init() tea.Cmd {
	if m.callbacks.UpdateCheck == nil {
		return nil
	}
	return m.updateCheckCmd()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.setHeight(m.height)
		m.table.ensureVisible(0)
		return m, nil
	case cursorBlinkMsg:
		if !m.modalActive {
			return m, nil
		}
		m.cursorVisible = !m.cursorVisible
		return m, blinkCursorCmd()
	case commandResultMsg:
		m.busy = false
		if msg.err != nil {
			if m.restoreState.active {
				var conflict restorepkg.TargetExistsError
				if errors.As(msg.err, &conflict) {
					m.restoreState.stage = restoreStageInputNewName
					m.status = "Target exists; choose another name"
					return m, m.openRestoreRenameModal(conflict.SuggestedName())
				}
			}
			m.status = "Error: " + msg.err.Error()
			return m, m.openResultModal("Error\n" + msg.err.Error())
		}
		m.status = "Command complete"
		m.formOpen = false
		m.closeModal()
		if m.restoreState.active {
			m.restoreState = restoreState{}
		}
		if msg.refreshRepos && m.callbacks.RefreshRepos != nil {
			m.status = "Command complete (refreshing repositories...)"
			_ = m.openResultModal(msg.output)
			return m, m.refreshReposCmd()
		}
		return m, m.openResultModal(msg.output)
	case reposRefreshedMsg:
		if msg.err != nil {
			m.status = "Warning: refresh failed: " + msg.err.Error()
			return m, nil
		}
		m.table.replaceRepos(msg.repos)
		m.status = "Command complete (repositories refreshed)"
		return m, nil
	case settingsCurrentMsg:
		if msg.err != nil {
			m.settings.status = "Error: " + msg.err.Error()
			return m, nil
		}
		m.settings.currentLabel = msg.label
		return m, nil
	case settingsLocalMsg:
		if msg.err != nil {
			m.settings.status = "Error: " + msg.err.Error()
			return m, nil
		}
		m.settings.localThemes = msg.themes
		m.settings.activeTheme = msg.active
		m.settings.currentSource = "~/.config/gh-manager/themes"
		return m, nil
	case settingsRemoteMsg:
		if msg.err != nil {
			m.settings.status = "Error: " + msg.err.Error()
			return m, nil
		}
		m.settings.remoteThemes = msg.themes
		m.settings.currentSource = msg.source
		return m, nil
	case settingsInstallMsg:
		if msg.err != nil {
			m.settings.status = "Error: " + msg.err.Error()
			return m, nil
		}
		m.settings.status = msg.output
		return m, m.settingsListLocalCmd()
	case settingsApplyMsg:
		if msg.err != nil {
			m.settings.status = "Error: " + msg.err.Error()
			return m, nil
		}
		m.theme = msg.theme.withDefaults()
		m.settings.status = msg.output
		return m, m.settingsListLocalCmd()
	case settingsUninstallMsg:
		if msg.err != nil {
			m.settings.status = "Error: " + msg.err.Error()
			return m, nil
		}
		m.theme = msg.theme.withDefaults()
		m.settings.status = msg.output
		return m, m.settingsListLocalCmd()
	case updateCheckedMsg:
		if msg.err != nil {
			m.settings.updateInfo.UpdateAvailable = false
			m.settings.updateInfo.Error = msg.err.Error()
			m.settings.updateInfo.CheckedAt = time.Now()
			m.settings.updateStatus = "Error: " + msg.err.Error()
			return m, nil
		}
		m.settings.updateInfo = msg.info
		if strings.TrimSpace(m.settings.updateInfo.CurrentVersion) == "" {
			m.settings.updateInfo.CurrentVersion = formatVersionLabel(m.appVersion)
		}
		if m.settings.updateInfo.UpdateAvailable {
			m.settings.updateStatus = fmt.Sprintf("Update available: %s", formatVersionLabel(m.settings.updateInfo.LatestVersion))
		} else {
			m.settings.updateStatus = "Already up to date"
		}
		return m, nil
	case updateRunMsg:
		m.settings.updateBusy = false
		if msg.err != nil {
			m.settings.updateStatus = "Error: " + msg.err.Error()
			return m, nil
		}
		latest := formatVersionLabel(m.settings.updateInfo.LatestVersion)
		m.settings.updateStatus = "Update installed. Restart gh-manager to use " + latest + "."
		return m, m.openResultModal(msg.output)
	case tea.KeyMsg:
		s := msg.String()
		if s == "ctrl+c" || s == "q" {
			m.quitting = true
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.modalActive {
			return m.updateModalInput(s)
		}

		switch s {
		case "1":
			m.activeMode = modeBrowse
			m.activePane = paneTable
			m.formOpen = false
			m.restoreState = restoreState{}
			m.closeModal()
			return m, nil
		case "2":
			m.activeMode = modeCommands
			m.activePane = paneCommands
			return m, nil
		case "3":
			m.activeMode = modeDetails
			m.activePane = paneTable
			m.formOpen = false
			m.restoreState = restoreState{}
			m.closeModal()
			return m, nil
		case "tab":
			if m.activePane == paneTable {
				m.activePane = paneCommands
			} else {
				m.activePane = paneTable
			}
			return m, nil
		}

		if m.activeMode == modeCommands || m.activePane == paneCommands {
			return m.updateCommands(s)
		}
		return m.updateBrowse(s)
	}
	return m, nil
}

func (m appModel) updateBrowse(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.table.moveCursor(-1, 0)
	case "down", "j":
		m.table.moveCursor(1, 0)
	case "pgup":
		m.table.pageMove(-1, 0)
	case "pgdown":
		m.table.pageMove(1, 0)
	case " ":
		m.table.toggleCurrent()
	case "a":
		m.table.selectAllFiltered()
	case "x":
		m.table.clearAllFiltered()
	case "backspace":
		m.table.backspaceFilter()
	case "n":
		m.table.setSortField(sortFieldName)
	case "u":
		m.table.setSortField(sortFieldUpdated)
	case "v":
		m.table.setSortField(sortFieldVisibility)
	default:
		m.table.appendFilterChar(key)
	}
	return m, nil
}

func (m appModel) updateCommands(key string) (tea.Model, tea.Cmd) {
	if m.restoreState.active {
		return m.updateRestoreFlow(key)
	}

	switch key {
	case "up", "k":
		if m.cmdCursor > 0 {
			m.cmdCursor--
		}
	case "down", "j":
		if m.cmdCursor < len(m.commands)-1 {
			m.cmdCursor++
		}
	case "enter":
		cmd := m.openFormForCurrentCommand()
		return m, cmd
	}
	return m, nil
}

func (m *appModel) openFormForCurrentCommand() tea.Cmd {
	if len(m.commands) == 0 || m.cmdCursor < 0 || m.cmdCursor >= len(m.commands) {
		return nil
	}
	cmd := m.commands[m.cmdCursor]
	if cmd.name == "Restore" {
		return m.startRestoreFlow()
	}
	if cmd.name == "Delete" {
		repo, ok := m.table.currentRepo()
		if !ok {
			m.status = "No repository selected"
			return nil
		}
		return m.openDeleteConfirmModal(repo)
	}
	if cmd.name == "Settings" {
		return m.openSettingsModal()
	}
	m.formCommand = cmd.name
	m.formFields = append([]formField(nil), cmd.fields...)
	m.formFieldIdx = 0
	m.formOpen = true
	m.status = "Fill form and press enter to run"
	return m.openCommandFormModal()
}

func (m appModel) submitCommandForm() (tea.Cmd, error) {
	vals := map[string]formField{}
	for _, f := range m.formFields {
		vals[f.key] = f
		if f.required && strings.TrimSpace(f.value) == "" {
			return nil, fmt.Errorf("%s is required", f.label)
		}
	}

	switch m.formCommand {
	case "Plan":
		selected := m.table.selectedReposSorted()
		if len(selected) == 0 {
			return nil, fmt.Errorf("no repositories selected")
		}
		if m.callbacks.Plan == nil {
			return nil, fmt.Errorf("plan callback unavailable")
		}
		outPath := strings.TrimSpace(vals["out"].value)
		return func() tea.Msg {
			out, err := m.callbacks.Plan(selected, outPath)
			return commandResultMsg{output: out, err: err}
		}, nil
	case "Inspect":
		if m.callbacks.Inspect == nil {
			return nil, fmt.Errorf("inspect callback unavailable")
		}
		planPath := strings.TrimSpace(vals["plan"].value)
		return func() tea.Msg {
			out, err := m.callbacks.Inspect(planPath)
			return commandResultMsg{output: out, err: err}
		}, nil
	case "Backup":
		if m.callbacks.Backup == nil {
			return nil, fmt.Errorf("backup callback unavailable")
		}
		planPath := strings.TrimSpace(vals["plan"].value)
		backupLocation := strings.TrimSpace(vals["backup_location"].value)
		dryRun := vals["dry_run"].boolValue
		confirm := strings.TrimSpace(vals["confirm"].value)
		selected := m.table.selectedReposSorted()
		return func() tea.Msg {
			out, err := m.callbacks.Backup(planPath, backupLocation, dryRun, confirm, selected)
			return commandResultMsg{output: out, err: err}
		}, nil
	case "Execute":
		if m.callbacks.Execute == nil {
			return nil, fmt.Errorf("execute callback unavailable")
		}
		planPath := strings.TrimSpace(vals["plan"].value)
		backupLocation := strings.TrimSpace(vals["backup_location"].value)
		dryRun := vals["dry_run"].boolValue
		confirm := strings.TrimSpace(vals["confirm"].value)
		selected := m.table.selectedReposSorted()
		return func() tea.Msg {
			out, err := m.callbacks.Execute(planPath, backupLocation, dryRun, confirm, selected)
			return commandResultMsg{output: out, err: err, refreshRepos: err == nil}
		}, nil
	case "Restore":
		return nil, fmt.Errorf("restore is handled by restore window")
	default:
		return nil, fmt.Errorf("unsupported command: %s", m.formCommand)
	}
}

func (m *appModel) openCommandFormModal() tea.Cmd {
	m.modalActive = true
	m.modalKind = modalCommandForm
	m.cursorVisible = true
	return blinkCursorCmd()
}

func (m *appModel) openRestoreYesNoModal() tea.Cmd {
	m.modalActive = true
	m.modalKind = modalRestoreYesNo
	m.cursorVisible = true
	return blinkCursorCmd()
}

func (m *appModel) openRestoreBrowseModal() tea.Cmd {
	m.modalActive = true
	m.modalKind = modalRestoreBrowse
	m.cursorVisible = false
	return nil
}

func (m *appModel) openRestoreSelectRepoModal() tea.Cmd {
	m.modalActive = true
	m.modalKind = modalRestoreSelectRepo
	m.cursorVisible = false
	return nil
}

func (m *appModel) openRestoreRenameModal(seed string) tea.Cmd {
	m.modalActive = true
	m.modalKind = modalRestoreRename
	m.cursorVisible = true
	m.restoreState.promptInput = seed
	return blinkCursorCmd()
}

func (m *appModel) openDeleteConfirmModal(repo planfile.RepoRecord) tea.Cmd {
	m.modalActive = true
	m.modalKind = modalDeleteConfirm
	m.cursorVisible = true
	m.deleteRepo = repo
	m.deleteInput = ""
	m.status = "Danger: type repo name to confirm delete"
	return blinkCursorCmd()
}

func (m *appModel) openSettingsModal() tea.Cmd {
	m.modalActive = true
	m.modalKind = modalSettings
	m.cursorVisible = false
	m.settings = settingsState{
		stage:        settingsStageConfigHome,
		mode:         settingsModeView,
		homeCursor:   0,
		status:       "Configuration",
		updateInfo:   m.settings.updateInfo,
		updateStatus: m.settings.updateStatus,
	}
	return tea.Batch(m.settingsCurrentCmd(), m.settingsListLocalCmd(), m.updateCheckCmd())
}

func (m *appModel) closeModal() {
	savedUpdate := m.settings.updateInfo
	savedUpdateStatus := m.settings.updateStatus
	m.modalActive = false
	m.modalKind = modalNone
	m.cursorVisible = false
	m.resultText = ""
	m.resultScroll = 0
	m.deleteRepo = planfile.RepoRecord{}
	m.deleteInput = ""
	m.settings = settingsState{
		updateInfo:   savedUpdate,
		updateStatus: savedUpdateStatus,
	}
}

func (m *appModel) openResultModal(text string) tea.Cmd {
	m.modalActive = true
	m.modalKind = modalResult
	m.cursorVisible = false
	m.resultText = strings.TrimSpace(text)
	if m.resultText == "" {
		m.resultText = "Command complete."
	}
	m.resultScroll = 0
	return nil
}

func (m appModel) refreshReposCmd() tea.Cmd {
	if m.callbacks.RefreshRepos == nil {
		return nil
	}
	return func() tea.Msg {
		repos, err := m.callbacks.RefreshRepos()
		return reposRefreshedMsg{repos: repos, err: err}
	}
}

func (m appModel) settingsCurrentCmd() tea.Cmd {
	if m.callbacks.ThemeCurrent == nil {
		return func() tea.Msg { return settingsCurrentMsg{err: fmt.Errorf("theme current callback unavailable")} }
	}
	return func() tea.Msg {
		label, err := m.callbacks.ThemeCurrent()
		return settingsCurrentMsg{label: label, err: err}
	}
}

func (m appModel) settingsListLocalCmd() tea.Cmd {
	if m.callbacks.ThemeListLocal == nil {
		return func() tea.Msg { return settingsLocalMsg{err: fmt.Errorf("theme local list callback unavailable")} }
	}
	return func() tea.Msg {
		themes, active, err := m.callbacks.ThemeListLocal()
		return settingsLocalMsg{themes: themes, active: active, err: err}
	}
}

func (m appModel) settingsListRemoteCmd() tea.Cmd {
	if m.callbacks.ThemeListRemote == nil {
		return func() tea.Msg { return settingsRemoteMsg{err: fmt.Errorf("theme remote list callback unavailable")} }
	}
	return func() tea.Msg {
		themes, source, err := m.callbacks.ThemeListRemote()
		return settingsRemoteMsg{themes: themes, source: source, err: err}
	}
}

func (m appModel) updateCheckCmd() tea.Cmd {
	if m.callbacks.UpdateCheck == nil {
		return nil
	}
	return func() tea.Msg {
		info, err := m.callbacks.UpdateCheck()
		return updateCheckedMsg{info: info, err: err}
	}
}

func (m appModel) updateRunCmd() tea.Cmd {
	if m.callbacks.UpdateRun == nil {
		return nil
	}
	return func() tea.Msg {
		out, err := m.callbacks.UpdateRun()
		return updateRunMsg{output: out, err: err}
	}
}

func (m appModel) updateModalInput(key string) (tea.Model, tea.Cmd) {
	switch m.modalKind {
	case modalCommandForm:
		switch key {
		case "esc":
			m.formOpen = false
			m.closeModal()
			m.status = "Form canceled"
		case "up":
			if m.formFieldIdx > 0 {
				m.formFieldIdx--
			}
		case "down", "tab":
			if m.formFieldIdx < len(m.formFields)-1 {
				m.formFieldIdx++
			} else {
				m.formFieldIdx = 0
			}
		case "space", " ":
			if len(m.formFields) > 0 && m.formFields[m.formFieldIdx].kind == fieldBool {
				m.formFields[m.formFieldIdx].boolValue = !m.formFields[m.formFieldIdx].boolValue
			}
		case "backspace":
			if len(m.formFields) > 0 && m.formFields[m.formFieldIdx].kind == fieldText {
				v := m.formFields[m.formFieldIdx].value
				if len(v) > 0 {
					m.formFields[m.formFieldIdx].value = v[:len(v)-1]
				}
			}
		case "enter":
			cmd, err := m.submitCommandForm()
			if err != nil {
				m.status = "Error: " + err.Error()
				return m, nil
			}
			m.closeModal()
			m.busy = true
			m.status = "Running " + strings.ToLower(m.formCommand) + "..."
			return m, cmd
		default:
			if len(m.formFields) > 0 && m.formFields[m.formFieldIdx].kind == fieldText && isPrintableKey(key) {
				m.formFields[m.formFieldIdx].value += key
			}
		}
		return m, nil
	case modalRestoreBrowse, modalRestoreSelectRepo:
		return m.updateRestoreFlow(key)
	case modalRestoreYesNo:
		switch key {
		case "esc":
			m.closeModal()
			m.restoreState.stage = restoreStageSelectRepo
			m.status = "Restore: select repository"
		case "backspace":
			if len(m.restoreState.promptInput) > 0 {
				m.restoreState.promptInput = m.restoreState.promptInput[:len(m.restoreState.promptInput)-1]
			}
		case "enter":
			yes, valid := parseYesNo(m.restoreState.promptInput)
			if !valid {
				m.status = "Please answer yes or no"
				return m, nil
			}
			if yes {
				cmd, err := m.submitRestore(originalRepoName(m.restoreState.selected.fullName))
				if err != nil {
					m.status = "Error: " + err.Error()
					return m, nil
				}
				m.closeModal()
				m.busy = true
				m.status = "Running restore..."
				return m, cmd
			}
			return m, m.openRestoreRenameModal(originalRepoName(m.restoreState.selected.fullName) + "-ghm")
		default:
			if isPrintableKey(key) {
				m.restoreState.promptInput += key
			}
		}
		return m, nil
	case modalRestoreRename:
		switch key {
		case "esc":
			m.restoreState.stage = restoreStageAskUseOriginal
			return m, m.openRestoreYesNoModal()
		case "backspace":
			if len(m.restoreState.promptInput) > 0 {
				m.restoreState.promptInput = m.restoreState.promptInput[:len(m.restoreState.promptInput)-1]
			}
		case "enter":
			name := strings.TrimSpace(m.restoreState.promptInput)
			if name == "" {
				m.status = "New name is required"
				return m, nil
			}
			cmd, err := m.submitRestore(name)
			if err != nil {
				m.status = "Error: " + err.Error()
				return m, nil
			}
			m.closeModal()
			m.busy = true
			m.status = "Running restore..."
			return m, cmd
		default:
			if isPrintableKey(key) {
				m.restoreState.promptInput += key
			}
		}
		return m, nil
	case modalDeleteConfirm:
		switch key {
		case "esc":
			m.closeModal()
			m.status = "Delete canceled"
			return m, nil
		case "backspace":
			if len(m.deleteInput) > 0 {
				m.deleteInput = m.deleteInput[:len(m.deleteInput)-1]
			}
			return m, nil
		case "enter":
			if m.callbacks.Delete == nil {
				m.status = "Error: delete callback unavailable"
				return m, nil
			}
			expected := strings.TrimSpace(m.deleteRepo.Name)
			if strings.TrimSpace(m.deleteInput) != expected {
				m.status = "Error: confirmation must match repository name"
				return m, nil
			}
			repo := m.deleteRepo
			m.closeModal()
			m.busy = true
			m.status = "Deleting " + repo.FullName + "..."
			return m, func() tea.Msg {
				out, err := m.callbacks.Delete(repo)
				return commandResultMsg{output: out, err: err, refreshRepos: err == nil}
			}
		default:
			if isPrintableKey(key) {
				m.deleteInput += key
			}
			return m, nil
		}
	case modalSettings:
		return m.updateSettingsModal(key)
	case modalResult:
		switch key {
		case "esc", "enter", " ":
			m.closeModal()
			return m, nil
		case "up":
			if m.resultScroll > 0 {
				m.resultScroll--
			}
		case "down":
			m.resultScroll++
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m appModel) updateSettingsModal(key string) (tea.Model, tea.Cmd) {
	s := m.settings
	switch s.stage {
	case settingsStageConfigHome:
		actions := []string{"Theme", "Update", "Close"}
		switch key {
		case "esc":
			m.closeModal()
			return m, nil
		case "up", "k":
			if s.homeCursor > 0 {
				s.homeCursor--
			}
		case "down", "j":
			if s.homeCursor < len(actions)-1 {
				s.homeCursor++
			}
		case "enter":
			switch actions[s.homeCursor] {
			case "Theme":
				s.stage = settingsStageThemeHome
				s.themeHomeCursor = 0
				m.settings = s
				return m, m.settingsCurrentCmd()
			case "Update":
				s.stage = settingsStageUpdateHome
				s.updateHomeCursor = 0
				m.settings = s
				return m, m.updateCheckCmd()
			case "Close":
				m.closeModal()
				return m, nil
			}
		}
	case settingsStageThemeHome:
		actions := []string{"Current", "List local themes", "Apply local theme", "Uninstall local theme", "List remote themes", "Install remote theme", "Back"}
		switch key {
		case "esc":
			s.stage = settingsStageConfigHome
		case "up", "k":
			if s.themeHomeCursor > 0 {
				s.themeHomeCursor--
			}
		case "down", "j":
			if s.themeHomeCursor < len(actions)-1 {
				s.themeHomeCursor++
			}
		case "enter":
			switch actions[s.themeHomeCursor] {
			case "Current":
				m.settings = s
				return m, m.settingsCurrentCmd()
			case "List local themes":
				s.stage = settingsStageThemeLocalList
				s.mode = settingsModeView
				s.cursor = 0
				m.settings = s
				return m, m.settingsListLocalCmd()
			case "Apply local theme":
				s.stage = settingsStageThemeLocalList
				s.mode = settingsModeApply
				s.cursor = 0
				m.settings = s
				return m, m.settingsListLocalCmd()
			case "Uninstall local theme":
				s.stage = settingsStageThemeLocalList
				s.mode = settingsModeUninstall
				s.cursor = 0
				m.settings = s
				return m, m.settingsListLocalCmd()
			case "List remote themes":
				s.stage = settingsStageThemeRemoteList
				s.mode = settingsModeView
				s.cursor = 0
				m.settings = s
				return m, m.settingsListRemoteCmd()
			case "Install remote theme":
				s.stage = settingsStageThemeRemoteList
				s.mode = settingsModeInstall
				s.cursor = 0
				m.settings = s
				return m, m.settingsListRemoteCmd()
			case "Back":
				s.stage = settingsStageConfigHome
			}
		}
	case settingsStageThemeLocalList:
		switch key {
		case "esc":
			s.stage = settingsStageThemeHome
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.localThemes)-1 {
				s.cursor++
			}
		case "enter":
			if len(s.localThemes) == 0 {
				break
			}
			id := s.localThemes[s.cursor]
			if s.mode == settingsModeApply {
				if m.callbacks.ThemeApply == nil {
					s.status = "Error: theme apply callback unavailable"
					break
				}
				m.settings = s
				return m, func() tea.Msg {
					theme, out, err := m.callbacks.ThemeApply(id)
					return settingsApplyMsg{theme: theme, output: out, err: err}
				}
			}
			if s.mode == settingsModeUninstall {
				if m.callbacks.ThemeUninstall == nil {
					s.status = "Error: theme uninstall callback unavailable"
					break
				}
				m.settings = s
				return m, func() tea.Msg {
					theme, out, err := m.callbacks.ThemeUninstall(id)
					return settingsUninstallMsg{theme: theme, output: out, err: err}
				}
			}
			s.status = "Selected local theme: " + id
		}
	case settingsStageThemeRemoteList:
		switch key {
		case "esc":
			s.stage = settingsStageThemeHome
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.remoteThemes)-1 {
				s.cursor++
			}
		case "enter":
			if len(s.remoteThemes) == 0 {
				break
			}
			opt := s.remoteThemes[s.cursor]
			if s.mode == settingsModeInstall {
				if m.callbacks.ThemeInstall == nil {
					s.status = "Error: theme install callback unavailable"
					break
				}
				m.settings = s
				return m, func() tea.Msg {
					out, err := m.callbacks.ThemeInstall(opt.ID)
					return settingsInstallMsg{output: out, err: err}
				}
			}
			s.status = "Remote theme: " + opt.ID
		}
	case settingsStageUpdateHome:
		actions := []string{"Check now", "Update now", "View release URL", "Back"}
		switch key {
		case "esc":
			s.stage = settingsStageConfigHome
		case "up", "k":
			if s.updateHomeCursor > 0 {
				s.updateHomeCursor--
			}
		case "down", "j":
			if s.updateHomeCursor < len(actions)-1 {
				s.updateHomeCursor++
			}
		case "enter":
			switch actions[s.updateHomeCursor] {
			case "Check now":
				m.settings = s
				return m, m.updateCheckCmd()
			case "Update now":
				if s.updateBusy {
					break
				}
				if !s.updateInfo.UpdateAvailable {
					s.updateStatus = "Already up to date"
					break
				}
				if m.callbacks.UpdateRun == nil {
					s.updateStatus = "Error: update callback unavailable"
					break
				}
				s.updateBusy = true
				m.settings = s
				return m, m.updateRunCmd()
			case "View release URL":
				if strings.TrimSpace(s.updateInfo.ReleaseURL) == "" {
					s.updateStatus = "No release URL available"
				} else {
					s.updateStatus = s.updateInfo.ReleaseURL
				}
			case "Back":
				s.stage = settingsStageConfigHome
			}
		}
	}
	m.settings = s
	return m, nil
}

func (m appModel) View() string {
	if m.quitting {
		return "Exited.\n"
	}
	if m.width <= 0 {
		m.width = 140
	}
	if m.height <= 0 {
		m.height = 36
	}
	status := fmt.Sprintf("Mode: %s | Focus: %s | Sort: %s | Filter: %s | Selected: %d | Visible: %d/%d", modeLabel(m.activeMode), paneLabel(m.activePane), sortLabel(m.table.sortBy, m.table.sortDir), m.table.filter, len(m.table.selected), len(m.table.filtered), len(m.table.repos))

	help := globalHelp()
	if m.activeMode == modeCommands {
		help = help + " | " + commandHelp()
	} else {
		help = help + " | " + browseHelp()
	}

	topBanner := m.renderTopBanner(m.width)
	help = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.HelpText)).Render(clampLine(help, m.width))
	status = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusText)).Render(clampLine(status, m.width))

	chromeHeight := len(topBanner) + 3
	bodyHeight := m.height - chromeHeight
	if bodyHeight < 4 {
		bodyHeight = 4
	}
	m.table.setHeight(bodyHeight)

	gap := 1
	availableWidth := m.width
	if availableWidth < 70 {
		availableWidth = 70
	}
	contentWidth := availableWidth - gap
	leftWidth := (contentWidth * 2) / 3
	rightWidth := contentWidth - leftWidth
	if leftWidth < 48 {
		leftWidth = 48
	}
	if rightWidth < 24 {
		rightWidth = 24
	}
	if leftWidth+rightWidth > contentWidth {
		over := leftWidth + rightWidth - contentWidth
		if leftWidth-48 >= over {
			leftWidth -= over
		} else {
			leftWidth = 48
			rightWidth = contentWidth - leftWidth
		}
	}
	if rightWidth < 0 {
		rightWidth = 0
	}

	left := m.table.renderTableWithTheme(leftWidth, m.activePane == paneTable, 0, m.theme)
	body := left
	if rightWidth > 0 {
		right := m.renderRightColumn(rightWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	out := make([]string, 0, len(topBanner)+3)
	out = append(out, topBanner...)
	out = append(out, "")
	out = append(out, help, status, body)
	view := strings.Join(out, "\n")
	if m.modalActive {
		backdrop := applyBackdrop(view, m.width, m.height)
		view = overlayCentered(backdrop, m.renderModalOverlay(), m.width, m.height)
	}
	return view + "\n"
}

func (m appModel) renderRightColumn(width, height int) string {
	topHeight := height / 2
	bottomHeight := height - topHeight
	if topHeight < 6 {
		topHeight = 6
	}
	if bottomHeight < 6 {
		bottomHeight = 6
		topHeight = height - bottomHeight
	}
	top := m.renderCommandPanel(width, topHeight)
	bottom := m.renderDetailPanel(width, bottomHeight)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (m appModel) renderCommandPanel(width, height int) string {
	lines := []string{"Commands"}
	innerW := panelInnerWidth(width)
	const commandIndent = "  "
	for i, c := range m.commands {
		if i == m.cmdCursor {
		}
		label := commandIndent + c.name
		if strings.TrimSpace(c.icon) != "" {
			label = commandIndent + c.icon + " " + c.name
		}
		line := fmt.Sprintf("%s - %s", label, c.desc)
		line = truncateRaw(line, innerW)
		if i == m.cmdCursor {
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color(m.theme.SelectionFg)).
				Background(lipgloss.Color(m.theme.SelectionBg)).
				Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, truncateRaw("Status: "+m.status, innerW))
	lines = fitLines(lines, innerHeight(height))

	style := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).Padding(0, 1)
	if m.activePane == paneCommands {
		style = style.BorderForeground(lipgloss.Color(m.theme.PaneBorderActive))
	} else {
		style = style.BorderForeground(lipgloss.Color(m.theme.PaneBorderInactive))
	}
	return style.Width(width).Render(strings.Join(lines, "\n"))
}

func (m appModel) renderDetailPanel(width, height int) string {
	repo, ok := m.table.currentRepo()
	if !ok {
		empty := []string{"Repo Details", "No repository selected"}
		empty = fitAndWrapLines(empty, innerHeight(height), panelInnerWidth(width))
		for i := range empty {
			if i == 0 {
				empty[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DetailsLabel)).Render(empty[i])
			} else {
				empty[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DetailsValue)).Render(empty[i])
			}
		}
		return lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(m.theme.PaneBorderInactive)).
			Padding(0, 1).
			Width(width).
			Render(strings.Join(empty, "\n"))
	}
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
	if height < 10 {
		lines = []string{
			"Repo Details",
			fmt.Sprintf("%s | %s", repo.FullName, visibilityLabel(repo)),
			fmt.Sprintf("fork=%t archived=%t updatedAt=%s", repo.IsFork, repo.IsArchived, repo.UpdatedAt),
		}
	}
	lines = fitAndWrapLines(lines, innerHeight(height), panelInnerWidth(width))
	for i := range lines {
		if i == 0 {
			lines[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DetailsLabel)).Render(lines[i])
			continue
		}
		lines[i] = colorizeDetailLine(lines[i], m.theme)
	}
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(m.theme.PaneBorderInactive)).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
}

func (m appModel) renderModalOverlay() string {
	width := m.width - 6
	if width > 96 {
		width = 96
	}
	if width < 36 {
		width = 36
	}
	title := "Input"
	lines := []string{}
	maxLines := 14
	popupBorderColor := lipgloss.Color(m.theme.PopupBorder)
	switch m.modalKind {
	case modalCommandForm:
		title = "Form: " + m.formCommand
		lines = append(lines, "Shortcuts suspended. Enter submits, Esc cancels.", "")
		for i, f := range m.formFields {
			prefix := "  "
			if i == m.formFieldIdx {
				prefix = "> "
			}
			if f.kind == fieldBool {
				lines = append(lines, fmt.Sprintf("%s%s: %t", prefix, f.label, f.boolValue))
				continue
			}
			val := f.value
			if strings.TrimSpace(val) == "" {
				val = f.placeholder
			}
			if i == m.formFieldIdx {
				val = renderInputLineWithCursor(val, m.cursorVisible)
			}
			lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, f.label, val))
		}
	case modalRestoreBrowse:
		title = "Restore: Archive Browser"
		lines = append(lines, "Select archive root.", "Enter open/select, Backspace parent, h/l scroll, Esc cancel.", "")
		lines = append(lines, truncateRaw("dir: "+m.restoreState.browserDir, panelInnerWidth(width)), "")
		itemWidth := panelInnerWidth(width) - 2
		if itemWidth < 1 {
			itemWidth = 1
		}
		scroll := m.restoreState.browserHScroll
		maxScroll := 0
		for i, it := range m.restoreState.browserItems {
			prefix := "  "
			if i == m.restoreState.browserCursor {
				prefix = "> "
			}
			label, _, max := windowedText(it.label, scroll, itemWidth)
			if max > maxScroll {
				maxScroll = max
			}
			lines = append(lines, prefix+label)
		}
		lines = append(lines, "", fmt.Sprintf("[h/l scroll: %d/%d]", clampInt(scroll, 0, maxScroll), maxScroll))
		maxLines = 20
	case modalRestoreSelectRepo:
		title = "Restore: Select Repository"
		lines = append(lines, truncateRaw("Archive: "+m.restoreState.archiveRoot, panelInnerWidth(width)), "Enter choose source repo, h/l scroll, Esc back.", "")
		itemWidth := panelInnerWidth(width) - 2
		if itemWidth < 1 {
			itemWidth = 1
		}
		scroll := m.restoreState.repoHScroll
		maxScroll := 0
		for i, r := range m.restoreState.repos {
			prefix := "  "
			if i == m.restoreState.repoCursor {
				prefix = "> "
			}
			label := fmt.Sprintf("%s (%s)", r.fullName, r.sourceKind)
			label, _, max := windowedText(label, scroll, itemWidth)
			if max > maxScroll {
				maxScroll = max
			}
			lines = append(lines, prefix+label)
		}
		lines = append(lines, "", fmt.Sprintf("[h/l scroll: %d/%d]", clampInt(scroll, 0, maxScroll), maxScroll))
		maxLines = 20
	case modalRestoreYesNo:
		title = "Restore Confirmation"
		lines = append(lines,
			"Use original name for "+m.restoreState.selected.fullName+"?",
			"Type yes/no and press Enter.",
			"",
			"input: "+renderInputLineWithCursor(m.restoreState.promptInput, m.cursorVisible),
		)
	case modalRestoreRename:
		title = "Restore Rename"
		lines = append(lines,
			"Enter new repository name and press Enter.",
			"",
			"name: "+renderInputLineWithCursor(m.restoreState.promptInput, m.cursorVisible),
		)
	case modalDeleteConfirm:
		title = ""
		popupBorderColor = lipgloss.Color(m.theme.Danger)
		dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DangerText))
		dangerBold := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DangerText)).Bold(true)
		bold := lipgloss.NewStyle().Bold(true)
		desc := strings.TrimSpace(m.deleteRepo.Description)
		if desc == "" {
			desc = "(none)"
		}
		lines = append(lines,
			dangerBold.Render("DANGER:")+dangerStyle.Render(" Delete Repository"),
			dangerBold.Render("WARNING:")+dangerStyle.Render(" No backup has been selected."),
			dangerStyle.Render("This action permanently deletes the GitHub repository."),
			"",
			fmt.Sprintf("fullName: %s", m.deleteRepo.FullName),
			fmt.Sprintf("owner: %s", m.deleteRepo.Owner),
			bold.Render(fmt.Sprintf("name: %s", m.deleteRepo.Name)),
			fmt.Sprintf("visibility: %s", visibilityLabel(m.deleteRepo)),
			fmt.Sprintf("fork: %t | archived: %t", m.deleteRepo.IsFork, m.deleteRepo.IsArchived),
			fmt.Sprintf("updatedAt: %s", m.deleteRepo.UpdatedAt),
			fmt.Sprintf("description: %s", desc),
			"",
			"Type repo name to confirm: "+bold.Render(m.deleteRepo.Name),
			renderInputLineWithCursor(m.deleteInput, m.cursorVisible),
		)
		maxLines = 18
	case modalSettings:
		title = "Settings"
		switch m.settings.stage {
		case settingsStageConfigHome:
			lines = append(lines,
				"Configuration",
				"",
			)
			actions := []string{"Theme", "Update", "Close"}
			for i, a := range actions {
				p := "  "
				if i == m.settings.homeCursor {
					p = "> "
				}
				lines = append(lines, p+a)
			}
			if strings.TrimSpace(m.settings.status) != "" {
				lines = append(lines, "", "Status: "+m.settings.status)
			}
		case settingsStageThemeHome:
			lines = append(lines,
				"Theme settings",
				fmt.Sprintf("Current: %s", m.settings.currentLabel),
				"",
			)
			actions := []string{"Current", "List local themes", "Apply local theme", "Uninstall local theme", "List remote themes", "Install remote theme", "Back"}
			for i, a := range actions {
				p := "  "
				if i == m.settings.themeHomeCursor {
					p = "> "
				}
				lines = append(lines, p+a)
			}
			if strings.TrimSpace(m.settings.status) != "" {
				lines = append(lines, "", "Status: "+m.settings.status)
			}
		case settingsStageThemeLocalList:
			modeLabel := "View local themes"
			if m.settings.mode == settingsModeApply {
				modeLabel = "Apply local theme"
			}
			if m.settings.mode == settingsModeUninstall {
				modeLabel = "Uninstall local theme"
			}
			lines = append(lines, modeLabel, fmt.Sprintf("Active: %s", m.settings.activeTheme), "")
			if len(m.settings.localThemes) == 0 {
				lines = append(lines, "(no local themes)")
			}
			for i, id := range m.settings.localThemes {
				p := "  "
				if i == m.settings.cursor {
					p = "> "
				}
				if id == m.settings.activeTheme {
					lines = append(lines, p+id+" (active)")
				} else {
					lines = append(lines, p+id)
				}
			}
			if strings.TrimSpace(m.settings.status) != "" {
				lines = append(lines, "", "Status: "+m.settings.status)
			}
		case settingsStageThemeRemoteList:
			modeLabel := "View remote themes"
			if m.settings.mode == settingsModeInstall {
				modeLabel = "Install remote theme"
			}
			lines = append(lines, modeLabel, fmt.Sprintf("Source: %s", m.settings.currentSource), "")
			if len(m.settings.remoteThemes) == 0 {
				lines = append(lines, "(no remote themes)")
			}
			for i, opt := range m.settings.remoteThemes {
				p := "  "
				if i == m.settings.cursor {
					p = "> "
				}
				lines = append(lines, p+fmt.Sprintf("%s - %s", opt.ID, opt.Name))
			}
			if strings.TrimSpace(m.settings.status) != "" {
				lines = append(lines, "", "Status: "+m.settings.status)
			}
		case settingsStageUpdateHome:
			lines = append(lines,
				"Update settings",
				fmt.Sprintf("Current: %s", formatVersionLabel(m.settings.updateInfo.CurrentVersion)),
			)
			if strings.TrimSpace(m.settings.updateInfo.LatestVersion) != "" {
				lines = append(lines, fmt.Sprintf("Latest: %s", formatVersionLabel(m.settings.updateInfo.LatestVersion)))
			}
			if strings.TrimSpace(m.settings.updateInfo.ReleaseURL) != "" {
				lines = append(lines, "Release: "+m.settings.updateInfo.ReleaseURL)
			}
			lines = append(lines, "")
			actions := []string{"Check now", "Update now", "View release URL", "Back"}
			for i, a := range actions {
				p := "  "
				if i == m.settings.updateHomeCursor {
					p = "> "
				}
				lines = append(lines, p+a)
			}
			if strings.TrimSpace(m.settings.updateStatus) != "" {
				lines = append(lines, "", "Status: "+m.settings.updateStatus)
			}
		}
		maxLines = 20
	case modalResult:
		title = "Result"
		lines = append(lines, m.renderResultModalLines(panelInnerWidth(width), 14)...)
	}
	if m.modalKind != modalResult {
		lines = fitAndWrapLines(lines, maxLines, panelInnerWidth(width))
	}
	content := strings.Join(lines, "\n")
	inner := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(popupBorderColor).
		Padding(0, 1).
		Width(width).
		Render(title + "\n" + content)
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color(m.theme.PopupOuterBorder)).
		Padding(0, 1).
		Render(inner)
}

func (m appModel) renderResultModalLines(maxWidth, maxLines int) []string {
	lines := wrapLines(strings.Split(m.resultText, "\n"), maxWidth)
	if len(lines) == 0 {
		lines = []string{"(no output)"}
	}
	if m.resultScroll < 0 {
		m.resultScroll = 0
	}
	if m.resultScroll > len(lines)-1 {
		m.resultScroll = len(lines) - 1
	}
	end := m.resultScroll + maxLines - 2
	if end > len(lines) {
		end = len(lines)
	}
	visible := append([]string(nil), lines[m.resultScroll:end]...)
	footer := "Enter/Esc close"
	if len(lines) > maxLines-2 {
		footer = fmt.Sprintf("Up/Down scroll | Enter/Esc close (%d/%d)", m.resultScroll+1, len(lines))
	}
	visible = append(visible, "", footer)
	return fitLines(visible, maxLines)
}

func renderInputLineWithCursor(value string, visible bool) string {
	if visible {
		return value + "|"
	}
	return value + " "
}

func overlayCentered(base, overlay string, width, height int) string {
	base = stripANSI(base)

	baseLines := strings.Split(base, "\n")
	overLines := strings.Split(overlay, "\n")
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = len(baseLines)
		if height < 1 {
			height = 1
		}
	}

	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}
	for i := range baseLines {
		r := []rune(baseLines[i])
		if len(r) < width {
			baseLines[i] = baseLines[i] + strings.Repeat(" ", width-len(r))
		} else if len(r) > width {
			baseLines[i] = string(r[:width])
		}
	}

	overH := len(overLines)
	overW := 0
	for _, l := range overLines {
		if w := lipgloss.Width(stripANSI(l)); w > overW {
			overW = w
		}
	}
	startY := (height - overH) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (width - overW) / 2
	if startX < 0 {
		startX = 0
	}
	for y := 0; y < overH && startY+y < len(baseLines); y++ {
		baseRunes := []rune(baseLines[startY+y])
		for len(baseRunes) < startX+overW {
			baseRunes = append(baseRunes, ' ')
		}
		lineWidth := lipgloss.Width(stripANSI(overLines[y]))
		if lineWidth < 0 {
			lineWidth = 0
		}
		if startX+lineWidth > len(baseRunes) {
			lineWidth = len(baseRunes) - startX
		}
		prefix := string(baseRunes[:startX])
		suffix := ""
		if startX+lineWidth < len(baseRunes) {
			suffix = string(baseRunes[startX+lineWidth:])
		}
		baseLines[startY+y] = prefix + overLines[y] + suffix
	}
	return strings.Join(baseLines, "\n")
}

func applyBackdrop(base string, width, height int) string {
	lines := strings.Split(stripANSI(base), "\n")
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = len(lines)
		if height < 1 {
			height = 1
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		r := []rune(lines[i])
		if len(r) < width {
			r = append(r, []rune(strings.Repeat(" ", width-len(r)))...)
		} else if len(r) > width {
			r = r[:width]
		}
		for j := range r {
			r[j] = softenRune(r[j])
		}
		lines[i] = string(r)
	}
	return strings.Join(lines, "\n")
}

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func softenRune(r rune) rune {
	switch r {
	case '│', '┃':
		return '┆'
	case '─', '━':
		return '┄'
	case '┬', '┴', '┼':
		return '┼'
	case '├':
		return '┝'
	case '┤':
		return '┥'
	case '┌':
		return '┍'
	case '┐':
		return '┑'
	case '└':
		return '┕'
	case '┘':
		return '┙'
	default:
		return r
	}
}

func innerHeight(totalHeight int) int {
	h := totalHeight - 2
	if h < 1 {
		return 1
	}
	return h
}

func fitLines(lines []string, maxLines int) []string {
	if maxLines <= 0 {
		return []string{}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	if len(lines) > maxLines {
		if maxLines == 1 {
			return []string{truncate(lines[0], 20)}
		}
		out := append([]string(nil), lines[:maxLines-1]...)
		out = append(out, "~")
		return out
	}
	out := append([]string(nil), lines...)
	for len(out) < maxLines {
		out = append(out, "")
	}
	return out
}

func clampLine(s string, maxWidth int) string {
	if maxWidth <= 1 {
		return truncate(s, 1)
	}
	r := []rune(s)
	if len(r) <= maxWidth {
		return s
	}
	return truncate(s, maxWidth)
}

func (m appModel) renderTopBanner(maxWidth int) []string {
	logo := []string{
		"                                           ",
		"     _                                     ",
		" ___| |_ ___ _____ ___ ___ ___ ___ ___ ___ ",
		"| . |   |___|     | .'|   | .'| . | -_|  _|",
		"|_  |_|_|   |_|_|_|__,|_|_|__,|_  |___|_|  ",
		"|___|                         |___|         ",
	}
	version := formatVersionLabel(m.appVersion)
	versionRow := len(logo) - 1
	if versionRow < 0 {
		versionRow = 0
	}
	versionBlock := make([]string, len(logo))
	for i := range versionBlock {
		if i == versionRow {
			versionBlock[i] = version
		}
	}
	lineGap := 2
	out := make([]string, 0, len(logo))
	logoStyles := []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.LogoLine1)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.LogoLine2)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.LogoLine3)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.LogoLine4)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.LogoLine5)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.LogoLine6)),
	}
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TextMuted))
	currentVersionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DangerText)).Bold(true)
	latestVersionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SuccessText)).Bold(true)
	updateAvailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Success)).Bold(true)
	updateFailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TextMuted))
	updateIndicator := ""
	if m.settings.updateInfo.UpdateAvailable {
		cur := formatVersionLabel(m.settings.updateInfo.CurrentVersion)
		if cur == "vdev" {
			cur = version
		}
		updateIndicator = currentVersionStyle.Render(cur) +
			versionStyle.Render(" -> ") +
			latestVersionStyle.Render(formatVersionLabel(m.settings.updateInfo.LatestVersion)) +
			versionStyle.Render(" - ") +
			updateAvailStyle.Render("Update available!")
	} else if strings.TrimSpace(m.settings.updateInfo.Error) != "" {
		updateIndicator = updateFailStyle.Render("update check failed")
	}
	for i := range logo {
		line := logoStyles[i].Render(logo[i])
		if i < len(versionBlock) && versionBlock[i] != "" {
			line += strings.Repeat(" ", lineGap) + versionStyle.Render(versionBlock[i])
			if updateIndicator != "" {
				line += strings.Repeat(" ", lineGap) + updateIndicator
			}
		}
		out = append(out, line)
	}
	if maxWidth < 42 {
		line := "gh-manager " + version
		if updateIndicator != "" {
			line += " " + stripANSI(updateIndicator)
		}
		return []string{lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.HeaderText)).Render(line)}
	}
	return out
}

func formatVersionLabel(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return "vdev"
	}
	if strings.HasPrefix(trimmed, "v") {
		return trimmed
	}
	return "v" + trimmed
}

func (m appModel) rightPaneBorderColor() lipgloss.TerminalColor {
	if m.activePane == paneCommands {
		return lipgloss.Color(m.theme.PaneBorderActive)
	}
	return lipgloss.Color(m.theme.PaneBorderInactive)
}

func panelInnerWidth(totalWidth int) int {
	w := totalWidth - 4
	if w < 1 {
		return 1
	}
	return w
}

func fitAndWrapLines(lines []string, maxLines, maxWidth int) []string {
	wrapped := wrapLines(lines, maxWidth)
	return fitLines(wrapped, maxLines)
}

func wrapLines(lines []string, width int) []string {
	if width <= 0 {
		return []string{}
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapLine(line, width)...)
	}
	return out
}

func wrapLine(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{""}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	out := make([]string, 0, 4)
	current := ""
	for _, w := range words {
		for len([]rune(w)) > width {
			if current != "" {
				out = append(out, current)
				current = ""
			}
			r := []rune(w)
			out = append(out, string(r[:width]))
			w = string(r[width:])
		}
		if current == "" {
			current = w
			continue
		}
		candidate := current + " " + w
		if len([]rune(candidate)) <= width {
			current = candidate
			continue
		}
		out = append(out, current)
		current = w
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

func truncateRaw(s string, max int) string {
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

func windowedText(s string, offset, width int) (string, int, int) {
	if width <= 0 {
		return "", 0, 0
	}
	r := []rune(s)
	if len(r) <= width {
		return s, 0, 0
	}
	maxOffset := len(r) - width
	offset = clampInt(offset, 0, maxOffset)
	end := offset + width
	out := append([]rune(nil), r[offset:end]...)
	if offset > 0 && len(out) > 0 {
		out[0] = '…'
	}
	if end < len(r) && len(out) > 0 {
		out[len(out)-1] = '…'
	}
	return string(out), offset, maxOffset
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
