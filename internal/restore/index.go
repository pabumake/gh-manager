package restore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gh-manager/internal/manifest"
)

type ArchiveEntry struct {
	FullName     string
	BundlePath   string
	SnapshotPath string
	UpdatedAt    string
}

type Source struct {
	Kind string
	Path string
}

func LoadIndex(root string) ([]ArchiveEntry, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("archive root is required")
	}
	entries := map[string]*ArchiveEntry{}

	if err := loadFromManifest(root, entries); err != nil {
		return nil, err
	}
	if err := scanBundles(root, entries); err != nil {
		return nil, err
	}
	if err := scanSnapshots(root, entries); err != nil {
		return nil, err
	}

	out := make([]ArchiveEntry, 0, len(entries))
	for _, e := range entries {
		if e.FullName == "" {
			continue
		}
		if e.BundlePath == "" && e.SnapshotPath == "" {
			continue
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName < out[j].FullName })
	return out, nil
}

func PreferredSource(e ArchiveEntry) (Source, bool) {
	if e.BundlePath != "" {
		if fi, err := os.Stat(e.BundlePath); err == nil && !fi.IsDir() {
			return Source{Kind: "bundle", Path: e.BundlePath}, true
		}
	}
	if e.SnapshotPath != "" {
		if fi, err := os.Stat(e.SnapshotPath); err == nil && fi.IsDir() {
			return Source{Kind: "snapshot", Path: e.SnapshotPath}, true
		}
	}
	return Source{}, false
}

func IsArchiveRoot(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "manifest.json")); err == nil {
		return true
	}
	if st, err := os.Stat(filepath.Join(path, "bundles")); err == nil && st.IsDir() {
		return true
	}
	if st, err := os.Stat(filepath.Join(path, "snapshots")); err == nil && st.IsDir() {
		return true
	}
	return false
}

func loadFromManifest(root string, out map[string]*ArchiveEntry) error {
	path := filepath.Join(root, "manifest.json")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	m, err := manifest.Read(path)
	if err != nil {
		return err
	}
	for _, re := range m.RepoExecutions {
		if re.FullName == "" {
			continue
		}
		e := ensureEntry(out, re.FullName)
		e.UpdatedAt = firstNonEmpty(e.UpdatedAt, re.LastAttemptAt)
		if re.BundlePath != "" {
			e.BundlePath = resolvePath(root, re.BundlePath)
		}
		if re.BrowsablePath != "" {
			e.SnapshotPath = resolvePath(root, re.BrowsablePath)
		}
	}
	return nil
}

func scanBundles(root string, out map[string]*ArchiveEntry) error {
	dir := filepath.Join(root, "bundles")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, ent := range ents {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".bundle") {
			continue
		}
		fullName, ok := bundleNameToFullName(ent.Name())
		if !ok {
			continue
		}
		e := ensureEntry(out, fullName)
		e.BundlePath = filepath.Join(dir, ent.Name())
	}
	return nil
}

func scanSnapshots(root string, out map[string]*ArchiveEntry) error {
	dir := filepath.Join(root, "snapshots")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		fullName, ok := snapshotNameToFullName(ent.Name())
		if !ok {
			continue
		}
		e := ensureEntry(out, fullName)
		e.SnapshotPath = filepath.Join(dir, ent.Name())
	}
	return nil
}

func ensureEntry(out map[string]*ArchiveEntry, fullName string) *ArchiveEntry {
	e, ok := out[fullName]
	if ok {
		return e
	}
	e = &ArchiveEntry{FullName: fullName}
	out[fullName] = e
	return e
}

func bundleNameToFullName(filename string) (string, bool) {
	base := strings.TrimSuffix(filename, ".bundle")
	parts := strings.SplitN(base, "__", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func snapshotNameToFullName(dirname string) (string, bool) {
	parts := strings.SplitN(dirname, "__", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func resolvePath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
