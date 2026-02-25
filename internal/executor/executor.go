package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gh-manager/internal/app"
	"gh-manager/internal/manifest"
	"gh-manager/internal/planfile"
)

const (
	ModeDelete                      = "delete"
	ModeBackup                      = "backup"
	archiveMaxBundleSizeBytes int64 = 100 * 1024 * 1024
)

type Config struct {
	PlanPath          string
	Resume            bool
	BackupDir         string
	MaxDeleteRetries  int
	Mode              string
	DryRun            bool
	ArchiveRepo       string
	ArchiveBranch     string
	ArchiveVisibility string
	NoArchive         bool
}

type Result struct {
	ManifestPath        string
	BackupRoot          string
	Deleted             int
	Failed              int
	ArchiveFailed       int
	ArchiveSkippedSize  int
	Total               int
	ArchiveCommit       string
	ArchiveRepo         string
	ArchiveBranch       string
	ArchiveSkippedRepos []string
}

type Executor struct {
	GH      RepoDeleter
	RepoMgr ArchiveRepoManager
	Backup  BackupProvider
	Archive ArchivePublisher
	Now     func() time.Time
	In      io.Reader
	Out     io.Writer
}

type RepoDeleter interface {
	DeleteRepo(ctx context.Context, fullName string) error
}

type ArchiveRepoManager interface {
	EnsureRepo(ctx context.Context, fullName, visibility string) error
}

type BackupProvider interface {
	MirrorBackup(ctx context.Context, repo planfile.RepoRecord, root string) (string, error)
	CreateBrowsableSnapshot(ctx context.Context, repo planfile.RepoRecord, root string) (string, error)
	CreateBundle(ctx context.Context, repo planfile.RepoRecord, root string) (string, error)
}

type ArchivePublisher interface {
	PublishBundles(ctx context.Context, archiveRepo, branch, backupRoot string, bundles []manifest.BundleArtifact, planFingerprint string) (string, error)
}

