package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gh-manager/internal/manifest"
	"gh-manager/internal/planfile"
)

type fakeGH struct {
	deleted   []string
	failFor   map[string]error
	attempts  map[string]int
	failCount map[string]int
	ensureErr error
	ensured   []string
}

func (f *fakeGH) DeleteRepo(_ context.Context, fullName string) error {
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.attempts[fullName]++
	if c, ok := f.failCount[fullName]; ok && c > 0 {
		f.failCount[fullName] = c - 1
		return errors.New("transient delete error")
	}
	if err := f.failFor[fullName]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, fullName)
	return nil
}

func (f *fakeGH) EnsureRepo(_ context.Context, _ string, _ string) error {
	f.ensured = append(f.ensured, "called")
	return f.ensureErr
}

type fakeBackup struct {
	paths      map[string]string
	snapshots  map[string]string
	bundlePath map[string]string
	failFor    map[string]error
	snapFail   map[string]error
	bundleFail map[string]error
	mirrorN    int
	snapshotN  int
	bundleN    int
}

func (f *fakeBackup) MirrorBackup(_ context.Context, repo planfile.RepoRecord, _ string) (string, error) {
	f.mirrorN++
	if err := f.failFor[repo.FullName]; err != nil {
		return "", err
	}
	p := f.paths[repo.FullName]
	if p == "" {
		p = "/tmp/" + strings.ReplaceAll(repo.Name, "/", "_") + ".git"
	}
	return p, nil
}

func (f *fakeBackup) CreateBundle(_ context.Context, repo planfile.RepoRecord, _ string) (string, error) {
	f.bundleN++
	if err := f.bundleFail[repo.FullName]; err != nil {
		return "", err
	}
	if f.bundlePath[repo.FullName] != "" {
		return f.bundlePath[repo.FullName], nil
	}
	return "/tmp/" + strings.ReplaceAll(repo.Name, "/", "_") + ".bundle", nil
}

func (f *fakeBackup) CreateBrowsableSnapshot(_ context.Context, repo planfile.RepoRecord, _ string) (string, error) {
	f.snapshotN++
	if err := f.snapFail[repo.FullName]; err != nil {
		return "", err
	}
	if f.snapshots[repo.FullName] != "" {
		return f.snapshots[repo.FullName], nil
	}
	return "/tmp/" + strings.ReplaceAll(repo.Name, "/", "_"), nil
}

type fakeArchive struct {
	commit string
	err    error
	calls  int
}

func (f *fakeArchive) PublishBundles(_ context.Context, _ string, _ string, _ string, _ []manifest.BundleArtifact, _ string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	if f.commit != "" {
		return f.commit, nil
	}
	return "abc123", nil
}

func TestExecuteDeleteDryRunHasNoSideEffects(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "r1", FullName: "alice/r1"}}, now)
	plan.Fingerprint = "fp-dry-delete"
	backupRoot := t.TempDir()
	gh := &fakeGH{}
	bk := &fakeBackup{bundlePath: map[string]string{}}
	out := &strings.Builder{}
	ex := Executor{GH: gh, Backup: bk, Now: func() time.Time { return now }, In: strings.NewReader("ACCEPT\n"), Out: out}
	res, err := ex.Execute(context.Background(), Config{PlanPath: "plan.json", Resume: true, BackupDir: backupRoot, Mode: ModeDelete, DryRun: true}, plan)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "[dry-run] Would delete alice/r1") {
		t.Fatalf("expected dry-run output, got: %s", out.String())
	}
	if len(gh.deleted) != 0 || bk.mirrorN != 0 || bk.snapshotN != 0 || bk.bundleN != 0 {
		t.Fatalf("expected no side-effect calls: deleted=%v mirror=%d snapshot=%d bundle=%d", gh.deleted, bk.mirrorN, bk.snapshotN, bk.bundleN)
	}
	if res.ManifestPath != "<dry-run>" {
		t.Fatalf("expected dry-run manifest marker, got %s", res.ManifestPath)
	}
}

func TestExecuteBackupDryRunHasNoSideEffects(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "r1", FullName: "alice/r1"}}, now)
	plan.Fingerprint = "fp-dry-backup"
	backupRoot := t.TempDir()
	gh := &fakeGH{}
	bk := &fakeBackup{bundlePath: map[string]string{}}
	arc := &fakeArchive{}
	out := &strings.Builder{}
	ex := Executor{RepoMgr: gh, Backup: bk, Archive: arc, Now: func() time.Time { return now }, In: strings.NewReader("ACCEPT\n"), Out: out}
	_, err := ex.Execute(context.Background(), Config{PlanPath: "plan.json", Resume: true, BackupDir: backupRoot, Mode: ModeBackup, DryRun: true}, plan)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "[dry-run] Would create bundle for alice/r1") {
		t.Fatalf("expected dry-run output, got: %s", out.String())
	}
	if bk.mirrorN != 0 || bk.snapshotN != 0 || bk.bundleN != 0 || arc.calls != 0 || len(gh.ensured) != 0 {
		t.Fatalf("expected no side-effect calls: mirror=%d snapshot=%d bundle=%d archiveCalls=%d ensured=%d", bk.mirrorN, bk.snapshotN, bk.bundleN, arc.calls, len(gh.ensured))
	}
}

