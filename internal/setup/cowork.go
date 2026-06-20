package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JaraEsequiel/mnemo/internal/ftsindex"
	"github.com/JaraEsequiel/mnemo/internal/vault"
)

// Cowork-relative paths inside the synced/working folder. ${CLAUDE_PROJECT_DIR}
// resolves to that folder in a cloud session; the `:-.` default keeps it working
// locally too.
const (
	coworkVaultDir = "Memory"
	// Keep the binary OUT of a `.mnemo/` dir: that name is the vault marker that
	// ResolveRoot walks for, so a `.mnemo/` at the folder root would shadow the
	// real vault at Memory/.mnemo.
	coworkBinRel     = ".mnemo-bin/mnemo"
	coworkHooksDir   = ".claude/mnemo-hooks"
	coworkBinExpr    = "${CLAUDE_PROJECT_DIR:-.}/.mnemo-bin/mnemo"
	coworkVaultExpr  = "${CLAUDE_PROJECT_DIR:-.}/Memory"
	coworkHooksExpr  = "${CLAUDE_PROJECT_DIR:-.}/.claude/mnemo-hooks"
	coworkHookMarker = "mnemo-hooks" // identifies our entries for idempotent merges
)

// RunCowork writes project-scoped mnemo config into a target folder so it works
// in a Claude Code cloud/sandbox session, where only the working folder is
// present (not ~/). Nothing is written to the home directory.
func RunCowork(opts Options, out io.Writer) error {
	target := opts.Target
	if target == "" {
		target = "."
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create target folder: %w", err)
	}
	vaultDir := filepath.Join(target, coworkVaultDir)

	// 1. Vault + index inside the folder.
	if err := ScaffoldVault(vaultDir); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}
	idx, err := ftsindex.Open(vault.DBPath(vaultDir))
	if err == nil {
		_, _ = ftsindex.Reindex(idx, vaultDir)
		_, _ = vault.GenerateIndexes(vaultDir)
		idx.Close()
	}
	fmt.Fprintf(out, "✓ vault at %s/%s\n", filepath.Base(target), coworkVaultDir)

	// 2. Binary: copy the running binary into the folder so it syncs to the sandbox.
	if err := copyRunningBinary(filepath.Join(target, coworkBinRel)); err != nil {
		fmt.Fprintf(out, "! binary not copied: %v (build it into %s yourself)\n", err, coworkBinRel)
	} else {
		fmt.Fprintf(out, "✓ binary copied to %s\n", coworkBinRel)
	}

	// 3. Project MCP server (merged into any existing .mcp.json).
	if err := writeCoworkMCP(target); err != nil {
		return fmt.Errorf(".mcp.json: %w", err)
	}
	fmt.Fprintf(out, "✓ .mcp.json (mnemo, vault relative to the folder)\n")

	// 4. Skills as project skills: .claude/skills/mnemo-*
	n, err := installCoworkSkills(opts.PluginSrc, target)
	if err != nil {
		fmt.Fprintf(out, "! skills not installed: %v\n", err)
	} else {
		fmt.Fprintf(out, "✓ %d skills in .claude/skills/ (/mnemo-maintainer, /mnemo-start, …)\n", n)
	}

	// 5. Hooks: scripts in .claude/mnemo-hooks/ + entries merged into settings.json.
	if err := installCoworkHooks(opts.PluginSrc, target); err != nil {
		fmt.Fprintf(out, "! hooks not installed: %v\n", err)
	} else {
		fmt.Fprintf(out, "✓ hooks in .claude/settings.json (Memory Protocol, tool loading, nudges)\n")
	}

	// 6. Root CLAUDE.md mnemo section (loads the L0 hot cache + protocol pointer).
	if err := writeCoworkClaudeMd(target); err == nil {
		fmt.Fprintf(out, "✓ CLAUDE.md mnemo section (imports Memory/CLAUDE.md)\n")
	}

	// 7. .gitignore the binary + derived index in case the folder is a git repo.
	_ = ensureGitignore(target, ".mnemo-bin/", "Memory/.mnemo/wiki.db", "Memory/.mnemo/wiki.db-*")

	fmt.Fprintf(out, "\nDone. This folder is now mnemo-ready for Cowork.\n")
	fmt.Fprintf(out, "Point Cowork at %q and the sandbox loads everything from the folder.\n", target)
	fmt.Fprintf(out, "Note: the copied binary is for linux/amd64 — rebuild it on a different host OS.\n")
	return nil
}

func copyRunningBinary(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func writeCoworkMCP(target string) error {
	path := filepath.Join(target, ".mcp.json")
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("existing .mcp.json is not valid JSON: %w", err)
		}
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["mnemo"] = map[string]any{
		"command": coworkBinExpr,
		"args":    []string{"mcp"},
		"env":     map[string]string{"MNEMO_VAULT": coworkVaultExpr},
	}
	root["mcpServers"] = servers
	return writeJSON(path, root)
}

