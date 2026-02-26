package restore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIndexFromManifestAndFallback(t *testing.T) {
	root := t.TempDir()
	manifest := `{
  "schemaVersion":"v1",
  "mode":"backup",
  "repoExecutions":[
    {"fullName":"alice/repo1","bundlePath":"` + filepath.ToSlash(filepath.Join(root, "bundles", "alice__repo1.bundle")) + `","browsablePath":"` + filepath.ToSlash(filepath.Join(root, "snapshots", "alice__repo1")) + `"}
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bundles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bundles", "alice__repo1.bundle"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bundles", "alice__repo2.bundle"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].FullName != "alice/repo1" || entries[1].FullName != "alice/repo2" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestPreferredSourceBundleFirst(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "x.bundle")
	snap := filepath.Join(root, "snap")
	if err := os.WriteFile(bundle, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}

	src, ok := PreferredSource(ArchiveEntry{FullName: "a/b", BundlePath: bundle, SnapshotPath: snap})
	if !ok {
		t.Fatal("expected source")
	}
	if src.Kind != "bundle" || src.Path != bundle {
		t.Fatalf("unexpected source: %#v", src)
	}
}

func TestBundleNameToFullName(t *testing.T) {
	got, ok := bundleNameToFullName("alice__my_repo.bundle")
	if !ok {
		t.Fatal("expected parse ok")
	}
	if got != "alice/my_repo" {
		t.Fatalf("unexpected full name: %s", got)
	}
}
