// Package setup performs the mnemo install actions: scaffold a vault, build its
// index, write the Obsidian graph config, install the Claude Code skills, and
// register the MCP server. It is driven either by the TUI wizard (tui.go) or by
// flags (`mnemo setup --yes`), so the same logic runs interactively or headless.
package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JaraEsequiel/mnemo/internal/ftsindex"
	"github.com/JaraEsequiel/mnemo/internal/graph"
	"github.com/JaraEsequiel/mnemo/internal/vault"
)

// Options are the resolved install choices.
type Options struct {
	Vault       string // vault root (created if missing)
	Scope       string // claude mcp scope: user | local | project
	PluginSrc   string // path to the repo's plugin/ dir (source of skills)
	SkillsDest  string // where skills are installed (default ~/.claude/skills/mnemo)
	WriteGraph  bool   // write Obsidian graph.json
	RegisterMCP bool   // run `claude mcp add`
}

// DefaultSkillsDest returns ~/.claude/skills/mnemo.
func DefaultSkillsDest() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "skills", "mnemo")
}

// Run executes the install actions, logging progress to out.
func Run(opts Options, out io.Writer) error {
	if opts.Vault == "" {
		return fmt.Errorf("setup: vault path is required")
	}
	abs, err := filepath.Abs(opts.Vault)
	if err != nil {
		return err
	}
	opts.Vault = abs

	// 1. Scaffold the vault (idempotent — never clobbers existing files).
	if err := ScaffoldVault(opts.Vault); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}
	fmt.Fprintf(out, "✓ vault ready at %s\n", opts.Vault)

	// 2. Build the derived index + folder catalogs.
	idx, err := ftsindex.Open(vault.DBPath(opts.Vault))
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	if _, err := ftsindex.Reindex(idx, opts.Vault); err != nil {
		idx.Close()
		return fmt.Errorf("index: %w", err)
	}
	_, _ = vault.GenerateIndexes(opts.Vault)
	idx.Close()
	fmt.Fprintf(out, "✓ index built (.mnemo/wiki.db)\n")

	// 3. Obsidian graph config (preserve existing).
	if opts.WriteGraph {
		if err := graph.Write(opts.Vault, graph.Preserve); err != nil {
			fmt.Fprintf(out, "! graph config skipped: %v\n", err)
		} else {
			fmt.Fprintf(out, "✓ Obsidian graph config written\n")
		}
	}

	// 4. Install skills.
	if opts.PluginSrc != "" {
		dest := opts.SkillsDest
		if dest == "" {
			dest = DefaultSkillsDest()
		}
		if err := installSkills(opts.PluginSrc, dest); err != nil {
			fmt.Fprintf(out, "! skills not installed: %v\n", err)
		} else {
			fmt.Fprintf(out, "✓ skills installed to %s (/mnemo:start, /mnemo:ingest, /mnemo:query, /mnemo:lint)\n", dest)
		}
	}

	// 5. Register the MCP server (passing the vault via -e, so no export needed).
	if opts.RegisterMCP {
		if err := registerMCP(opts.Vault, opts.Scope, out); err != nil {
			fmt.Fprintf(out, "! MCP not registered: %v\n", err)
			fmt.Fprintf(out, "  run manually: claude mcp add mnemo --scope %s -e MNEMO_VAULT=%q -- mnemo mcp\n", opts.Scope, opts.Vault)
		} else {
			fmt.Fprintf(out, "✓ MCP server registered (scope: %s)\n", opts.Scope)
		}
	}

	fmt.Fprintf(out, "\nDone. Next: open Claude Code in the vault so its CLAUDE.md (L0) loads:\n  cd %q && claude\n", opts.Vault)
	return nil
}

// ScaffoldVault creates the flat-by-type vault structure. Idempotent: existing
// files are never overwritten.
func ScaffoldVault(root string) error {
	dirs := []string{
		".mnemo",
		"entities", "concepts", "projects", "ideas", "decisions", "sources", "raw",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return err
		}
	}
	files := map[string]string{
		".gitignore": ".mnemo/wiki.db\n.mnemo/wiki.db-*\n",
		"CLAUDE.md":  "# Hot cache (L0)\n\nTop entities, active projects, and context live here.\nmnemo keeps this current (auto promote/demote).\n",
		"log.md":     "# Log\n\n<!-- ## [YYYY-MM-DD] ingest|query|lint | title -->\n",
		"working.md": "# Working memory (L1)\n\nActive threads and goals right now.\n",
		"index.md":   "# Vault index\n\n- [[entities]] · [[concepts]] · [[projects]] · [[ideas]] · [[decisions]] · [[sources]]\n",
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// installSkills copies the plugin's skills + manifest into dest.
func installSkills(pluginSrc, dest string) error {
	if fi, err := os.Stat(pluginSrc); err != nil || !fi.IsDir() {
		return fmt.Errorf("plugin source %q not found", pluginSrc)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, sub := range []string{".claude-plugin", "skills"} {
		src := filepath.Join(pluginSrc, sub)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyTree(src, filepath.Join(dest, sub)); err != nil {
			return err
		}
	}
	return nil
}

func registerMCP(vaultPath, scope string, out io.Writer) error {
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH")
	}
	// Replace any prior registration so re-running setup is idempotent.
	_ = exec.Command("claude", "mcp", "remove", "mnemo", "--scope", scope).Run()
	cmd := exec.Command("claude", "mcp", "add", "mnemo",
		"--scope", scope, "-e", "MNEMO_VAULT="+vaultPath, "--", "mnemo", "mcp")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	return nil
}

// copyTree recursively copies src to dst.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
