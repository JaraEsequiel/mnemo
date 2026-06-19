# mnemo

**Markdown-first knowledge memory for AI agents.** The vault of `.md` files is the source of
truth; a SQLite + FTS5 database (`.mnemo/wiki.db`) is a *derived, rebuildable* search index.

Inspired by reverse-engineering [engram](https://github.com/Gentleman-Programming/engram)
(techniques), the LLM Wiki pattern (architecture), and Anthropic's productivity plugin
(SKILL.md as a maintenance contract).

```
RAW (raw/, immutable)  →  WIKI (.md by type, TRUTH)  →  SCHEMA.md + CLAUDE.md (L0 hot cache)
                              │ index.md per folder (slug → description) · log.md chronicle
                              ▼
                       FTS5 derived index (.mnemo/wiki.db, gitignored)
                              ▲ watcher: .md change → incremental reindex (by hash)
                              │
                       MCP server  ←→  any agent (Claude Code, …)
```

## Install / build

```bash
go build -o bin/mnemo ./cmd/mnemo     # pure Go, no CGO (modernc.org/sqlite)
```

## CLI

```bash
mnemo init    --vault DIR              # scaffold a vault (.mnemo/, type folders, CLAUDE.md, log)
mnemo index   --vault DIR              # reindex FTS5 + regenerate every folder's index.md
mnemo indexes --vault DIR              # regenerate folder index.md catalogs only
mnemo search  <query> --vault DIR [--type T] [--limit N]
mnemo watch   --vault DIR [--debounce MS]   # auto-reindex on markdown change
mnemo mcp     --vault DIR              # MCP server over stdio
mnemo stats   --vault DIR
mnemo graph   --vault DIR [--mode preserve|force|skip]   # Obsidian graph.json
mnemo candidates <slug> --vault DIR    # find unrelated similar pages
mnemo relate  <source> <target> <type> <reason...> --vault DIR
mnemo hot suggest --vault DIR          # CLAUDE.md promote/demote signals
mnemo lint --semantic --vault DIR [--max N] [--verbose]   # headless LLM contradiction judge
```

## Relations (the knowledge graph)

Pages connect through **typed, reasoned edges** — a wikilink always carries a *why*. mnemo
stores them in a managed `## Related` block in each page's body (markdown stays the source of
truth; the `relations` index table is derived). `wiki_candidates` surfaces lexically-similar
pages; the agent judges them and records verdicts with `wiki_relate(source, target, type,
reason)`. Vocabulary: `supersedes`, `conflicts_with`, `related`, `refines`, `depends_on`.
Search annotates both directions (e.g. `superseded_by [[…]] — reason`).

When no interactive agent is present, `mnemo lint --semantic` shells out to your `claude` /
`opencode` CLI (`MNEMO_AGENT_CLI`) to judge candidates headlessly and write strong relations —
$0 on a Pro/Max subscription, mirroring engram's LLM-judge.

The vault is resolved by walking up for a `.mnemo/` directory, so inside a vault the
`--vault` flag is optional.

## How it works

- **vault** (`internal/vault`) — parses pages (YAML frontmatter + body + `[[wikilinks]]`),
  generates per-folder `index.md` catalogs. Markdown is authoritative.
- **ftsindex** (`internal/ftsindex`) — derived SQLite/FTS5 index; BM25 search with snippets;
  incremental reindex keyed by file hash.
- **watcher** (`internal/watcher`) — fsnotify; debounced reindex + catalog refresh on change.
- **mcp** (`internal/mcp`) — `wiki_search`, `wiki_get`, `wiki_list` over stdio.

## Memory levels

| Level | File / folder | Loaded |
|-------|---------------|--------|
| L0 hot cache | `CLAUDE.md` | every session (native to Claude Code) |
| L1 working | `working.md` | on task start |
| L2 wiki | `entities/ concepts/ projects/ ideas/ decisions/ sources/` | on demand (index → drill-in) |
| L3 raw | `raw/` (immutable) | on ingest |
| chronicle | `log.md` | rarely (grep) |

## Claude Code plugin

`plugin/` ships an MCP wiring (`.mcp.json`) and skills: `mnemo-maintainer` (always-active
contract), `/start` (the grill → scaffold), `/ingest`, `/query`, `/lint`.

## Install

**1. Get the binary** (pick one):

```bash
# Prebuilt release (no Go needed)
curl -fsSL https://raw.githubusercontent.com/JaraEsequiel/mnemo/main/scripts/get.sh | sh

# …or with Go
go install github.com/JaraEsequiel/mnemo/cmd/mnemo@latest
```

**2. Wire it into Claude Code** (pick one):

```bash
# A) Interactive wizard — vault path, MCP scope, skills, graph (no env vars to export)
mnemo setup

# B) As a Claude Code plugin (skills + MCP); uses ~/.mnemo/vault by default
claude plugin marketplace add JaraEsequiel/mnemo
claude plugin install mnemo
```

Then start Claude Code and run `/mnemo:start` to tailor your vault, or just ask it to
remember/recall things. Full guide + manual steps → **`docs/INSTALL.md`**.

> **Contributors / from source:** `./install.sh` builds the binary and runs the setup wizard.

## Design docs

- `docs/INSTALL.md` — install & test in Claude Code.
- `docs/SPEC.md` — consolidated spec and decisions.
- `docs/start-grill.md` — the `/start` discovery interview.

Plain CommonMark only — **not MDX** (see `docs/SPEC.md`).
