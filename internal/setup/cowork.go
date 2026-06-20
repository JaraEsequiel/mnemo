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

	// 4. Skills in Skills/ (referenced from the CLAUDE.md Skill-Registry).
	skills, err := installCoworkSkills(opts.PluginSrc, target)
	if err != nil {
		fmt.Fprintf(out, "! skills not installed: %v\n", err)
	} else {
		fmt.Fprintf(out, "✓ %d skills in Skills/ (registry in CLAUDE.md)\n", len(skills))
	}

	// 5. Hooks: scripts in .claude/mnemo-hooks/ + entries merged into settings.json.
	if err := installCoworkHooks(opts.PluginSrc, target); err != nil {
		fmt.Fprintf(out, "! hooks not installed: %v\n", err)
	} else {
		fmt.Fprintf(out, "✓ hooks in .claude/settings.json (Memory Protocol, tool loading, nudges)\n")
	}

	// 6. Root CLAUDE.md: load-L0 instruction + maintenance contract + Skill-Registry.
	if err := writeCoworkClaudeMd(target, skills); err == nil {
		fmt.Fprintf(out, "✓ CLAUDE.md (loads L0 from Memory/CLAUDE.md + Skill-Registry)\n")
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

type skillInfo struct{ Name, Desc, Path string }

// installCoworkSkills copies each plugin skill into <target>/Skills/<name>/SKILL.md
// — a visible registry referenced from the root CLAUDE.md. Returns the skills so
// the CLAUDE.md Skill-Registry can list them with their triggers.
func installCoworkSkills(pluginSrc, target string) ([]skillInfo, error) {
	srcDir := filepath.Join(pluginSrc, "skills")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}
	var skills []skillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.TrimPrefix(e.Name(), "mnemo-") // maintainer, start, ingest, query, lint
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		rel := "Skills/" + name + "/SKILL.md"
		dst := filepath.Join(target, "Skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return skills, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return skills, err
		}
		skills = append(skills, skillInfo{Name: name, Desc: skillDescription(string(data)), Path: rel})
	}
	return skills, nil
}

// skillDescription extracts the frontmatter description from a SKILL.md.
func skillDescription(md string) string {
	for _, l := range strings.Split(md, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "description:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "description:")), `"`)
		}
	}
	return ""
}

func tableCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	if len(s) > 130 {
		s = s[:127] + "…"
	}
	return s
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

func writeCoworkClaudeMd(target string, skills []skillInfo) error {
	path := filepath.Join(target, "CLAUDE.md")
	const begin = "<!-- mnemo:begin -->"
	const end = "<!-- mnemo:end -->"

	var sb strings.Builder
	sb.WriteString(begin + "\n# mnemo — knowledge memory\n\n")

	sb.WriteString("## Load your memory first\n")
	sb.WriteString("Your **L0 hot cache** is `Memory/CLAUDE.md` (top entities, active projects, current " +
		"context). It is imported below — keep it in context every session. The full knowledge vault lives " +
		"in `Memory/` and the markdown is the source of truth. Recall with the `wiki_search` / `wiki_list` / " +
		"`wiki_get` MCP tools before answering anything that touches past work.\n\n@Memory/CLAUDE.md\n\n")

	sb.WriteString("## Memory maintenance (always active)\n")
	sb.WriteString("**Save proactively — do not wait to be asked.** After a decision, a bug fixed, a " +
		"non-obvious discovery, a convention, a new idea/proposal, or a project status change: create or " +
		"update a page in `Memory/<type>/` with frontmatter (slug/title/type/tags/description) and a " +
		"What/Why/Where/Learned body. Tag the context (#work / #personal). After a decision/architecture " +
		"page, run `wiki_candidates` and record real relationships with `wiki_relate` (always with a " +
		"reason). Never hand-edit `Memory/.mnemo/wiki.db`. Full rules: `Skills/maintainer/SKILL.md`.\n\n")

	sb.WriteString("## Skill-Registry\n")
	sb.WriteString("These skills live in `Skills/`. Load one by **reading its file** when its trigger " +
		"matches:\n\n| Skill | When to use | File |\n|-------|-------------|------|\n")
	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("| %s | %s | `%s` |\n", s.Name, tableCell(s.Desc), s.Path))
	}
	sb.WriteString(end)
	block := sb.String()

	existing, _ := os.ReadFile(path)
	content := string(existing)
	// Replace an existing mnemo block (idempotent) or append below the user's content.
	if i := strings.Index(content, begin); i >= 0 {
		if j := strings.Index(content, end); j >= i {
			content = content[:i] + block + content[j+len(end):]
			return os.WriteFile(path, []byte(content), 0o644)
		}
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
