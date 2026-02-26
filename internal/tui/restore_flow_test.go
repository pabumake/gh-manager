package tui

import (
	"os"
	"path/filepath"
	"testing"

	restorepkg "gh-manager/internal/restore"
)

func TestParseYesNo(t *testing.T) {
	cases := []struct {
		in    string
		yes   bool
		valid bool
	}{
		{"y", true, true},
		{"YES", true, true},
		{"No", false, true},
		{"x", false, false},
	}
	for _, c := range cases {
		yes, valid := parseYesNo(c.in)
		if yes != c.yes || valid != c.valid {
			t.Fatalf("parseYesNo(%q) = (%v,%v), want (%v,%v)", c.in, yes, valid, c.yes, c.valid)
		}
	}
}

func TestRestoreFlowConflictReopensRename(t *testing.T) {
	m := newAppModel(nil, AppCallbacks{})
	m.restoreState = restoreState{active: true, stage: restoreStageAskUseOriginal}
	updated, _ := m.Update(commandResultMsg{err: restorepkg.TargetExistsError{TargetFullName: "alice/existing", Suggested: "existing-ghm"}})
	m2 := updated.(appModel)
	if m2.restoreState.stage != restoreStageInputNewName {
		t.Fatalf("expected rename stage, got %v", m2.restoreState.stage)
	}
	if m2.restoreState.promptInput != "existing-ghm" {
		t.Fatalf("unexpected suggested prompt: %q", m2.restoreState.promptInput)
	}
	if !m2.modalActive || m2.modalKind != modalRestoreRename {
		t.Fatalf("expected rename modal active")
	}
}

func TestStartRestoreFlowUsesConfiguredPath(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "gh-archive-test")
	if err := os.MkdirAll(filepath.Join(archive, "bundles"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := newAppModel(nil, AppCallbacks{RestoreDefaultArchiveDir: archive})
	_ = m.startRestoreFlow()
	if !m.restoreState.active {
		t.Fatal("restore flow should be active")
	}
	if m.restoreState.browserDir != archive {
		t.Fatalf("unexpected browser dir: %s", m.restoreState.browserDir)
	}
	if !m.modalActive || m.modalKind != modalRestoreBrowse {
		t.Fatalf("expected restore browser modal to be active")
	}
}