func TestRequireConfirmation(t *testing.T) {
	if err := requireConfirmation(strings.NewReader("ACCEPT\n"), &strings.Builder{}, 2, ModeDelete); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := requireConfirmation(strings.NewReader("CONFIRM\n"), &strings.Builder{}, 2, ModeBackup); err != nil {
		t.Fatalf("expected confirm success, got %v", err)
	}
}

func TestLoadOrCreateManifest(t *testing.T) {
	d := t.TempDir()
	p := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{FullName: "alice/r1"}}, time.Now())
	p.Fingerprint = "fp"
	m, err := loadOrCreateManifest(Config{PlanPath: "plan.json", Resume: true, Mode: ModeDelete}, p, d, manifest.Path(d), time.Now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if m.PlanFingerprint != "fp" {
		t.Fatalf("bad fingerprint")
	}
	if _, err := loadOrCreateManifest(Config{PlanPath: "plan.json", Resume: false, Mode: ModeDelete}, p, d, manifest.Path(d), time.Now); err == nil {
		t.Fatal("expected resume=false failure")
	}
}

func TestExecuteDeleteSuccessWithRetryAndResumeSkip(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "r1", FullName: "alice/r1"}, {Owner: "alice", Name: "r2", FullName: "alice/r2"}}, now)
	plan.Fingerprint = "fp-1"

	backupRoot := t.TempDir()
	gh := &fakeGH{failCount: map[string]int{"alice/r1": 1}}
	bk := &fakeBackup{bundlePath: map[string]string{}}
	ex := Executor{GH: gh, Backup: bk, Now: func() time.Time { return now }, In: strings.NewReader("ACCEPT\n"), Out: &strings.Builder{}}
	res, err := ex.Execute(context.Background(), Config{PlanPath: "plan.json", Resume: true, BackupDir: backupRoot, Mode: ModeDelete, MaxDeleteRetries: 3}, plan)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.Deleted != 2 || res.Failed != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(gh.deleted) != 2 || !slices.Contains(gh.deleted, "alice/r1") || !slices.Contains(gh.deleted, "alice/r2") {
		t.Fatalf("unexpected deleted set: %#v", gh.deleted)
	}
	if gh.attempts["alice/r1"] != 2 {
		t.Fatalf("expected retry for alice/r1")
	}

	ex2 := Executor{GH: &fakeGH{}, Backup: bk, Now: func() time.Time { return now.Add(time.Minute) }, In: strings.NewReader("ACCEPT\n"), Out: &strings.Builder{}}
	res2, err := ex2.Execute(context.Background(), Config{PlanPath: "plan.json", Resume: true, BackupDir: backupRoot, Mode: ModeDelete}, plan)
	if err != nil {
		t.Fatalf("resume execute failed: %v", err)
	}
	if res2.Deleted != 2 {
		t.Fatalf("resume should keep deleted=2: %+v", res2)
	}
}

