package manifest

import (
	"path/filepath"
	"testing"
	"time"

	"gh-manager/internal/planfile"
)

func TestManifestWriteReadAndCounters(t *testing.T) {
	p := planfile.New("alice", "github.com", "test", []planfile.RepoRecord{{FullName: "alice/r1"}, {FullName: "alice/r2"}}, time.Now())
	p.Fingerprint = "fp"
	m := New("/tmp/plan.json", t.TempDir(), p, time.Now(), NewOptions{Mode: "delete"})
	m.RepoExecutions[0].Status = StatusDeleted
	m.RepoExecutions[1].Status = StatusBackupFailed
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := Write(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if loaded.DeletedCount != 1 || loaded.FailedCount != 1 {
		t.Fatalf("unexpected counters: %+v", loaded)
	}
}