// installCoworkSkills copies each plugin skill to .claude/skills/mnemo-<name>,
// rewriting its frontmatter name to match (so it loads as /mnemo-<name>).
func installCoworkSkills(pluginSrc, target string) (int, error) {
	srcDir := filepath.Join(pluginSrc, "skills")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		newName := name
		if !strings.HasPrefix(name, "mnemo") {
			newName = "mnemo-" + name
		}
		data, err := os.ReadFile(filepath.Join(srcDir, name, "SKILL.md"))
		if err != nil {
			continue
		}
		patched := rewriteSkillName(string(data), newName)
		dst := filepath.Join(target, ".claude", "skills", newName, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return count, err
		}
		if err := os.WriteFile(dst, []byte(patched), 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// rewriteSkillName replaces the `name:` frontmatter field with newName.
func rewriteSkillName(md, newName string) string {
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "name:") {
			lines[i] = "name: " + newName
			break
		}
	}
	return strings.Join(lines, "\n")
}

func installCoworkHooks(pluginSrc, target string) error {
	// Copy hook scripts.
	srcHooks := filepath.Join(pluginSrc, "hooks")
	for _, s := range []string{"session-start.sh", "post-compaction.sh", "user-prompt-submit.sh"} {
		data, err := os.ReadFile(filepath.Join(srcHooks, s))
		if err != nil {
			return err
		}
		dst := filepath.Join(target, coworkHooksDir, s)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			return err
		}
	}

	// Merge hook entries into .claude/settings.json. MNEMO_BIN tells the scripts
	// where the binary is (not on PATH in the sandbox); MNEMO_VAULT pins the vault
	// so resolution never depends on cwd.
	envPrefix := fmt.Sprintf("MNEMO_BIN=%q MNEMO_VAULT=%q ", coworkBinExpr, coworkVaultExpr)
	cmd := func(script string) string {
		return envPrefix + fmt.Sprintf("%q", coworkHooksExpr+"/"+script)
	}
	mnemoHooks := map[string]any{
		"SessionStart": []any{
			hookEntry("startup|clear", cmd("session-start.sh")),
			hookEntry("compact", cmd("post-compaction.sh")),
		},
		"UserPromptSubmit": []any{
			hookEntryNoMatcher(cmd("user-prompt-submit.sh")),
		},
	}

	path := filepath.Join(target, ".claude", "settings.json")
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("existing settings.json is not valid JSON: %w", err)
		}
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for event, fresh := range mnemoHooks {
		existing, _ := hooks[event].([]any)
		existing = dropMnemoEntries(existing)
		existing = append(existing, fresh.([]any)...)
		hooks[event] = existing
	}
	root["hooks"] = hooks
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSON(path, root)
}

func hookEntry(matcher, command string) any {
	return map[string]any{
		"matcher": matcher,
		"hooks":   []any{map[string]any{"type": "command", "command": command}},
	}
}

func hookEntryNoMatcher(command string) any {
	return map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": command}},
	}
}

// dropMnemoEntries removes previously-written mnemo hook entries (idempotency).
func dropMnemoEntries(arr []any) []any {
	var out []any
	for _, e := range arr {
		if b, err := json.Marshal(e); err == nil && strings.Contains(string(b), coworkHookMarker) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func writeCoworkClaudeMd(target string) error {
	path := filepath.Join(target, "CLAUDE.md")
	const begin = "<!-- mnemo:begin -->"
	const end = "<!-- mnemo:end -->"
	block := begin + "\n## mnemo — knowledge memory\n\n" +
		"You have **mnemo** (MCP server `mnemo`, vault in `Memory/`). Recall with `wiki_search`/" +
		"`wiki_list` before answering past-work questions, and **save proactively** — after any " +
		"decision, bugfix, discovery, idea, or project change, create/update a page (see the " +
		"`mnemo-maintainer` skill). The markdown is the source of truth.\n\n@Memory/CLAUDE.md\n" + end

	existing, _ := os.ReadFile(path)
	content := string(existing)
	if strings.Contains(content, begin) {
		return nil // already present
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	content += block + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// ensureGitignore appends the given patterns to <target>/.gitignore if absent.
func ensureGitignore(target string, patterns ...string) error {
	path := filepath.Join(target, ".gitignore")
	existing, _ := os.ReadFile(path)
	content := string(existing)
	var add []string
	for _, p := range patterns {
		if !strings.Contains(content, p) {
			add = append(add, p)
		}
	}
	if len(add) == 0 {
		return nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "# mnemo (derived/binary — do not commit)\n" + strings.Join(add, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