func TestExecuteBackupSuccessWithArchive(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "r1", FullName: "alice/r1", UpdatedAt: now.Format(time.RFC3339)}}, now)
	plan.Fingerprint = "fp-backup"
	backupRoot := t.TempDir()

	gh := &fakeGH{}
	bundlePath := filepath.Join(backupRoot, "r1.bundle")
	if err := os.WriteFile(bundlePath, []byte("bundle"), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	bk := &fakeBackup{bundlePath: map[string]string{"alice/r1": bundlePath}}
	arc := &fakeArchive{commit: "deadbeef"}
	ex := Executor{RepoMgr: gh, Backup: bk, Archive: arc, Now: func() time.Time { return now }, In: strings.NewReader("CONFIRM\n"), Out: &strings.Builder{}}

	res, err := ex.Execute(context.Background(), Config{
		PlanPath:          "plan.json",
		Resume:            true,
		BackupDir:         backupRoot,
		Mode:              ModeBackup,
		ArchiveRepo:       "alice/gh-manager-archive",
		ArchiveBranch:     "main",
		ArchiveVisibility: "private",
	}, plan)
	if err != nil {
		t.Fatalf("backup execute failed: %v", err)
	}
	if res.ArchiveCommit == "" {
		t.Fatal("expected archive commit")
	}
	m, err := manifest.Read(filepath.Join(backupRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if m.Mode != ModeBackup {
		t.Fatalf("expected mode backup, got %s", m.Mode)
	}
	if m.RepoExecutions[0].BrowsablePath == "" {
		t.Fatalf("expected browsable path")
	}
	if m.RepoExecutions[0].ArchiveStatus != "archived" {
		t.Fatalf("expected archived status, got %s", m.RepoExecutions[0].ArchiveStatus)
	}
}

func TestExecuteBackupFailureSkipsArchive(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "r1", FullName: "alice/r1"}}, now)
	plan.Fingerprint = "fp-bf"
	backupRoot := t.TempDir()

	ex := Executor{
		RepoMgr: &fakeGH{},
		Backup:  &fakeBackup{bundlePath: map[string]string{}, bundleFail: map[string]error{"alice/r1": errors.New("bundle fail")}},
		Archive: &fakeArchive{},
		Now:     func() time.Time { return now },
		In:      strings.NewReader("ACCEPT\n"),
		Out:     &strings.Builder{},
	}
	res, err := ex.Execute(context.Background(), Config{PlanPath: "plan.json", Resume: true, BackupDir: backupRoot, Mode: ModeBackup, ArchiveRepo: "alice/gh-manager-archive"}, plan)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.Failed != 1 {
		t.Fatalf("expected failed=1, got %+v", res)
	}
	m, err := manifest.Read(filepath.Join(backupRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if m.RepoExecutions[0].Status != manifest.StatusBackupFailed {
		t.Fatalf("expected backup_failed, got %s", m.RepoExecutions[0].Status)
	}
}

func TestExecuteBackupArchivePublishFailureCounted(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "r1", FullName: "alice/r1"}}, now)
	plan.Fingerprint = "fp-archive-fail"
	backupRoot := t.TempDir()

	ex := Executor{
		RepoMgr: &fakeGH{},
		Backup:  &fakeBackup{bundlePath: map[string]string{"alice/r1": filepath.Join(backupRoot, "r1.bundle")}},
		Archive: &fakeArchive{err: errors.New("push rejected")},
		Now:     func() time.Time { return now },
		In:      strings.NewReader("ACCEPT\n"),
		Out:     &strings.Builder{},
	}
	if err := os.WriteFile(filepath.Join(backupRoot, "r1.bundle"), []byte("bundle"), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	res, err := ex.Execute(context.Background(), Config{
		PlanPath:    "plan.json",
		Resume:      true,
		BackupDir:   backupRoot,
		Mode:        ModeBackup,
		ArchiveRepo: "alice/gh-manager-archive",
	}, plan)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.Failed != 0 {
		t.Fatalf("expected local_failed=0, got %d", res.Failed)
	}
	if res.ArchiveFailed != 1 {
		t.Fatalf("expected archive_failed=1, got %d", res.ArchiveFailed)
	}
}

func TestExecuteBackupArchiveSkipsOversizedBundles(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	plan := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{Owner: "alice", Name: "small", FullName: "alice/small"}, {Owner: "alice", Name: "big", FullName: "alice/big"}}, now)
	plan.Fingerprint = "fp-size-skip"
	backupRoot := t.TempDir()

	small := filepath.Join(backupRoot, "small.bundle")
	big := filepath.Join(backupRoot, "big.bundle")
	if err := os.WriteFile(small, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write small bundle: %v", err)
	}
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("create big bundle: %v", err)
	}
	if _, err := f.Seek(archiveMaxBundleSizeBytes+1, 0); err != nil {
		t.Fatalf("seek big bundle: %v", err)
	}
	if _, err := f.Write([]byte{0}); err != nil {
		t.Fatalf("write big bundle: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close big bundle: %v", err)
	}

	arc := &fakeArchive{commit: "ok-commit"}
	ex := Executor{
		RepoMgr: &fakeGH{},
		Backup: &fakeBackup{bundlePath: map[string]string{
			"alice/small": small,
			"alice/big":   big,
		}},
		Archive: arc,
		Now:     func() time.Time { return now },
		In:      strings.NewReader("ACCEPT\n"),
		Out:     &strings.Builder{},
	}

	res, err := ex.Execute(context.Background(), Config{
		PlanPath:    "plan.json",
		Resume:      true,
		BackupDir:   backupRoot,
		Mode:        ModeBackup,
		ArchiveRepo: "alice/gh-manager-archive",
	}, plan)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.ArchiveSkippedSize != 1 {
		t.Fatalf("expected one size-skip, got %d", res.ArchiveSkippedSize)
	}
	if res.ArchiveFailed != 0 {
		t.Fatalf("expected no archive failure for eligible bundles, got %d", res.ArchiveFailed)
	}
	if arc.calls != 1 {
		t.Fatalf("expected archive publish for eligible bundle, calls=%d", arc.calls)
	}
	m, err := manifest.Read(filepath.Join(backupRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var sawSkipped bool
	for _, e := range m.RepoExecutions {
		if e.FullName == "alice/big" {
			if e.ArchiveStatus != "archive_skipped_size_limit" {
				t.Fatalf("expected size skip status, got %s", e.ArchiveStatus)
			}
			if !strings.Contains(e.BundlePath, "archive-skipped-size") {
				t.Fatalf("expected moved bundle path, got %s", e.BundlePath)
			}
			sawSkipped = true
		}
	}
	if !sawSkipped {
		t.Fatal("did not find skipped entry")
	}
}
