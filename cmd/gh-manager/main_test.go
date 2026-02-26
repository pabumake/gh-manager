package main

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	out []byte
	err error
}

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestCompareSemverLabels(t *testing.T) {
	if got := compareSemverLabels("v0.1.0", "v0.1.1"); got >= 0 {
		t.Fatalf("expected v0.1.0 < v0.1.1, got %d", got)
	}
	if got := compareSemverLabels("0.2.0", "v0.1.9"); got <= 0 {
		t.Fatalf("expected 0.2.0 > v0.1.9, got %d", got)
	}
	if got := compareSemverLabels("v0.1.0-rc1", "0.1.0"); got != 0 {
		t.Fatalf("expected prerelease suffix to normalize equal, got %d", got)
	}
}

func TestCheckLatestRelease(t *testing.T) {
	r := fakeRunner{out: []byte(`{"tag_name":"v0.1.3","html_url":"https://example.test/release"}`)}
	info, err := checkLatestRelease(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.UpdateAvailable {
		t.Fatalf("expected update available")
	}
	if info.LatestVersion != "v0.1.3" {
		t.Fatalf("unexpected latest version: %s", info.LatestVersion)
	}
}

func TestCheckLatestReleaseError(t *testing.T) {
	r := fakeRunner{err: errors.New("boom")}
	if _, err := checkLatestRelease(context.Background(), r); err == nil {
		t.Fatalf("expected error")
	}
}
