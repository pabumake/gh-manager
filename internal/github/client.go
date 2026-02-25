package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gh-manager/internal/app"
	"gh-manager/internal/planfile"
)

type Client struct {
	runner app.CommandRunner
}

func NewClient(r app.CommandRunner) Client {
	return Client{runner: r}
}

type ownerResponse struct {
	Login string `json:"login"`
}

type repoResponse struct {
	Name        string `json:"name"`
	NameWithOwn string `json:"nameWithOwner"`
	Description string `json:"description"`
	UpdatedAt   string `json:"updatedAt"`
	IsPrivate   bool   `json:"isPrivate"`
	IsFork      bool   `json:"isFork"`
	IsArchived  bool   `json:"isArchived"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (c Client) CurrentUser(ctx context.Context) (string, error) {
	out, err := c.runner.Run(ctx, "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c Client) ListUserRepos(ctx context.Context, owner string) ([]planfile.RepoRecord, error) {
	if owner == "" {
		u, err := c.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		owner = u
	}
	out, err := c.runner.Run(
		ctx,
		"gh", "repo", "list", owner,
		"--limit", "1000",
		"--json", "name,nameWithOwner,description,updatedAt,isPrivate,isFork,isArchived,owner",
	)
	if err != nil {
		return nil, err
	}
	var raw []repoResponse
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse repo list: %w", err)
	}
	repos := make([]planfile.RepoRecord, 0, len(raw))
	for _, r := range raw {
		updated := r.UpdatedAt
		if _, err := time.Parse(time.RFC3339, updated); err != nil {
			updated = ""
		}
		repos = append(repos, planfile.RepoRecord{
			Owner:       r.Owner.Login,
			Name:        r.Name,
			FullName:    r.NameWithOwn,
			Description: r.Description,
			IsPrivate:   r.IsPrivate,
			IsFork:      r.IsFork,
			IsArchived:  r.IsArchived,
			UpdatedAt:   updated,
		})
	}
	return repos, nil
}

func (c Client) DeleteRepo(ctx context.Context, fullName string) error {
	_, err := c.runner.Run(ctx, "gh", "repo", "delete", fullName, "--yes")
	return err
}

func (c Client) EnsureRepo(ctx context.Context, fullName, visibility string) error {
	if _, err := c.runner.Run(ctx, "gh", "repo", "view", fullName, "--json", "name", "--jq", ".name"); err == nil {
		return nil
	}
	vis := "--private"
	if visibility == "public" {
		vis = "--public"
	}
	_, err := c.runner.Run(ctx, "gh", "repo", "create", fullName, vis, "--confirm")
	return err
}
