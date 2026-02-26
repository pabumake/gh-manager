package theme

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func FetchIndex(ctx context.Context, indexURL string) (ThemeIndex, error) {
	b, err := fetchURL(ctx, indexURL)
	if err != nil {
		return ThemeIndex{}, err
	}
	var idx ThemeIndex
	if err := json.Unmarshal(b, &idx); err != nil {
		return ThemeIndex{}, err
	}
	if idx.Version == 0 {
		idx.Version = 1
	}
	return idx, nil
}

func FetchThemeByID(ctx context.Context, indexURL, id string) (ThemeFile, error) {
	idx, err := FetchIndex(ctx, indexURL)
	if err != nil {
		return ThemeFile{}, err
	}
	for _, entry := range idx.Themes {
		if entry.ID != id {
			continue
		}
		resolvedURL := resolveThemeURL(indexURL, entry.URL)
		b, err := fetchURL(ctx, resolvedURL)
		if err != nil {
			return ThemeFile{}, err
		}
		t, err := ParseThemeFile(b)
		if err != nil {
			return ThemeFile{}, err
		}
		return t, nil
	}
	return ThemeFile{}, fmt.Errorf("theme not found in index: %s", id)
}

func fetchURL(ctx context.Context, url string) ([]byte, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "file://") {
		return os.ReadFile(url)
	}
	if strings.HasPrefix(url, "file://") {
		u, err := netURL(url)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(u.Path)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("fetch %s failed: %s", url, res.Status)
	}
	return io.ReadAll(res.Body)
}

func resolveThemeURL(indexURL, entryURL string) string {
	if strings.HasPrefix(entryURL, "http://") || strings.HasPrefix(entryURL, "https://") || strings.HasPrefix(entryURL, "file://") {
		return entryURL
	}
	if strings.HasPrefix(indexURL, "http://") || strings.HasPrefix(indexURL, "https://") {
		base, err := netURL(indexURL)
		if err != nil {
			return entryURL
		}
		ref, err := url.Parse(entryURL)
		if err != nil {
			return entryURL
		}
		return base.ResolveReference(ref).String()
	}
	if strings.HasPrefix(indexURL, "file://") {
		u, err := netURL(indexURL)
		if err != nil {
			return entryURL
		}
		return filepath.Join(filepath.Dir(u.Path), entryURL)
	}
	return filepath.Join(filepath.Dir(indexURL), entryURL)
}

func netURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
