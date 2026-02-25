package backup

import (
	"path/filepath"
	"testing"

	"gh-manager/internal/planfile"
)

func TestBundlePath(t *testing.T) {
	repo := planfile.RepoRecord{Owner: "alice", Name: "demo"}
	got := BundlePath("/tmp/root", repo)
	want := filepath.Join("/tmp/root", "bundles", "alice__demo.bundle")
	if got != want {
		t.Fatalf("bundle path mismatch: got=%s want=%s", got, want)
	}
}

func TestSnapshotPath(t *testing.T) {
	repo := planfile.RepoRecord{Owner: "alice", Name: "demo"}
	got := SnapshotPath("/tmp/root", repo)
	want := filepath.Join("/tmp/root", "snapshots", "alice__demo")
	if got != want {
		t.Fatalf("snapshot path mismatch: got=%s want=%s", got, want)
	}
}
