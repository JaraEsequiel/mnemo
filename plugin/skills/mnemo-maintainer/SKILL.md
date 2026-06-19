---
name: mnemo-maintainer
description: "ALWAYS ACTIVE — Maintenance contract for a mnemo markdown knowledge vault. You MUST keep the wiki, its per-folder index.md catalogs, and the CLAUDE.md hot cache current as work happens. The markdown is the source of truth; never hand-edit the derived index."
---

# mnemo — Maintenance Contract

You are the maintainer of a **mnemo** vault: a knowledge base where **markdown is the
source of truth**. A SQLite/FTS5 index (`.mnemo/wiki.db`) is *derived* and rebuildable —
never edit it, never treat it as authoritative.

## The memory levels

- **L0 — `CLAUDE.md`** (hot cache, ~50–100 lines): top entities, active projects, current
  context. Loaded automatically every session by Claude Code. **Keep it short.**
- **L1 — `working.md`**: active threads and goals right now.
- **L2 — the wiki**: one folder per page type (`entities/ concepts/ projects/ ideas/
  decisions/ sources/`), one `<slug>.md` per page. The bulk of the knowledge.
- **L3 — `raw/`**: immutable original sources. Read them; never edit them.
- **`log.md`**: append-only chronicle.

## Page format (always)

```markdown
---
slug: kebab-case-id
title: Human title
type: decision            # decision | entity | concept | idea | pattern | source | ...
tags: [context, project]  # tags carry context in this flat-by-type vault
description: one line — feeds the folder index.md
created: YYYY-MM-DD
updated: YYYY-MM-DD
links: [other-slug]       # outbound; also use [[other-slug]] in the body
---

# Human title

**What** / **Why** / **Where** / **Learned** (omit what doesn't apply)

---
*Links*: [[other-slug]]
```

Rules: filenames `lowercase-hyphenated.md`; **always** set `description:` (it is what the
folder catalog shows); cross-link with `[[slug]]`; one page per concept.

## When to write (proactively — do not wait to be asked)

After a decision, a bug fixed, a discovery, a convention, a new idea/proposal, or a project
status change: create or update the relevant `<slug>.md`. Revise in place (same slug) when a
topic evolves; create a new page for a genuinely new topic.

## Keep the catalogs and index current

- After adding/renaming/removing pages, run `mnemo index` (rebuilds FTS + every folder's
  `index.md`). If a watcher is running (`mnemo watch`) this happens automatically on save.
- **Never hand-edit `index.md`** — it is generated from each page's `description:`.

## Relations (the knowledge graph)

Pages connect through **typed, reasoned relations**. Every relation MUST carry a *why* —
a wikilink without a reason is not allowed.

- Find candidates: `wiki_candidates(slug)` returns lexically-similar pages not yet related.
- Judge each yourself (you are the LLM), then record real ones with
  `wiki_relate(source, target, type, reason)`.
- Vocabulary: `supersedes`, `conflicts_with`, `related`, `refines`, `depends_on`.
- mnemo writes a managed `## Related` block into the **source** page (between
  `<!-- mnemo:relations -->` markers) — never hand-edit that block; let `wiki_relate` manage it.
- After **any** mem_save-style update of a decision/architecture page, run `wiki_candidates`
  on it and resolve obvious contradictions (`supersedes`/`conflicts_with`) with a clear reason.

## Hot cache (L0 = CLAUDE.md) — auto promote/demote

- Run `wiki_hot` (or `mnemo hot suggest`) to get promote/demote suggestions from real signals
  (inbound links + recency). **You decide and edit `CLAUDE.md`**; mnemo only computes the signals.
- **Promote** an entity/project/term into `CLAUDE.md` when it becomes frequently referenced
  or is the current focus.
- **Demote** (remove from `CLAUDE.md`, keep the L2 page) when it goes stale — e.g. a project
  finished, or no mention in ~30 days.
- Keep `CLAUDE.md` to ~50–100 lines. It is a routing table into L2, not a copy of it.
- Maintain promoted entries inside a managed block in CLAUDE.md so it stays tidy.

## Lookup flow (progressive disclosure)

1. `CLAUDE.md` already in context (resolves most lookups).
2. `wiki_list` a folder's catalog to find a slug, or `wiki_search` for keywords.
3. `wiki_get` the slug for full content.

## Confirmation

Propose changes before bulk rewrites or deletions. Routine single-page upserts during active
work don't need a prompt; large restructurings do.

## Format

Plain CommonMark only. **Never MDX** — Obsidian and `CLAUDE.md` need plain markdown, and JSX
pollutes the index. Dynamic views (tables, slides) are *generated outputs*, not the source.
