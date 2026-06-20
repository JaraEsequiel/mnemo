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
	"index.md":   true,
	"log.md":     true,
	"claude.md":  true,
	"schema.md":  true,
	"working.md": true, // L1 scratchpad, not a wiki page
}

// ResolveRoot returns the vault root. Precedence: the explicit flag, the
// MNEMO_VAULT environment variable, walking up from the cwd for a `.mnemo/`
// directory, the active-vault pointer written by `mnemo setup`, and finally the
// global default `~/.mnemo/vault`. The pointer lets hooks and ad-hoc commands
// find the configured vault from any directory without an exported env var.
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
	if p := readActiveVault(); p != "" {
		return p, nil
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

// ActiveVaultPointer is the path of the file recording the configured vault.
func ActiveVaultPointer() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".mnemo", "active-vault")
	}
	return filepath.Join(home, ".mnemo", "active-vault")
}

// WriteActiveVault records root as the active vault for hooks/CLI to resolve.
func WriteActiveVault(root string) error {
	p := ActiveVaultPointer()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(root+"\n"), 0o644)
}

func readActiveVault() string {
	data, err := os.ReadFile(ActiveVaultPointer())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
