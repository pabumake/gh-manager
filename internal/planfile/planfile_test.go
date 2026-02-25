package planfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanSignValidateAndTamperDetection(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	plan := New("alice", "github.com", "test", []RepoRecord{{
		Owner: "alice", Name: "r1", FullName: "alice/r1", UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}}, time.Now())
	if err := plan.Sign(secret); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := plan.Validate(secret); err != nil {
		t.Fatalf("validate: %v", err)
	}

	plan.Repos[0].Name = "r2"
	if err := plan.Validate(secret); err == nil {
		t.Fatal("expected tamper validation error")
	}
}

func TestEnsureSecret(t *testing.T) {
	d := t.TempDir()
	secret, err := EnsureSecret(d)
	if err != nil {
		t.Fatalf("ensure secret: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32-byte secret, got %d", len(secret))
	}
	secret2, err := EnsureSecret(d)
	if err != nil {
		t.Fatalf("ensure secret 2: %v", err)
	}
	if string(secret) != string(secret2) {
		t.Fatal("expected stable secret")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	p := New("alice", "github.com", "test", nil, time.Now())
	p.Fingerprint = "f"
	p.Signature = "s"
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := Write(path, p); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	inJSON, _ := json.Marshal(p)
	outJSON, _ := json.Marshal(out)
	if string(inJSON) != string(outJSON) {
		t.Fatalf("roundtrip mismatch")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file missing: %v", err)
	}
}
