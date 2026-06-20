// Command mnemo is a markdown-first knowledge memory for AI agents.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JaraEsequiel/mnemo/internal/ftsindex"
	"github.com/JaraEsequiel/mnemo/internal/graph"
	"github.com/JaraEsequiel/mnemo/internal/llm"
	mnemomcp "github.com/JaraEsequiel/mnemo/internal/mcp"
	"github.com/JaraEsequiel/mnemo/internal/setup"
	"github.com/JaraEsequiel/mnemo/internal/vault"

	"github.com/mattn/go-isatty"
)

// version is injected at build time via -ldflags "-X main.version=…" (GoReleaser).
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "index":
		cmdIndex(os.Args[2:])
	case "indexes":
		cmdIndexes(os.Args[2:])
	case "search":
		cmdSearch(os.Args[2:])
	case "mcp":
		cmdMCP(os.Args[2:])
	case "stats":
		cmdStats(os.Args[2:])
	case "context":
		cmdContext(os.Args[2:])
	case "last-activity":
		cmdLastActivity(os.Args[2:])
	case "graph":
		cmdGraph(os.Args[2:])
	case "relate":
		cmdRelate(os.Args[2:])
	case "candidates":
		cmdCandidates(os.Args[2:])
	case "hot":
		cmdHot(os.Args[2:])
	case "lint":
		cmdLint(os.Args[2:])
	case "setup":
		cmdSetup(os.Args[2:])
	case "init":
		cmdInit(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("mnemo %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`mnemo — markdown-first knowledge memory

Usage:
  mnemo setup   [--vault DIR] [--scope S] [--yes] [--no-mcp] [--no-graph]
                                              Interactive installer (vault + index + skills + MCP)
  mnemo setup --cowork [--target DIR]         Write project-scoped config into a folder for Cowork
  mnemo init    [--vault DIR]                 Scaffold a vault (.mnemo/, folders, L0/log)
  mnemo index   [--vault DIR]                 Reindex into FTS5 + regenerate folder index.md
  mnemo indexes [--vault DIR]                 Regenerate folder index.md catalogs only
  mnemo search  <query> [--vault DIR] [--type T] [--limit N]
  mnemo mcp     [--vault DIR]                 Run the MCP server over stdio
  mnemo stats   [--vault DIR]                 Show vault statistics
  mnemo graph   [--vault DIR] [--mode preserve|force|skip]   Write Obsidian graph.json
  mnemo candidates <slug> [--vault DIR] [--limit N]          Find unrelated similar pages
  mnemo relate  <source> <target> <type> <reason...> [--vault DIR]   Record a relation
  mnemo hot suggest [--vault DIR] [--days N] [--top N]       Suggest CLAUDE.md promote/demote
  mnemo lint --semantic [--vault DIR] [--max N] [--min-confidence F]   Headless LLM contradiction judge

The markdown is the source of truth; the index (.mnemo/wiki.db) is derived.
`)
}

// flagValue extracts "--name value" / "--name=value" from args, returning the
// value and the remaining positional args.
func flagValue(args []string, name string) (string, []string) {
	var rest []string
	val := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--"+name && i+1 < len(args):
			val = args[i+1]
			i++
		case strings.HasPrefix(a, "--"+name+"="):
			val = strings.TrimPrefix(a, "--"+name+"=")
		default:
			rest = append(rest, a)
		}
	}
	return val, rest
}

func openVault(vaultFlag string) (string, *ftsindex.Index) {
	root, err := vault.ResolveRoot(vaultFlag)
	if err != nil {
		fatal("resolve vault: %v", err)
	}
	idx, err := ftsindex.Open(vault.DBPath(root))
	if err != nil {
		fatal("open index: %v", err)
	}
	return root, idx
}

func cmdIndex(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root, idx := openVault(vaultFlag)
	defer idx.Close()

	st, err := ftsindex.Reindex(idx, root)
	if err != nil {
		fatal("reindex: %v", err)
	}
	im, err := vault.GenerateIndexes(root)
	if err != nil {
		fatal("generate indexes: %v", err)
	}
	fmt.Printf("indexed %s\n", root)
	fmt.Printf("  fts: created=%d updated=%d deleted=%d unchanged=%d errors=%d\n",
		st.Created, st.Updated, st.Deleted, st.Unchanged, st.Errors)
	fmt.Printf("  index.md: folders=%d written=%d\n", im.Folders, im.Written)
}

func cmdIndexes(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root, err := vault.ResolveRoot(vaultFlag)
	if err != nil {
		fatal("resolve vault: %v", err)
	}
	im, err := vault.GenerateIndexes(root)
	if err != nil {
		fatal("generate indexes: %v", err)
	}
	fmt.Printf("index.md: folders=%d written=%d\n", im.Folders, im.Written)
}

func cmdMCP(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root, err := vault.ResolveRoot(vaultFlag)
	if err != nil {
		fatal("resolve vault: %v", err)
	}
	// Zero-config: ensure the (possibly default ~/.mnemo/vault) vault exists so
	// the plugin's bare `mnemo mcp` works without a prior `mnemo setup`.
	if err := setup.ScaffoldVault(root); err != nil {
		fatal("scaffold vault: %v", err)
	}
	idx, err := ftsindex.Open(vault.DBPath(root))
	if err != nil {
		fatal("open index: %v", err)
	}
	defer idx.Close()

	// Build the index once on startup so the first search is warm.
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	srv := mnemomcp.NewServer(root, idx)
	if err := mnemomcp.Serve(srv); err != nil {
		fatal("mcp: %v", err)
	}
}

func cmdCandidates(args []string) {
	vaultFlag, args := flagValue(args, "vault")
	limitStr, args := flagValue(args, "limit")
	if len(args) < 1 {
		fatal("candidates: a slug is required")
	}
	slug := args[0]
	limit := 5
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			limit = n
		}
	}
	root, idx := openVault(vaultFlag)
	defer idx.Close()
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	cands, err := idx.Candidates(slug, limit)
	if err != nil {
		fatal("candidates: %v", err)
	}
	if len(cands) == 0 {
		fmt.Printf("no unrelated candidates for %q\n", slug)
		return
	}
	fmt.Printf("candidates for %q:\n", slug)
	for _, c := range cands {
		fmt.Printf("  %-22s %s\n", c.Slug, c.Title)
	}
}

func cmdRelate(args []string) {
	vaultFlag, args := flagValue(args, "vault")
	if len(args) < 4 {
		fatal("relate: usage: mnemo relate <source> <target> <type> <reason...>")
	}
	source, target, relType := args[0], args[1], args[2]
	reason := strings.TrimSpace(strings.Join(args[3:], " "))
	if !vault.ValidRelationType(relType) {
		fatal("invalid type %q — use: supersedes, conflicts_with, related, refines, depends_on", relType)
	}
	root, idx := openVault(vaultFlag)
	defer idx.Close()
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	srcPath, err := idx.PathForSlug(source)
	if err != nil || srcPath == "" {
		fatal("no source page with slug %q", source)
	}
	if err := vault.AddRelation(filepath.Join(root, filepath.FromSlash(srcPath)),
		vault.Relation{Type: relType, Target: target, Reason: reason}); err != nil {
		fatal("relate: %v", err)
	}
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	fmt.Printf("recorded: %s %s [[%s]] — %s\n", source, relType, target, reason)
}

// boolFlag removes a "--name" token from args and reports whether it was present.
func boolFlag(args []string, name string) (bool, []string) {
	var rest []string
	found := false
	for _, a := range args {
		if a == name {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return found, rest
}

func readPageBody(root, rel string) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(data)
}

func pairKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

func cmdLint(args []string) {
	semantic, args := boolFlag(args, "--semantic")
	verbose, args := boolFlag(args, "--verbose")
	vaultFlag, args := flagValue(args, "vault")
	maxStr, args := flagValue(args, "max")
	minConfStr, args := flagValue(args, "min-confidence")
	timeoutStr, _ := flagValue(args, "timeout")

	if !semantic {
		fmt.Println("Non-semantic lint (orphans, stale pages, missing pages, hot-cache drift)")
		fmt.Println("is driven by the /lint skill. Use --semantic for the headless LLM judge.")
		return
	}

	runner, err := llm.NewRunner(os.Getenv("MNEMO_AGENT_CLI"))
	if err != nil {
		fatal("%v", err)
	}
	maxCalls, minConf, perCall := 40, 0.6, 60
	if n, err := strconv.Atoi(maxStr); err == nil && n > 0 {
		maxCalls = n
	}
	if f, err := strconv.ParseFloat(minConfStr, 64); err == nil && f > 0 {
		minConf = f
	}
	if n, err := strconv.Atoi(timeoutStr); err == nil && n > 0 {
		perCall = n
	}

	root, idx := openVault(vaultFlag)
	defer idx.Close()
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	signals, err := idx.PageSignals()
	if err != nil {
		fatal("lint: %v", err)
	}
	// Judge the most recently edited pages first — that is where new
	// contradictions appear (mirrors surfacing conflicts on save).
	sort.SliceStable(signals, func(i, j int) bool { return signals[i].MtimeUnix > signals[j].MtimeUnix })

	fmt.Printf("semantic lint of %s (judge: %s)\n\n", root, os.Getenv("MNEMO_AGENT_CLI"))
	seen := map[string]bool{}
	calls, written, skipped, errs := 0, 0, 0, 0

	for _, s := range signals {
		if calls >= maxCalls {
			break
		}
		srcPath, _ := idx.PathForSlug(s.Slug)
		if srcPath == "" {
			continue
		}
		cands, err := idx.Candidates(s.Slug, 3)
		if err != nil {
			continue
		}
		srcBody := readPageBody(root, srcPath)
		for _, c := range cands {
			if calls >= maxCalls {
				break
			}
			key := pairKey(s.Slug, c.Slug)
			if seen[key] {
				continue
			}
			seen[key] = true

			prompt := llm.BuildJudgePrompt(s.Title, srcBody, c.Title, readPageBody(root, c.RelPath))
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(perCall)*time.Second)
			v, jerr := runner.Judge(ctx, prompt)
			cancel()
			calls++
			if jerr != nil {
				errs++
				fmt.Fprintf(os.Stderr, "  judge %s~%s: %v\n", s.Slug, c.Slug, jerr)
				continue
			}
			if verbose {
				fmt.Printf("  · %s ~ %s → %s (%.2f) %s\n", s.Slug, c.Slug, v.Relation, v.Confidence, v.Reason)
			}
			// Only record strong edges; skip "related"/"not_conflict" and low confidence.
			if v.Relation == "not_conflict" || v.Relation == "related" || v.Confidence < minConf {
				skipped++
				continue
			}
			if err := vault.AddRelation(filepath.Join(root, filepath.FromSlash(srcPath)),
				vault.Relation{Type: v.Relation, Target: c.Slug, Reason: v.Reason}); err != nil {
				errs++
				continue
			}
			written++
			fmt.Printf("  %s %s [[%s]] — %s (%.2f)\n", s.Slug, v.Relation, c.Slug, v.Reason, v.Confidence)
		}
	}
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	fmt.Printf("\njudged=%d written=%d skipped=%d errors=%d\n", calls, written, skipped, errs)
}

func cmdHot(args []string) {
	// only subcommand for now: suggest
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		if args[0] != "suggest" {
			fatal("hot: unknown subcommand %q (try: suggest)", args[0])
		}
		args = args[1:]
	}
	vaultFlag, args := flagValue(args, "vault")
	daysStr, args := flagValue(args, "days")
	topStr, _ := flagValue(args, "top")
	days, top := 30, 15
	if n, err := strconv.Atoi(daysStr); err == nil && n > 0 {
		days = n
	}
	if n, err := strconv.Atoi(topStr); err == nil && n > 0 {
		top = n
	}

	root, idx := openVault(vaultFlag)
	defer idx.Close()
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	signals, err := idx.PageSignals()
	if err != nil {
		fatal("hot: %v", err)
	}
	inCache := vault.ReferencedSlugs(filepath.Join(root, "CLAUDE.md"))
	now := time.Now().Unix()
	ageDays := func(mtime int64) int64 {
		if mtime == 0 {
			return 9999
		}
		return (now - mtime) / 86400
	}

	var promote, demote []ftsindex.Signal
	for _, s := range signals {
		switch {
		case !inCache[s.Slug] && s.Inbound >= 1:
			if len(promote) < top {
				promote = append(promote, s)
			}
		case inCache[s.Slug] && s.Inbound == 0 && ageDays(s.MtimeUnix) > int64(days):
			demote = append(demote, s)
		}
	}

	fmt.Printf("hot-cache suggestions (CLAUDE.md) for %s\n", root)
	fmt.Printf("\nPROMOTE (referenced but not in CLAUDE.md):\n")
	if len(promote) == 0 {
		fmt.Println("  (none)")
	}
	for _, s := range promote {
		fmt.Printf("  + %-24s %d inbound · %s\n", s.Slug, s.Inbound, humanAge(ageDays(s.MtimeUnix)))
	}
	fmt.Printf("\nDEMOTE (in CLAUDE.md but stale & unreferenced > %dd):\n", days)
	if len(demote) == 0 {
		fmt.Println("  (none)")
	}
	for _, s := range demote {
		fmt.Printf("  - %-24s %d inbound · %s\n", s.Slug, s.Inbound, humanAge(ageDays(s.MtimeUnix)))
	}
	fmt.Printf("\nThe agent decides and edits CLAUDE.md; mnemo only suggests.\n")
}

func humanAge(days int64) string {
	if days >= 9999 {
		return "no date"
	}
	if days == 0 {
		return "today"
	}
	return fmt.Sprintf("%dd ago", days)
}

// cmdContext prints the L0 hot cache + the most recently touched pages, for the
// SessionStart hook to inject. Filesystem-based so it never needs the DB.
func cmdContext(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root, err := vault.ResolveRoot(vaultFlag)
	if err != nil {
		return
	}
	if data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md")); err == nil {
		fmt.Printf("### mnemo hot cache (%s)\n\n%s\n", root, strings.TrimSpace(string(data)))
	}
	recent := recentPages(root, 8)
	if len(recent) > 0 {
		fmt.Println("\n### Recent memory")
		for _, p := range recent {
			desc := p.Description
			if desc == "" {
				desc = p.Title
			}
			fmt.Printf("- %s/%s — %s\n", p.Folder, p.Slug, desc)
		}
		fmt.Println("\nSearch deeper with the wiki_search / wiki_list tools.")
	}
}

// cmdLastActivity prints the unix mtime of the most recently modified content
// page (0 if none), used by the save-nudge hook.
func cmdLastActivity(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root, err := vault.ResolveRoot(vaultFlag)
	if err != nil {
		fmt.Println(0)
		return
	}
	var newest int64
	files, _ := vault.WalkPages(root)
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil && fi.ModTime().Unix() > newest {
			newest = fi.ModTime().Unix()
		}
	}
	fmt.Println(newest)
}

// recentPages returns up to n content pages, newest first.
func recentPages(root string, n int) []*vault.Page {
	files, _ := vault.WalkPages(root)
	var pages []*vault.Page
	for _, f := range files {
		if p, err := vault.ParsePage(f, root); err == nil {
			pages = append(pages, p)
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].ModTime > pages[j].ModTime })
	if len(pages) > n {
		pages = pages[:n]
	}
	return pages
}

func cmdGraph(args []string) {
	vaultFlag, args := flagValue(args, "vault")
	modeStr, _ := flagValue(args, "mode")
	if modeStr == "" {
		modeStr = "preserve"
	}
	mode, err := graph.ParseMode(modeStr)
	if err != nil {
		fatal("%v", err)
	}
	root, err := vault.ResolveRoot(vaultFlag)
	if err != nil {
		fatal("resolve vault: %v", err)
	}
	if err := graph.Write(root, mode); err != nil {
		fatal("graph: %v", err)
	}
	fmt.Printf("graph config (%s) at %s\n", mode, filepath.Join(root, ".obsidian", "graph.json"))
}

func cmdStats(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root, idx := openVault(vaultFlag)
	defer idx.Close()

	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}
	folders, err := idx.Folders()
	if err != nil {
		fatal("stats: %v", err)
	}
	names := make([]string, 0, len(folders))
	total := 0
	for f, n := range folders {
		names = append(names, f)
		total += n
	}
	sort.Strings(names)
	fmt.Printf("vault: %s\n", root)
	fmt.Printf("pages: %d across %d folder(s)\n", total, len(names))
	for _, f := range names {
		fmt.Printf("  %-12s %d\n", f, folders[f])
	}
}

func cmdSearch(args []string) {
	vaultFlag, args := flagValue(args, "vault")
	typ, args := flagValue(args, "type")
	limitStr, args := flagValue(args, "limit")
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		fatal("search: a query is required")
	}
	limit := 10
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			limit = n
		}
	}

	root, idx := openVault(vaultFlag)
	defer idx.Close()

	// Keep results fresh without a running watcher: incremental reindex first.
	if _, err := ftsindex.Reindex(idx, root); err != nil {
		fatal("reindex: %v", err)
	}

	results, err := idx.Search(query, typ, limit)
	if err != nil {
		fatal("search: %v", err)
	}
	if len(results) == 0 {
		fmt.Printf("no results for %q\n", query)
		return
	}
	for _, r := range results {
		typeTag := ""
		if r.Type != "" {
			typeTag = "  (" + r.Type + ")"
		}
		fmt.Printf("• %s%s  — %s\n", r.Title, typeTag, r.Slug)
		fmt.Printf("    %s\n", strings.ReplaceAll(r.Snippet, "\n", " "))
		if rels, err := idx.RelationsFor(r.Slug); err == nil {
			for _, rel := range rels {
				fmt.Printf("    ↳ %s [[%s]] — %s\n", rel.Type, rel.Slug, rel.Reason)
			}
		}
		fmt.Printf("    %s   bm25=%.3f\n", r.RelPath, r.Rank)
	}
}

func cmdInit(args []string) {
	vaultFlag, _ := flagValue(args, "vault")
	root := vaultFlag
	if root == "" {
		root, _ = os.Getwd()
	}
	root, _ = filepath.Abs(root)
	if err := setup.ScaffoldVault(root); err != nil {
		fatal("init: %v", err)
	}
	fmt.Printf("initialized vault at %s\n", root)
}

func cmdSetup(args []string) {
	cowork, args := boolFlag(args, "--cowork")
	yes, args := boolFlag(args, "--yes")
	noMCP, args := boolFlag(args, "--no-mcp")
	noGraph, args := boolFlag(args, "--no-graph")
	vaultFlag, args := flagValue(args, "vault")
	scope, args := flagValue(args, "scope")
	pluginSrc, args := flagValue(args, "plugin-src")
	skillsDest, args := flagValue(args, "skills-dest")
	target, _ := flagValue(args, "target")

	resolvedPluginSrc := pluginSrc
	if resolvedPluginSrc == "" {
		if exe, err := os.Executable(); err == nil {
			guess := filepath.Join(filepath.Dir(exe), "..", "plugin")
			if fi, err := os.Stat(guess); err == nil && fi.IsDir() {
				resolvedPluginSrc = guess
			}
		}
	}

	// ── Cowork mode: write project-scoped config into the target folder ──────────
	if cowork {
		if target == "" {
			target = "."
		}
		if err := setup.RunCowork(setup.Options{Target: target, PluginSrc: resolvedPluginSrc}, os.Stdout); err != nil {
			fatal("setup --cowork: %v", err)
		}
		return
	}

	home, _ := os.UserHomeDir()
	defVault := vaultFlag
	if defVault == "" {
		defVault = filepath.Join(home, "brain")
	}
	if scope == "" {
		scope = "user"
	}

	opts := setup.Options{
		Vault:       defVault,
		Scope:       scope,
		PluginSrc:   resolvedPluginSrc,
		SkillsDest:  skillsDest,
		WriteGraph:  !noGraph,
		RegisterMCP: !noMCP,
	}

	// Interactive wizard unless --yes was given. Require a real terminal so a
	// piped/CI invocation fails fast instead of hanging on the TUI.
	if !yes {
		if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			fatal("setup needs a terminal; for non-interactive install pass --yes with --vault and --scope")
		}
		chosen, err := setup.RunWizard(opts)
		if err != nil {
			fatal("%v", err)
		}
		// Preserve non-interactive fields the wizard doesn't ask about.
		chosen.PluginSrc = opts.PluginSrc
		chosen.SkillsDest = opts.SkillsDest
		if noMCP {
			chosen.RegisterMCP = false
		}
		opts = chosen
	}

	if err := setup.Run(opts, os.Stdout); err != nil {
		fatal("setup: %v", err)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "mnemo: "+format+"\n", a...)
	os.Exit(1)
}
