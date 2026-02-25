package planfile

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RepoRecord struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"isPrivate"`
	IsFork      bool   `json:"isFork"`
	IsArchived  bool   `json:"isArchived"`
	UpdatedAt   string `json:"updatedAt"`
}

type DeletionPlanV1 struct {
	SchemaVersion string       `json:"schemaVersion"`
	CreatedAt     string       `json:"createdAt"`
	Actor         string       `json:"actor"`
	Host          string       `json:"host"`
	Repos         []RepoRecord `json:"repos"`
	Count         int          `json:"count"`
	Fingerprint   string       `json:"fingerprint"`
	Signature     string       `json:"signature"`
	ToolVersion   string       `json:"toolVersion"`
}

type canonicalPlan struct {
	SchemaVersion string       `json:"schemaVersion"`
	CreatedAt     string       `json:"createdAt"`
	Actor         string       `json:"actor"`
	Host          string       `json:"host"`
	Repos         []RepoRecord `json:"repos"`
	Count         int          `json:"count"`
	ToolVersion   string       `json:"toolVersion"`
}

func New(actor, host, toolVersion string, repos []RepoRecord, now time.Time) DeletionPlanV1 {
	sorted := append([]RepoRecord(nil), repos...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FullName < sorted[j].FullName
	})
	return DeletionPlanV1{
		SchemaVersion: "v1",
		CreatedAt:     now.UTC().Format(time.RFC3339),
		Actor:         actor,
		Host:          host,
		Repos:         sorted,
		Count:         len(sorted),
		ToolVersion:   toolVersion,
	}
}

func (p *DeletionPlanV1) Sign(secret []byte) error {
	if len(secret) == 0 {
		return errors.New("empty signing secret")
	}
	fp, err := p.ComputeFingerprint()
	if err != nil {
		return err
	}
	p.Fingerprint = fp
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(fp))
	p.Signature = hex.EncodeToString(h.Sum(nil))
	return nil
}

func (p DeletionPlanV1) Validate(secret []byte) error {
	if p.SchemaVersion != "v1" {
		return fmt.Errorf("unsupported schemaVersion: %s", p.SchemaVersion)
	}
	if p.Count != len(p.Repos) {
		return fmt.Errorf("count mismatch: count=%d repos=%d", p.Count, len(p.Repos))
	}
	if _, err := time.Parse(time.RFC3339, p.CreatedAt); err != nil {
		return fmt.Errorf("invalid createdAt: %w", err)
	}
	fp, err := p.ComputeFingerprint()
	if err != nil {
		return err
	}
	if p.Fingerprint != fp {
		return errors.New("fingerprint mismatch")
	}
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(fp))
	expected := hex.EncodeToString(h.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(p.Signature))) {
		return errors.New("invalid signature")
	}
	return nil
}

func (p DeletionPlanV1) ComputeFingerprint() (string, error) {
	canon := canonicalPlan{
		SchemaVersion: p.SchemaVersion,
		CreatedAt:     p.CreatedAt,
		Actor:         p.Actor,
		Host:          p.Host,
		Repos:         append([]RepoRecord(nil), p.Repos...),
		Count:         p.Count,
		ToolVersion:   p.ToolVersion,
	}
	sort.Slice(canon.Repos, func(i, j int) bool {
		return canon.Repos[i].FullName < canon.Repos[j].FullName
	})
	b, err := json.Marshal(canon)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func Write(path string, p DeletionPlanV1) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func Read(path string) (DeletionPlanV1, error) {
	var p DeletionPlanV1
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	return p, nil
}

func EnsureSecret(configDir string) ([]byte, error) {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return nil, err
	}
	secretPath := filepath.Join(configDir, "secret.hex")
	if b, err := os.ReadFile(secretPath); err == nil {
		raw, decErr := hex.DecodeString(strings.TrimSpace(string(b)))
		if decErr != nil {
			return nil, fmt.Errorf("invalid secret format: %w", decErr)
		}
		if len(raw) < 32 {
			return nil, errors.New("secret too short")
		}
		return raw, nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	enc := []byte(hex.EncodeToString(raw) + "\n")
	if err := os.WriteFile(secretPath, enc, 0o600); err != nil {
		return nil, err
	}
	return raw, nil
}