func (e Executor) Execute(ctx context.Context, cfg Config, plan planfile.DeletionPlanV1) (Result, error) {
	if e.Now == nil {
		e.Now = time.Now
	}
	if e.In == nil {
		e.In = os.Stdin
	}
	if e.Out == nil {
		e.Out = os.Stdout
	}
	if cfg.MaxDeleteRetries <= 0 {
		cfg.MaxDeleteRetries = 3
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeDelete
	}
	if cfg.Mode != ModeDelete && cfg.Mode != ModeBackup {
		return Result{}, fmt.Errorf("unsupported mode: %s", cfg.Mode)
	}
	if e.Backup == nil {
		return Result{}, errors.New("executor backup service is nil")
	}
	if cfg.Mode == ModeDelete && e.GH == nil {
		return Result{}, errors.New("executor GH client is nil")
	}

	backupRoot, err := e.resolveBackupRoot(plan.Fingerprint, cfg.BackupDir, cfg.Resume)
	if err != nil {
		return Result{}, err
	}

	if err := requireConfirmation(e.In, e.Out, len(plan.Repos), cfg.Mode); err != nil {
		return Result{}, err
	}

	if cfg.DryRun {
		return e.simulate(cfg, plan, backupRoot), nil
	}

	if err := os.MkdirAll(backupRoot, 0o700); err != nil {
		return Result{}, err
	}
	manifestPath := manifest.Path(backupRoot)
	m, err := loadOrCreateManifest(cfg, plan, backupRoot, manifestPath, e.Now)
	if err != nil {
		return Result{}, err
	}

	repoByFullName := make(map[string]planfile.RepoRecord, len(plan.Repos))
	for _, r := range plan.Repos {
		repoByFullName[r.FullName] = r
	}

	archiveBundles := make([]manifest.BundleArtifact, 0)

	for i := range m.RepoExecutions {
		entry := &m.RepoExecutions[i]
		if shouldSkipEntry(cfg.Mode, *entry) {
			continue
		}
		repo, ok := repoByFullName[entry.FullName]
		if !ok {
			entry.Status = manifest.StatusDeleteFailed
			entry.Error = "repo missing from plan"
			entry.Attempts++
			entry.LastAttemptAt = e.Now().UTC().Format(time.RFC3339)
			m.Touch(e.Now())
			_ = manifest.Write(manifestPath, m)
			continue
		}

		if entry.BackupPath == "" || entry.Status == manifest.StatusPending || entry.Status == manifest.StatusBackupFailed {
			fmt.Fprintf(e.Out, "Backing up %s...\n", repo.FullName)
			backupPath, berr := e.Backup.MirrorBackup(ctx, repo, backupRoot)
			entry.Attempts++
			entry.LastAttemptAt = e.Now().UTC().Format(time.RFC3339)
			if berr != nil {
				entry.Status = manifest.StatusBackupFailed
				entry.Error = berr.Error()
				m.Touch(e.Now())
				_ = manifest.Write(manifestPath, m)
				fmt.Fprintf(e.Out, "Backup failed for %s: %v\n", repo.FullName, berr)
				continue
			}
			entry.BackupPath = backupPath
			entry.Status = manifest.StatusBackupOK
			entry.Error = ""
			m.Touch(e.Now())
			if err := manifest.Write(manifestPath, m); err != nil {
				return Result{}, err
			}
		}
		if entry.BrowsablePath == "" {
			fmt.Fprintf(e.Out, "Creating browsable snapshot %s...\n", repo.FullName)
			snapshotPath, serr := e.Backup.CreateBrowsableSnapshot(ctx, repo, backupRoot)
			entry.Attempts++
			entry.LastAttemptAt = e.Now().UTC().Format(time.RFC3339)
			if serr != nil {
				entry.Status = manifest.StatusBackupFailed
				entry.Error = serr.Error()
				m.Touch(e.Now())
				_ = manifest.Write(manifestPath, m)
				fmt.Fprintf(e.Out, "Browsable snapshot failed for %s: %v\n", repo.FullName, serr)
				continue
			}
			entry.BrowsablePath = snapshotPath
			entry.Error = ""
			m.Touch(e.Now())
			if err := manifest.Write(manifestPath, m); err != nil {
				return Result{}, err
			}
		}

		if cfg.Mode == ModeBackup {
			if entry.BundlePath == "" {
				fmt.Fprintf(e.Out, "Creating bundle %s...\n", repo.FullName)
				bundlePath, berr := e.Backup.CreateBundle(ctx, repo, backupRoot)
				entry.Attempts++
				entry.LastAttemptAt = e.Now().UTC().Format(time.RFC3339)
				if berr != nil {
					entry.Status = manifest.StatusBackupFailed
					entry.Error = berr.Error()
					m.Touch(e.Now())
					_ = manifest.Write(manifestPath, m)
					fmt.Fprintf(e.Out, "Bundle failed for %s: %v\n", repo.FullName, berr)
					continue
				}
				entry.BundlePath = bundlePath
				entry.Error = ""
				m.Touch(e.Now())
				if err := manifest.Write(manifestPath, m); err != nil {
					return Result{}, err
				}
			}
			archiveBundles = append(archiveBundles, manifest.BundleArtifact{
				FullName:   repo.FullName,
				BundlePath: entry.BundlePath,
				UpdatedAt:  repo.UpdatedAt,
			})
			continue
		}

		fmt.Fprintf(e.Out, "Deleting %s...\n", repo.FullName)
		var derr error
		for attempt := 1; attempt <= cfg.MaxDeleteRetries; attempt++ {
			derr = e.GH.DeleteRepo(ctx, repo.FullName)
			if derr == nil {
				break
			}
		}
		entry.Attempts++
		entry.LastAttemptAt = e.Now().UTC().Format(time.RFC3339)
		if derr != nil {
			entry.Status = manifest.StatusDeleteFailed
			entry.Error = derr.Error()
			fmt.Fprintf(e.Out, "Delete failed for %s: %v\n", repo.FullName, derr)
		} else {
			entry.Status = manifest.StatusDeleted
			entry.Error = ""
			fmt.Fprintf(e.Out, "Deleted %s\n", repo.FullName)
		}
		m.Touch(e.Now())
		if err := manifest.Write(manifestPath, m); err != nil {
			return Result{}, err
		}
	}

	archiveCommit := ""
	if cfg.Mode == ModeBackup {
		if !cfg.NoArchive && len(archiveBundles) > 0 {
			if cfg.ArchiveRepo == "" {
				cfg.ArchiveRepo = plan.Actor + "/gh-manager-archive"
			}
			if cfg.ArchiveBranch == "" {
				cfg.ArchiveBranch = "main"
			}
			if cfg.ArchiveVisibility == "" {
				cfg.ArchiveVisibility = "private"
			}
			eligibleBundles, sizeSkipped := filterArchiveBundlesBySize(backupRoot, archiveBundles, &m, archiveMaxBundleSizeBytes, e.Out)
			m.Touch(e.Now())
			_ = manifest.Write(manifestPath, m)
			if len(sizeSkipped) > 0 {
				fmt.Fprintf(e.Out, "Archive size-skip: %d bundle(s) moved to %s\n", len(sizeSkipped), filepath.Join(backupRoot, "archive-skipped-size"))
			}
			if len(eligibleBundles) == 0 {
				fmt.Fprintln(e.Out, "No bundles eligible for archive publish after size checks.")
			} else {
				if e.RepoMgr == nil || e.Archive == nil {
					return Result{}, errors.New("archive enabled but archive services are nil")
				}
				if err := e.RepoMgr.EnsureRepo(ctx, cfg.ArchiveRepo, cfg.ArchiveVisibility); err != nil {
					markArchiveFailure(&m, err, eligibleBundles)
					_ = manifest.Write(manifestPath, m)
					return Result{}, err
				}
				archiveCommit, err = e.Archive.PublishBundles(ctx, cfg.ArchiveRepo, cfg.ArchiveBranch, backupRoot, eligibleBundles, plan.Fingerprint)
				if err != nil {
					markArchiveFailure(&m, err, eligibleBundles)
					m.Touch(e.Now())
					_ = manifest.Write(manifestPath, m)
					fmt.Fprintf(e.Out, "Archive publish failed: %v\n", err)
				} else {
					markArchiveSuccess(&m, archiveCommit, eligibleBundles)
					m.Touch(e.Now())
					_ = manifest.Write(manifestPath, m)
				}
			}
		} else {
			markArchiveSkipped(&m)
			_ = manifest.Write(manifestPath, m)
		}
	}

	m.RecomputeCounters()
	_ = manifest.Write(manifestPath, m)

	return Result{
		ManifestPath:        manifestPath,
		BackupRoot:          backupRoot,
		Deleted:             m.DeletedCount,
		Failed:              m.FailedCount,
		ArchiveFailed:       countArchiveFailures(m),
		ArchiveSkippedSize:  countArchiveSkippedSize(m),
		Total:               len(m.RepoExecutions),
		ArchiveCommit:       archiveCommit,
		ArchiveRepo:         cfg.ArchiveRepo,
		ArchiveBranch:       cfg.ArchiveBranch,
		ArchiveSkippedRepos: listArchiveSkippedSizeRepos(m),
	}, nil
}

