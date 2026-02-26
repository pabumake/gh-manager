package restore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls [][]string
	fail  map[string]error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	key := strings.Join(call, " ")
	if err, ok := f.fail[key]; ok {
		if err != nil {
			return nil, err
		}
		return []byte("ok"), nil
	}
	if strings.HasPrefix(key, "gh repo view") {
		return nil, errors.New("HTTP 404: Not Found")
	}
	return []byte("ok"), nil
}

func TestRestoreBundleSuccess(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "alice__repo.bundle")
	if err := os.WriteFile(bundle, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{fail: map[string]error{}}
	s := NewService(r)
	res, err := s.Restore(context.Background(), Request{
		SourceKind:       "bundle",
		SourcePath:       bundle,
		TargetOwner:      "alice",
		TargetName:       "repo-restored",
		TargetVisibility: "private",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TargetFullName != "alice/repo-restored" {
		t.Fatalf("unexpected target: %s", res.TargetFullName)
	}
	joined := flatten(r.calls)
	mustContain(t, joined, "git clone "+bundle)
	mustContain(t, joined, "gh repo create alice/repo-restored --private --confirm")
	mustContain(t, joined, "git -C "+res.WorkDir+" push --all origin")
	mustContain(t, joined, "git -C "+res.WorkDir+" push --tags origin")
}

func TestRestoreConflict(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{
		"gh repo view alice/existing --json name --jq .name": nil,
	}}
	s := NewService(r)
	root := t.TempDir()
	bundle := filepath.Join(root, "alice__repo.bundle")
	if err := os.WriteFile(bundle, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.Restore(context.Background(), Request{
		SourceKind:  "bundle",
		SourcePath:  bundle,
		TargetOwner: "alice",
		TargetName:  "existing",
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var c TargetExistsError
	if !errors.As(err, &c) {
		t.Fatalf("expected TargetExistsError, got %T", err)
	}
	if c.SuggestedName() != "existing-ghm" {
		t.Fatalf("unexpected suggestion: %s", c.SuggestedName())
	}
}

func TestRestoreSnapshotFallback(t *testing.T) {
	root := t.TempDir()
	snap := filepath.Join(root, "snapshot")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{fail: map[string]error{}}
	s := NewService(r)
	_, err := s.Restore(context.Background(), Request{
		SourceKind:  "snapshot",
		SourcePath:  snap,
		TargetOwner: "alice",
		TargetName:  "from-snap",
	})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, flatten(r.calls), "git clone "+snap)
}

func flatten(calls [][]string) string {
	rows := make([]string, 0, len(calls))
	for _, c := range calls {
		rows = append(rows, strings.Join(c, " "))
	}
	return strings.Join(rows, "\n")
}

func mustContain(t *testing.T, text, sub string) {
	t.Helper()
	if !strings.Contains(text, sub) {
		t.Fatalf("missing %q in:\n%s", sub, text)
	}
}
