package restore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gh-manager/internal/app"
)

type Service struct {
	runner app.CommandRunner
}

func NewService(r app.CommandRunner) Service {
	return Service{runner: r}
}

type Request struct {
	ArchiveRoot      string
	RepoFullName     string
	SourceKind       string
	SourcePath       string
	TargetOwner      string
	TargetName       string
	TargetVisibility string
}

type Result struct {
	TargetFullName string
	WorkDir        string
	SourceKind     string
	SourcePath     string
}

type TargetExistsError struct {
	TargetFullName string
	Suggested      string
}

func (e TargetExistsError) Error() string {
	return fmt.Sprintf("target repository already exists: %s", e.TargetFullName)
}

func (e TargetExistsError) SuggestedName() string {
	return e.Suggested
}

func (s Service) Restore(ctx context.Context, req Request) (Result, error) {
	if s.runner == nil {
		return Result{}, fmt.Errorf("restore runner is nil")
	}
	if strings.TrimSpace(req.SourcePath) == "" {
		return Result{}, fmt.Errorf("source path is required")
	}
	if strings.TrimSpace(req.TargetOwner) == "" || strings.TrimSpace(req.TargetName) == "" {
		return Result{}, fmt.Errorf("target owner and name are required")
	}
	if req.TargetVisibility == "" {
		req.TargetVisibility = "private"
	}
	if req.TargetVisibility != "private" && req.TargetVisibility != "public" {
		return Result{}, fmt.Errorf("unsupported visibility: %s", req.TargetVisibility)
	}
	targetFullName := req.TargetOwner + "/" + req.TargetName

	if err := validateSource(req.SourceKind, req.SourcePath); err != nil {
		return Result{}, err
	}
	if exists, err := repoExists(ctx, s.runner, targetFullName); err != nil {
		return Result{}, err
	} else if exists {
		return Result{}, TargetExistsError{TargetFullName: targetFullName, Suggested: req.TargetName + "-ghm"}
	}

	workdir, err := os.MkdirTemp("", "gh-manager-restore-*")
	if err != nil {
		return Result{}, err
	}

	if _, err := s.runner.Run(ctx, "git", "clone", req.SourcePath, workdir); err != nil {
		return Result{}, err
	}

	visFlag := "--private"
	if req.TargetVisibility == "public" {
		visFlag = "--public"
	}
	if _, err := s.runner.Run(ctx, "gh", "repo", "create", targetFullName, visFlag, "--confirm"); err != nil {
		return Result{}, err
	}

	remote := "git@github.com:" + targetFullName + ".git"
	if _, err := s.runner.Run(ctx, "git", "-C", workdir, "remote", "set-url", "origin", remote); err != nil {
		if _, addErr := s.runner.Run(ctx, "git", "-C", workdir, "remote", "add", "origin", remote); addErr != nil {
			return Result{}, err
		}
	}
	if _, err := s.runner.Run(ctx, "git", "-C", workdir, "push", "--all", "origin"); err != nil {
		return Result{}, err
	}
	if _, err := s.runner.Run(ctx, "git", "-C", workdir, "push", "--tags", "origin"); err != nil {
		return Result{}, err
	}

	return Result{
		TargetFullName: targetFullName,
		WorkDir:        workdir,
		SourceKind:     req.SourceKind,
		SourcePath:     req.SourcePath,
	}, nil
}

func validateSource(kind, path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	switch kind {
	case "bundle":
		if st.IsDir() {
			return fmt.Errorf("bundle source must be a file: %s", path)
		}
		if filepath.Ext(path) != ".bundle" {
			return fmt.Errorf("bundle source must end with .bundle: %s", path)
		}
	case "snapshot":
		if !st.IsDir() {
			return fmt.Errorf("snapshot source must be a directory: %s", path)
		}
	default:
		return fmt.Errorf("unsupported source kind: %s", kind)
	}
	return nil
}

func repoExists(ctx context.Context, runner app.CommandRunner, targetFullName string) (bool, error) {
	_, err := runner.Run(ctx, "gh", "repo", "view", targetFullName, "--json", "name", "--jq", ".name")
	if err == nil {
		return true, nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "could not resolve") || strings.Contains(msg, "http 404") {
		return false, nil
	}
	return false, err
}