func shouldSkipEntry(mode string, entry manifest.RepoExecutionEntry) bool {
	if mode == ModeDelete {
		return entry.Status == manifest.StatusDeleted
	}
	return entry.Status == manifest.StatusBackupOK && entry.BundlePath != ""
}

func markArchiveSuccess(m *manifest.ExecutionManifestV1, commit string, targets []manifest.BundleArtifact) {
	targetsSet := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		targetsSet[t.FullName] = struct{}{}
	}
	for i := range m.RepoExecutions {
		entry := &m.RepoExecutions[i]
		if _, ok := targetsSet[entry.FullName]; !ok {
			continue
		}
		if entry.Status == manifest.StatusBackupOK && entry.BundlePath != "" {
			entry.ArchiveStatus = "archived"
			entry.ArchiveCommit = commit
		}
	}
}

func markArchiveFailure(m *manifest.ExecutionManifestV1, err error, targets []manifest.BundleArtifact) {
	targetsSet := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		targetsSet[t.FullName] = struct{}{}
	}
	for i := range m.RepoExecutions {
		entry := &m.RepoExecutions[i]
		if _, ok := targetsSet[entry.FullName]; !ok {
			continue
		}
		if entry.Status == manifest.StatusBackupOK && entry.BundlePath != "" {
			entry.ArchiveStatus = "archive_failed"
			entry.Error = err.Error()
		}
	}
}

