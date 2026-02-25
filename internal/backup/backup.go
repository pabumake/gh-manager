package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gh-manager/internal/app"
	"gh-manager/internal/manifest"
	"gh-manager/internal/planfile"
)

type Service struct {
	runner app.CommandRunner
}

func NewService(r app.CommandRunner) Service {
	return Service{runner: r}
}

func MirrorPath(root string, repo planfile.RepoRecord) string {
	name := strings.ReplaceAll(repo.Name, "/", "_")
	return filepath.Join(root, name+".git")
}

func BundlePath(root string, repo planfile.RepoRecord) string {
	name := strings.ReplaceAll(repo.Name, "/", "_")
	owner := strings.ReplaceAll(repo.Owner, "/", "_")
	return filepath.Join(root, "bundles", owner+"__"+name+".bundle")
}

func SnapshotPath(root string, repo planfile.RepoRecord) string {
	name := strings.ReplaceAll(repo.Name, "/", "_")
	owner := strings.ReplaceAll(repo.Owner, "/", "_")
	return filepath.Join(root, "snapshots", owner+"__"+name)
}

func (s Service) MirrorBackup(ctx context.Context, repo planfile.RepoRecord, root string) (string, error) {
	dst := MirrorPath(root, repo)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	url := "git@github.com:" + repo.FullName + ".git"
	_, err := s.runner.Run(ctx, "git", "clone", "--mirror", url, dst)
	if err != nil {
		return "", err
	}
	return dst, nil
}

func (s Service) CreateBundle(ctx context.Context, repo planfile.RepoRecord, root string) (string, error) {
	mirror, err := s.MirrorBackup(ctx, repo, root)
	if err != nil {
		return "", err
	}
	bundle := BundlePath(root, repo)
	if err := os.MkdirAll(filepath.Dir(bundle), 0o700); err != nil {
		return "", err
	}
	_, err = s.runner.Run(ctx, "git", "-C", mirror, "bundle", "create", bundle, "--all")
	if err != nil {
		return "", err
	}
	return bundle, nil
}

func (s Service) CreateBrowsableSnapshot(ctx context.Context, repo planfile.RepoRecord, root string) (string, error) {
	mirror, err := s.MirrorBackup(ctx, repo, root)
	if err != nil {
		return "", err
	}
	snapshot := SnapshotPath(root, repo)
	if _, err := os.Stat(snapshot); err == nil {
		return snapshot, nil
	}
	if err := os.MkdirAll(filepath.Dir(snapshot), 0o700); err != nil {
		return "", err
	}
	_, err = s.runner.Run(ctx, "git", "clone", mirror, snapshot)
	if err != nil {
		return "", err
	}
	return snapshot, nil
}

type ArchiveService struct {
	runner app.CommandRunner
	now    func() time.Time
}

func NewArchiveService(r app.CommandRunner) ArchiveService {
	return ArchiveService{runner: r, now: time.Now}
}

type archiveManifest struct {
	PlanFingerprint string                 `json:"planFingerprint"`
	CreatedAt       string                 `json:"createdAt"`
	Bundles         []archiveManifestEntry `json:"bundles"`
}

type archiveManifestEntry struct {
	FullName   string `json:"fullName"`
	BundleFile string `json:"bundleFile"`
	SHA256     string `json:"sha256"`
	UpdatedAt  string `json:"updatedAt"`
}

func (a ArchiveService) PublishBundles(ctx context.Context, archiveRepo, branch, backupRoot string, bundles []manifest.BundleArtifact, planFingerprint string) (string, error) {
	if len(bundles) == 0 {
		return "", nil
	}
	if branch == "" {
		branch = "main"
	}
	if a.now == nil {
		a.now = time.Now
	}

	workdir, err := os.MkdirTemp("", "gh-manager-archive-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workdir)

	cloneDir := filepath.Join(workdir, "repo")
	if _, err := a.runner.Run(ctx, "gh", "repo", "clone", archiveRepo, cloneDir); err != nil {
		return "", err
	}
	if _, err := a.runner.Run(ctx, "git", "-C", cloneDir, "checkout", "-B", branch); err != nil {
		return "", err
	}

	timestamp := a.now().UTC().Format("2006-01-02-150405")
	archiveRoot := filepath.Join(cloneDir, "archives", timestamp)
	bundlesDir := filepath.Join(archiveRoot, "bundles")
	if err := os.MkdirAll(bundlesDir, 0o755); err != nil {
		return "", err
	}

	entries := make([]archiveManifestEntry, 0, len(bundles))
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].FullName < bundles[j].FullName })
	for _, b := range bundles {
		dst := filepath.Join(bundlesDir, filepath.Base(b.BundlePath))
		content, readErr := os.ReadFile(b.BundlePath)
		if readErr != nil {
			return "", readErr
		}
		if writeErr := os.WriteFile(dst, content, 0o644); writeErr != nil {
			return "", writeErr
		}
		h := sha256.Sum256(content)
		entries = append(entries, archiveManifestEntry{
			FullName:   b.FullName,
			BundleFile: filepath.ToSlash(filepath.Join("bundles", filepath.Base(b.BundlePath))),
			SHA256:     hex.EncodeToString(h[:]),
			UpdatedAt:  b.UpdatedAt,
		})
	}

	man := archiveManifest{
		PlanFingerprint: planFingerprint,
		CreatedAt:       a.now().UTC().Format(time.RFC3339),
		Bundles:         entries,
	}
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return "", err
	}
	manBytes = append(manBytes, '\n')
	if err := os.WriteFile(filepath.Join(archiveRoot, "manifest.json"), manBytes, 0o644); err != nil {
		return "", err
	}

	if _, err := a.runner.Run(ctx, "git", "-C", cloneDir, "add", "."); err != nil {
		return "", err
	}
	msg := fmt.Sprintf("backup: %d repos from plan %s", len(entries), shortFingerprint(planFingerprint))
	if _, err := a.runner.Run(ctx, "git", "-C", cloneDir, "commit", "-m", msg); err != nil {
		return "", err
	}
	if _, err := a.runner.Run(ctx, "git", "-C", cloneDir, "push", "origin", branch); err != nil {
		return "", err
	}
	sha, err := a.runner.Run(ctx, "git", "-C", cloneDir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(sha)), nil
}

func shortFingerprint(fp string) string {
	if len(fp) <= 10 {
		return fp
	}
	return fp[:10]
}
