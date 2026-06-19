---
name: start
description: Scaffold a new mnemo vault by interviewing the user about their memory needs (the "grill"), then writing the folder structure, SCHEMA.md, and CLAUDE.md hot cache. Use when setting up mnemo for a new directory or domain.
argument-hint: "[path]"
---

# /start — scaffold a mnemo vault

Set up a markdown knowledge vault tailored to the user. **Interview first, then write.**
Never scaffold blindly; never overwrite existing files.

## 1. Run the grill (the discovery interview)

Ask, adaptively, and echo back the vault decision each answer implies:

- **Domain** — code memory / research / personal / mixed? (drives default taxonomy + whether `raw/` exists)
- **Memory levels** — which of L0 (`CLAUDE.md` hot cache), L1 (`working.md`), L2 (wiki), L3 (`raw/`), chronicle (`log.md`)?
- **Hot cache** — size (~50–100 lines) and auto promote/demote on or off?
- **Taxonomy** — confirm/edit the page-type folders. Defaults by domain:
  - code → `entities concepts decisions architecture bugs patterns`
  - research → `sources concepts entities claims`
  - personal → `areas people goals journal insights`
  - mixed → `entities concepts projects ideas decisions sources`
- **Scope/sharing** — solo+one machine (default), multi-machine (git), or team?
- **Lifecycle** — revise-in-place (default) vs append+supersede; review cycles by type?
- **Outputs** — Obsidian graph view, Marp slides, Dataview tables, query file-back?

(Full reference: the project's `docs/start-grill.md`.)

## 2. Propose, then scaffold

Render the proposed vault tree and a draft `SCHEMA.md`. On confirmation:

```bash
mnemo init --vault <path>        # creates .mnemo/, the type folders, CLAUDE.md, working.md, log.md, .gitignore
```

Then adjust to the grill answers: create/remove folders to match the chosen taxonomy, and
write the customized `SCHEMA.md` (the maintenance contract — see the mnemo-maintainer skill)
and a starter `CLAUDE.md` (L0 hot cache, ~50–100 lines).

## 3. First index

```bash
mnemo index --vault <path>       # builds the FTS5 index + every folder's index.md
```

The index refreshes on demand — re-run `mnemo index` (or `/mnemo:ingest` / `/mnemo:lint`) after
editing markdown directly. The MCP server also reindexes on every search, so agent recall is
always fresh. There is no background watcher to manage.

## Notes

- Store the raw answers in `.mnemo/start-answers.json` so `/start` can be re-run to revise.
- Markdown is the source of truth. `.mnemo/wiki.db` is derived and gitignored.