func markArchiveSkipped(m *manifest.ExecutionManifestV1) {
	for i := range m.RepoExecutions {
		entry := &m.RepoExecutions[i]
		if entry.Status == manifest.StatusBackupOK && entry.BundlePath != "" && entry.ArchiveStatus == "pending" {
			entry.ArchiveStatus = "skipped"
		}
	}
}

func requireConfirmation(in io.Reader, out io.Writer, count int, mode string) error {
	_ = count
	_ = mode
	fmt.Fprint(out, "Type ACCEPT or CONFIRM to continue: ")
	r := bufio.NewReader(in)
	text, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	input := strings.TrimSpace(strings.ToUpper(text))
	if input != "ACCEPT" && input != "CONFIRM" {
		return errors.New("confirmation phrase mismatch")
	}
	return nil
}

func (e Executor) simulate(cfg Config, plan planfile.DeletionPlanV1, backupRoot string) Result {
	archiveRepo := cfg.ArchiveRepo
	archiveBranch := cfg.ArchiveBranch
	if archiveBranch == "" {
		archiveBranch = "main"
	}
	for _, repo := range plan.Repos {
		fmt.Fprintf(e.Out, "[dry-run] Would mirror backup %s to %s\n", repo.FullName, backupRoot)
		fmt.Fprintf(e.Out, "[dry-run] Would create browsable snapshot for %s\n", repo.FullName)
		if cfg.Mode == ModeBackup {
			fmt.Fprintf(e.Out, "[dry-run] Would create bundle for %s\n", repo.FullName)
		}
		if cfg.Mode == ModeDelete {
			fmt.Fprintf(e.Out, "[dry-run] Would delete %s\n", repo.FullName)
		}
	}
	if cfg.Mode == ModeBackup && !cfg.NoArchive {
		if archiveRepo == "" {
			archiveRepo = plan.Actor + "/gh-manager-archive"
		}
		fmt.Fprintf(e.Out, "[dry-run] Would publish bundles to %s (branch %s)\n", archiveRepo, archiveBranch)
	}
	return Result{
		ManifestPath:        "<dry-run>",
		BackupRoot:          backupRoot,
		Deleted:             0,
		Failed:              0,
		ArchiveFailed:       0,
		ArchiveSkippedSize:  0,
		Total:               len(plan.Repos),
		ArchiveRepo:         archiveRepo,
		ArchiveBranch:       archiveBranch,
		ArchiveSkippedRepos: nil,
	}
}

func countArchiveFailures(m manifest.ExecutionManifestV1) int {
	count := 0
	for _, entry := range m.RepoExecutions {
		if entry.ArchiveStatus == "archive_failed" {
			count++
		}
	}
	return count
}

func countArchiveSkippedSize(m manifest.ExecutionManifestV1) int {
	count := 0
	for _, entry := range m.RepoExecutions {
		if entry.ArchiveStatus == "archive_skipped_size_limit" {
			count++
		}
	}
	return count
}

func listArchiveSkippedSizeRepos(m manifest.ExecutionManifestV1) []string {
	out := make([]string, 0)
	for _, entry := range m.RepoExecutions {
		if entry.ArchiveStatus == "archive_skipped_size_limit" {
			out = append(out, entry.FullName)
		}
	}
	sort.Strings(out)
	return out
}

