package vault

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ignoredDirs are never walked when collecting pages.
var ignoredDirs = map[string]bool{
	".mnemo":       true,
	".obsidian":    true,
	".git":         true,
	"node_modules": true,
}

// ignoredFiles are markdown files that are catalogs/chronicles, not content.
// index.md is generated from pages; indexing it would duplicate content.
// ignoredFiles are not indexed as wiki content: catalogs, the chronicle, and
// the meta/hot-cache files (CLAUDE.md = L0, SCHEMA.md = the contract).
var ignoredFiles = map[string]bool{
	"index.md":  true,
	"log.md":    true,
	"claude.md": true,
	"schema.md": true,
}

// ResolveRoot returns the vault root. Precedence: the explicit flag, then the
// MNEMO_VAULT environment variable, then walking up from the cwd for a `.mnemo/`
// directory; if none is found, the global default `~/.mnemo/vault` is used so a
// zero-config install (e.g. the plugin's `mnemo mcp`) still has one brain.
func ResolveRoot(flag string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}
	if env := strings.TrimSpace(os.Getenv("MNEMO_VAULT")); env != "" {
		return filepath.Abs(env)
	}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for {
			if fi, err := os.Stat(filepath.Join(dir, ".mnemo")); err == nil && fi.IsDir() {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return DefaultVault(), nil
}

// DefaultVault is the global fallback vault location: ~/.mnemo/vault.
func DefaultVault() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".mnemo", "vault")
	}
	return filepath.Join(home, ".mnemo", "vault")
}

// DBPath returns the derived index path for a vault root.
func DBPath(root string) string {
	return filepath.Join(root, ".mnemo", "wiki.db")
}

// WalkPages returns absolute paths of all indexable markdown files under root.
func WalkPages(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable entries
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		if ignoredFiles[strings.ToLower(name)] {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out, err
}
