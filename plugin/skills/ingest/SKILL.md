---
name: ingest
description: Ingest a source into the mnemo wiki — read it, discuss takeaways, then integrate the knowledge across entity/concept/decision pages, refresh the catalogs, and append to the log. Use when adding an article, paper, transcript, note, or any source to the knowledge base.
argument-hint: "[path-or-url]"
---

# /ingest — integrate a source into the wiki

The point of mnemo is that knowledge is **compiled once and kept current**, not re-derived on
every query. Ingest integrates a source across the wiki — it does not just dump a summary.

## Flow

1. **Read the source.** If it's a file, read it; if a URL, fetch it. If it's a raw original
   worth keeping, save it under `raw/` (immutable — never edit it later).
2. **Discuss takeaways** with the user briefly — what matters, what's new, what to emphasize.
3. **Integrate across pages** (a single source often touches 5–15 pages):
   - Write/update a summary page in `sources/<slug>.md`.
   - Update or create the relevant `entities/`, `concepts/`, `decisions/`, etc. pages.
   - Set `description:` on every page (feeds the catalog), and cross-link with `[[slug]]`.
   - After writing/updating a page, call `wiki_candidates(slug)` to surface similar pages,
     judge them, and record real relationships with `wiki_relate(source, target, type, reason)`
     — **always with a reason**. If the source **contradicts** an existing page, record a
     `supersedes` or `conflicts_with` relation and surface it to the user rather than silently
     overwriting.
4. **Refresh the index**: run `mnemo index` (rebuilds FTS + the folder `index.md` catalogs).
   This is the deliberate moment the structure updates — there is no background watcher.
5. **Promote** anything now central into `CLAUDE.md` (L0); keep it short.
6. **Append to the chronicle** — add a line to `log.md`:
   `## [YYYY-MM-DD] ingest | <source title>` followed by the pages touched.

## Style

- Prefer revising existing pages (same slug) over creating near-duplicates.
- One page per concept; tags carry context in this flat-by-type vault.
- Confirm before large rewrites; routine single-page updates are fine to make directly.