func filterArchiveBundlesBySize(backupRoot string, bundles []manifest.BundleArtifact, m *manifest.ExecutionManifestV1, maxBytes int64, out io.Writer) ([]manifest.BundleArtifact, []string) {
	eligible := make([]manifest.BundleArtifact, 0, len(bundles))
	skipped := make([]string, 0)
	skippedDir := filepath.Join(backupRoot, "archive-skipped-size")
	_ = os.MkdirAll(skippedDir, 0o700)

	for _, bundle := range bundles {
		info, err := os.Stat(bundle.BundlePath)
		if err != nil {
			if entry := findEntry(m, bundle.FullName); entry != nil {
				entry.ArchiveStatus = "archive_failed"
				entry.Error = "bundle stat failed: " + err.Error()
			}
			continue
		}
		if info.Size() > maxBytes {
			newPath := filepath.Join(skippedDir, filepath.Base(bundle.BundlePath))
			if mvErr := moveFile(bundle.BundlePath, newPath); mvErr != nil {
				if entry := findEntry(m, bundle.FullName); entry != nil {
					entry.ArchiveStatus = "archive_failed"
					entry.Error = "failed moving oversized bundle: " + mvErr.Error()
				}
				continue
			}
			if entry := findEntry(m, bundle.FullName); entry != nil {
				entry.BundlePath = newPath
				entry.ArchiveStatus = "archive_skipped_size_limit"
				entry.Error = fmt.Sprintf("bundle size %d exceeds archive limit %d bytes", info.Size(), maxBytes)
			}
			skipped = append(skipped, bundle.FullName)
			fmt.Fprintf(out, "Archive skip (size): %s (%d bytes)\n", bundle.FullName, info.Size())
			continue
		}
		eligible = append(eligible, bundle)
	}
	return eligible, skipped
}

func findEntry(m *manifest.ExecutionManifestV1, fullName string) *manifest.RepoExecutionEntry {
	for i := range m.RepoExecutions {
		if m.RepoExecutions[i].FullName == fullName {
			return &m.RepoExecutions[i]
		}
	}
	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

func loadOrCreateManifest(cfg Config, plan planfile.DeletionPlanV1, backupRoot, manifestPath string, now func() time.Time) (manifest.ExecutionManifestV1, error) {
	if _, err := os.Stat(manifestPath); err == nil {
		if !cfg.Resume {
			return manifest.ExecutionManifestV1{}, errors.New("manifest already exists and --resume=false")
		}
		m, err := manifest.Read(manifestPath)
		if err != nil {
			return manifest.ExecutionManifestV1{}, err
		}
		if m.PlanFingerprint != plan.Fingerprint {
			return manifest.ExecutionManifestV1{}, errors.New("manifest plan fingerprint mismatch")
		}
		if m.Mode != "" && cfg.Mode != "" && m.Mode != cfg.Mode {
			return manifest.ExecutionManifestV1{}, errors.New("manifest mode mismatch")
		}
		return m, nil
	}
	m := manifest.New(cfg.PlanPath, backupRoot, plan, now(), manifest.NewOptions{
		Mode:          cfg.Mode,
		ArchiveRepo:   cfg.ArchiveRepo,
		ArchiveBranch: cfg.ArchiveBranch,
	})
	if err := manifest.Write(manifestPath, m); err != nil {
		return manifest.ExecutionManifestV1{}, err
	}
	return m, nil
}

func (e Executor) resolveBackupRoot(fingerprint, explicit string, resume bool) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if resume {
		if found := findExistingBackupRoot(fingerprint); found != "" {
			return found, nil
		}
	}
	return app.DefaultBackupRoot(e.Now())
}

func findExistingBackupRoot(fingerprint string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		return ""
	}
	candidates := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "gh-manager-archive-") {
			continue
		}
		root := filepath.Join(home, entry.Name())
		m, err := manifest.Read(manifest.Path(root))
		if err != nil {
			continue
		}
		if m.PlanFingerprint == fingerprint {
			candidates = append(candidates, root)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	return candidates[len(candidates)-1]
}
