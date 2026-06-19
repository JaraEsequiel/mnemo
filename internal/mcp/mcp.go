// Package mcp exposes a mnemo vault over the Model Context Protocol (stdio),
// so any MCP-aware agent (Claude Code, etc.) can search and read the wiki.
package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JaraEsequiel/mnemo/internal/ftsindex"
	"github.com/JaraEsequiel/mnemo/internal/vault"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const instructions = `mnemo is a markdown-first knowledge memory. The markdown vault is the source of truth.
Use wiki_search to find relevant pages by keyword (BM25), wiki_get to read a page's full content
by slug, and wiki_list to browse a folder's catalog (or list folders). Prefer searching the index
before answering, and read the per-folder catalog to navigate before drilling into a page.`

// NewServer builds the mnemo MCP server over the given vault root and index.
func NewServer(root string, idx *ftsindex.Index) *server.MCPServer {
	srv := server.NewMCPServer(
		"mnemo", "0.1.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(instructions),
	)

	srv.AddTool(
		mcp.NewTool("wiki_search",
			mcp.WithDescription("Full-text search the wiki (BM25). Returns ranked pages with a snippet. Use this first to recall prior knowledge before answering."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search keywords.")),
			mcp.WithString("type", mcp.Description("Optional page-type filter (decision, entity, concept, idea, ...).")),
			mcp.WithNumber("limit", mcp.Description("Max results (default 10).")),
		),
		handleSearch(root, idx),
	)

	srv.AddTool(
		mcp.NewTool("wiki_get",
			mcp.WithDescription("Read the full markdown content of a page by its slug. The file on disk is authoritative."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("The page slug (e.g. jwt-auth-model).")),
		),
		handleGet(root, idx),
	)

	srv.AddTool(
		mcp.NewTool("wiki_list",
			mcp.WithDescription("Browse the wiki. With a folder, returns that folder's catalog (slug + description). Without one, lists all folders and their page counts."),
			mcp.WithString("folder", mcp.Description("Optional folder name (decisions, entities, ...). Omit to list folders.")),
		),
		handleList(root, idx),
	)

	srv.AddTool(
		mcp.NewTool("wiki_candidates",
			mcp.WithDescription("Find pages lexically similar to a given page that are NOT already related — candidates for a contradiction or relationship. You (the agent) then judge each and call wiki_relate to record a verdict."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("The page to find related/conflicting candidates for.")),
			mcp.WithNumber("limit", mcp.Description("Max candidates (default 5).")),
		),
		handleCandidates(root, idx),
	)

	srv.AddTool(
		mcp.NewTool("wiki_hot",
			mcp.WithDescription("Suggest what to promote into / demote from the CLAUDE.md hot cache (L0), based on inbound links and recency. You decide and edit CLAUDE.md; mnemo only computes signals."),
			mcp.WithNumber("days", mcp.Description("Staleness threshold in days for demotion (default 30).")),
			mcp.WithNumber("top", mcp.Description("Max promote suggestions (default 15).")),
		),
		handleHot(root, idx),
	)

	srv.AddTool(
		mcp.NewTool("wiki_relate",
			mcp.WithDescription("Record a typed, reasoned relation between two pages. Writes a managed '## Related' block into the source page (markdown is the source of truth). A reason is REQUIRED — every relation must explain WHY the pages are related."),
			mcp.WithString("source", mcp.Required(), mcp.Description("Source page slug (the relation is written here).")),
			mcp.WithString("target", mcp.Required(), mcp.Description("Target page slug.")),
			mcp.WithString("type", mcp.Required(), mcp.Description("Relation: supersedes | conflicts_with | related | refines | depends_on.")),
			mcp.WithString("reason", mcp.Required(), mcp.Description("Why they are related — a short explanation. REQUIRED.")),
		),
		handleRelate(root, idx),
	)

	return srv
}

// Serve runs the MCP server over stdio until the client disconnects.
func Serve(srv *server.MCPServer) error { return server.ServeStdio(srv) }

func handleSearch(root string, idx *ftsindex.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, _ := req.GetArguments()["query"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		limit := intArg(req, "limit", 10)
		if strings.TrimSpace(query) == "" {
			return mcp.NewToolResultError("query is required"), nil
		}
		// Keep results fresh: incremental reindex before searching.
		if _, err := ftsindex.Reindex(idx, root); err != nil {
			return mcp.NewToolResultError("reindex failed: " + err.Error()), nil
		}
		results, err := idx.Search(query, typ, limit)
		if err != nil {
			return mcp.NewToolResultError("search failed: " + err.Error()), nil
		}
		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No results for %q.", query)), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d result(s) for %q:\n\n", len(results), query)
		for _, r := range results {
			typeTag := ""
			if r.Type != "" {
				typeTag = " (" + r.Type + ")"
			}
			fmt.Fprintf(&sb, "- %s%s — slug: %s\n  %s\n  %s\n",
				r.Title, typeTag, r.Slug,
				strings.ReplaceAll(r.Snippet, "\n", " "), r.RelPath)
			if rels, err := idx.RelationsFor(r.Slug); err == nil {
				for _, rel := range rels {
					fmt.Fprintf(&sb, "  %s [[%s]] — %s\n", rel.Type, rel.Slug, rel.Reason)
				}
			}
		}
		sb.WriteString("\nUse wiki_get with a slug to read the full page.")
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func handleGet(root string, idx *ftsindex.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug, _ := req.GetArguments()["slug"].(string)
		if strings.TrimSpace(slug) == "" {
			return mcp.NewToolResultError("slug is required"), nil
		}
		rel, err := idx.PathForSlug(slug)
		if err != nil {
			return mcp.NewToolResultError("lookup failed: " + err.Error()), nil
		}
		if rel == "" {
			return mcp.NewToolResultError(fmt.Sprintf("no page with slug %q", slug)), nil
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return mcp.NewToolResultError("read failed: " + err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("# %s\n\n%s", rel, string(data))), nil
	}
}

func handleList(root string, idx *ftsindex.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		folder, _ := req.GetArguments()["folder"].(string)
		folder = strings.Trim(strings.TrimSpace(folder), "/")

		if folder == "" {
			folders, err := idx.Folders()
			if err != nil {
				return mcp.NewToolResultError("list failed: " + err.Error()), nil
			}
			if len(folders) == 0 {
				return mcp.NewToolResultText("The vault has no indexed pages yet."), nil
			}
			names := make([]string, 0, len(folders))
			for f := range folders {
				names = append(names, f)
			}
			sort.Strings(names)
			var sb strings.Builder
			sb.WriteString("Folders:\n")
			for _, f := range names {
				fmt.Fprintf(&sb, "- %s (%d)\n", f, folders[f])
			}
			return mcp.NewToolResultText(sb.String()), nil
		}

		pages, err := idx.ListFolder(folder)
		if err != nil {
			return mcp.NewToolResultError("list failed: " + err.Error()), nil
		}
		if len(pages) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No pages in folder %q.", folder)), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%s — %d page(s):\n", folder, len(pages))
		for _, p := range pages {
			desc := p.Description
			if desc == "" {
				desc = p.Title
			}
			fmt.Fprintf(&sb, "- %s — %s\n", p.Slug, desc)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func handleHot(root string, idx *ftsindex.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		days := intArg(req, "days", 30)
		top := intArg(req, "top", 15)
		if _, err := ftsindex.Reindex(idx, root); err != nil {
			return mcp.NewToolResultError("reindex failed: " + err.Error()), nil
		}
		signals, err := idx.PageSignals()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		inCache := vault.ReferencedSlugs(filepath.Join(root, "CLAUDE.md"))
		now := time.Now().Unix()
		var promote, demote []ftsindex.Signal
		for _, s := range signals {
			ageDays := int64(1 << 30)
			if s.MtimeUnix > 0 {
				ageDays = (now - s.MtimeUnix) / 86400
			}
			switch {
			case !inCache[s.Slug] && s.Inbound >= 1 && len(promote) < top:
				promote = append(promote, s)
			case inCache[s.Slug] && s.Inbound == 0 && ageDays > int64(days):
				demote = append(demote, s)
			}
		}
		var sb strings.Builder
		sb.WriteString("Hot-cache (CLAUDE.md) suggestions. You decide and edit CLAUDE.md.\n\nPROMOTE:\n")
		if len(promote) == 0 {
			sb.WriteString("  (none)\n")
		}
		for _, s := range promote {
			fmt.Fprintf(&sb, "  + %s (%d inbound)\n", s.Slug, s.Inbound)
		}
		sb.WriteString("DEMOTE (stale & unreferenced):\n")
		if len(demote) == 0 {
			sb.WriteString("  (none)\n")
		}
		for _, s := range demote {
			fmt.Fprintf(&sb, "  - %s\n", s.Slug)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func handleCandidates(root string, idx *ftsindex.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug, _ := req.GetArguments()["slug"].(string)
		if strings.TrimSpace(slug) == "" {
			return mcp.NewToolResultError("slug is required"), nil
		}
		limit := intArg(req, "limit", 5)
		if _, err := ftsindex.Reindex(idx, root); err != nil {
			return mcp.NewToolResultError("reindex failed: " + err.Error()), nil
		}
		cands, err := idx.Candidates(slug, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(cands) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No unrelated candidates for %q.", slug)), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Candidates related to %q — read each and judge the relationship:\n\n", slug)
		for _, c := range cands {
			fmt.Fprintf(&sb, "- %s (slug: %s)\n  %s\n", c.Title, c.Slug, strings.ReplaceAll(c.Snippet, "\n", " "))
		}
		sb.WriteString("\nFor any real relationship, call wiki_relate(source, target, type, reason). " +
			"Always include a reason. Types: supersedes, conflicts_with, related, refines, depends_on.")
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func handleRelate(root string, idx *ftsindex.Index) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		source, _ := req.GetArguments()["source"].(string)
		target, _ := req.GetArguments()["target"].(string)
		relType, _ := req.GetArguments()["type"].(string)
		reason, _ := req.GetArguments()["reason"].(string)

		if strings.TrimSpace(reason) == "" {
			return mcp.NewToolResultError("reason is required — every relation must explain WHY the pages are related"), nil
		}
		if !vault.ValidRelationType(relType) {
			return mcp.NewToolResultError("invalid type — use: supersedes, conflicts_with, related, refines, depends_on"), nil
		}
		srcPath, err := idx.PathForSlug(source)
		if err != nil {
			return mcp.NewToolResultError("lookup failed: " + err.Error()), nil
		}
		if srcPath == "" {
			return mcp.NewToolResultError(fmt.Sprintf("no source page with slug %q", source)), nil
		}
		warn := ""
		if tp, _ := idx.PathForSlug(target); tp == "" {
			warn = fmt.Sprintf(" (note: no page with slug %q exists yet — the link will resolve once it does)", target)
		}

		if err := vault.AddRelation(filepath.Join(root, filepath.FromSlash(srcPath)),
			vault.Relation{Type: relType, Target: target, Reason: reason}); err != nil {
			return mcp.NewToolResultError("write failed: " + err.Error()), nil
		}
		if _, err := ftsindex.Reindex(idx, root); err != nil {
			return mcp.NewToolResultError("relation written but reindex failed: " + err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Recorded: %s %s [[%s]] — %s%s", source, relType, target, reason, warn)), nil
	}
}

func intArg(req mcp.CallToolRequest, key string, def int) int {
	if v, ok := req.GetArguments()[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}
