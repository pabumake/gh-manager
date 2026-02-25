package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"gh-manager/internal/planfile"
)

type RepoExecutionStatus string

const (
	StatusPending      RepoExecutionStatus = "pending"
	StatusBackupOK     RepoExecutionStatus = "backup_ok"
	StatusDeleted      RepoExecutionStatus = "deleted"
	StatusBackupFailed RepoExecutionStatus = "backup_failed"
	StatusDeleteFailed RepoExecutionStatus = "delete_failed"
)

type RepoExecutionEntry struct {
	FullName      string              `json:"fullName"`
	Status        RepoExecutionStatus `json:"status"`
	BackupPath    string              `json:"backupPath,omitempty"`
	BrowsablePath string              `json:"browsablePath,omitempty"`
	BundlePath    string              `json:"bundlePath,omitempty"`
	ArchiveCommit string              `json:"archiveCommit,omitempty"`
	ArchiveStatus string              `json:"archiveStatus,omitempty"`
	Error         string              `json:"error,omitempty"`
	Attempts      int                 `json:"attempts"`
	LastAttemptAt string              `json:"lastAttemptAt,omitempty"`
}

type ExecutionManifestV1 struct {
	SchemaVersion    string               `json:"schemaVersion"`
	Mode             string               `json:"mode"`
	PlanFingerprint  string               `json:"planFingerprint"`
	PlanPath         string               `json:"planPath"`
	Actor            string               `json:"actor"`
	Host             string               `json:"host"`
	ArchiveRepo      string               `json:"archiveRepo,omitempty"`
	ArchiveBranch    string               `json:"archiveBranch,omitempty"`
	BackupRoot       string               `json:"backupRoot"`
	CreatedAt        string               `json:"createdAt"`
	UpdatedAt        string               `json:"updatedAt"`
	RepoExecutions   []RepoExecutionEntry `json:"repoExecutions"`
	DeletedCount     int                  `json:"deletedCount"`
	FailedCount      int                  `json:"failedCount"`
	SkippedFailCount int                  `json:"skippedFailCount"`
}

type NewOptions struct {
	Mode          string
	ArchiveRepo   string
	ArchiveBranch string
}

type BundleArtifact struct {
	FullName   string `json:"fullName"`
	BundlePath string `json:"bundlePath"`
	UpdatedAt  string `json:"updatedAt"`
}

func New(planPath, backupRoot string, p planfile.DeletionPlanV1, now time.Time, opts NewOptions) ExecutionManifestV1 {
	repos := make([]RepoExecutionEntry, 0, len(p.Repos))
	for _, r := range p.Repos {
		archiveStatus := "skipped"
		if opts.Mode == "backup" {
			archiveStatus = "pending"
		}
		repos = append(repos, RepoExecutionEntry{
			FullName:      r.FullName,
			Status:        StatusPending,
			ArchiveStatus: archiveStatus,
		})
	}
	ts := now.UTC().Format(time.RFC3339)
	return ExecutionManifestV1{
		SchemaVersion:   "v1",
		Mode:            opts.Mode,
		PlanFingerprint: p.Fingerprint,
		PlanPath:        planPath,
		Actor:           p.Actor,
		Host:            p.Host,
		ArchiveRepo:     opts.ArchiveRepo,
		ArchiveBranch:   opts.ArchiveBranch,
		BackupRoot:      backupRoot,
		CreatedAt:       ts,
		UpdatedAt:       ts,
		RepoExecutions:  repos,
	}
}

func Path(backupRoot string) string {
	return filepath.Join(backupRoot, "manifest.json")
}

func Read(path string) (ExecutionManifestV1, error) {
	var m ExecutionManifestV1
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func Write(path string, m ExecutionManifestV1) error {
	m.RecomputeCounters()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func (m *ExecutionManifestV1) Touch(now time.Time) {
	m.UpdatedAt = now.UTC().Format(time.RFC3339)
}

func (m *ExecutionManifestV1) RecomputeCounters() {
	var deleted, failed int
	for _, r := range m.RepoExecutions {
		if r.Status == StatusDeleted {
			deleted++
		}
		if r.Status == StatusBackupFailed || r.Status == StatusDeleteFailed {
			failed++
		}
	}
	m.DeletedCount = deleted
	m.FailedCount = failed
	m.SkippedFailCount = failed
}
