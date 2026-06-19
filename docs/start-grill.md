# mnemo — `/start` "The Grill"

> The discovery interview that `/start` runs to understand a user's memory needs
> and scaffold their vault. Adaptive and branching: a few framing questions, then
> the path forks by domain. **Every answer maps to a concrete vault decision**
> (folders, `SCHEMA.md` rules, `config.json`, decay policy, sharing model).
>
> Principle: mnemo never guesses the structure. It *interviews*, proposes, and
> only writes after the user confirms. The grill output is `SCHEMA.md` — the
> maintenance contract the agent follows forever after.

---

## How the grill works

1. Run the sections in order. Skip sections that don't apply based on earlier answers.
2. For each answer, echo back the **vault decision** it implies so the user can correct it.
3. At the end, render a **proposed vault tree + `SCHEMA.md` draft** and ask for one final confirmation before writing anything.
4. Store the raw answers in `.mnemo/start-answers.json` so `/start --revise` can re-run without starting from scratch.

---

## Section 0 — Framing  *(1 question, always)*

**Q0. What is this memory FOR?**
- A code project / software work
- A research topic (deep-dive over weeks/months)
- A book or course (companion wiki)
- Personal life (goals, health, journal, self-knowledge)
- A business / team knowledge base
- Mixed / something else → free text

→ *Maps to:* default taxonomy, whether a `raw/` sources layer exists, default scopes, decay defaults.

---

## Section 1 — Memory levels  *(the core — what the user emphasized)*

Present the tiered model, then ask which tiers they need. This is the heart of the grill.

| Level | What it is | Loaded | Default |
|-------|------------|--------|---------|
| **L0 — Hot cache** | `MNEMO.md`: ~50–100 lines, top entities/terms/active context. The "decoder ring." | Every session | ✅ almost always |
| **L1 — Working memory** | Active threads/goals/projects right now. Small, churns fast. | On task start | ✅ usually |
| **L2 — Knowledge wiki** | The synthesized entity/concept pages. The bulk of the vault. | On demand (index → drill-in) | ✅ always |
| **L3 — Raw sources** | Immutable originals (papers, articles, transcripts, PDFs). mnemo reads, never edits. | On ingest | depends on domain |
| **Chronicle** | `log.md`: append-only timeline of ingests/queries/lints. | Rarely (grep) | ✅ |

**Q1a. Which levels do you need?** (multi-select; L0+L2+Chronicle pre-checked)
**Q1b. How big should the hot cache (L0) be?** tiny (~30 lines) / standard (~80) / generous (~150)
**Q1c. Should the hot cache auto-promote/demote?** (frequently-used entities rise into L0; stale ones drop to L2 only) — yes/no

→ *Maps to:* which root files/folders exist, the promotion/demotion rules in `SCHEMA.md`, the index-first lookup flow.

---

## Section 2 — Taxonomy  *(branches by Q0)*

Propose page types for the domain; user edits the list. Each type becomes a folder with its own `index.md` (slug → description).

- **Code** → `entities/` (modules, services, APIs), `decisions/`, `architecture/`, `bugs/`, `patterns/`, `glossary.md`
- **Research** → `sources/`, `concepts/`, `entities/` (people, orgs), `claims/` (with a running thesis), `open-questions.md`
- **Book** → `characters/`, `places/`, `themes/`, `plot-threads/`, `timeline.md`
- **Personal** → `areas/` (health, career, …), `people/`, `goals/`, `journal/`, `insights/`
- **Business/team** → `people/`, `projects/`, `customers/`, `processes/`, `glossary.md`

**Q2a. Confirm/edit the page types.**
**Q2b. Naming convention?** lowercase-hyphenated (default) / other.
**Q2c. Frontmatter style?** YAML frontmatter / bold-key header block (Anthropic style) / minimal.

→ *Maps to:* the folder scaffold, per-folder `index.md` columns, `description:` source for the index.

---

## Section 3 — Scope & sharing

**Q3a. Solo or shared?** just me / shared with a team.
**Q3b. Cross-machine?** sync across machines (git is the default transport) — yes/no.
**Q3c. Do you need scope tiers?** (personal vs project vs global vs team-shared) — engram-style scoping, or keep it flat.

→ *Maps to:* whether pages carry a `scope:` field, `.gitignore` for the derived index, sync workflow in `SCHEMA.md`.

---

## Section 4 — Lifecycle

**Q4a. Does knowledge go stale here?** yes (research/decisions age) / no (mostly timeless).
**Q4b. Want review cycles?** auto-flag pages for review after N months by type (engram-style: decision=6mo, preference=3mo) — yes/no/custom.
**Q4c. Revise-in-place or append-only?** pages get updated as understanding evolves (default) / never overwrite, only append + supersede.

→ *Maps to:* optional `review_after`/`expires_at` frontmatter, the lint "stale" check, supersede vs upsert behavior.

---

## Section 5 — Workflow style

**Q5a. Ingest cadence?** supervised, one source at a time (review each) / batch with light supervision.
**Q5b. Confirm before writes?** always (default) / trust the agent for routine updates.
**Q5c. Contradiction handling?** when a new source contradicts an existing page → flag for me / auto-supersede with a note / ask each time.

→ *Maps to:* the ingest skill's interaction model, the conflict/supersede policy (phase 2: FTS5 candidates + LLM-judge).

---

## Section 6 — Outputs

**Q6a. Output formats beyond markdown pages?** tables / Marp slide decks / charts / canvas / just MD.
**Q6b. Obsidian graph view?** ship an opinionated `graph.json` (color groups by type) / leave it alone.
**Q6c. File good answers back into the wiki?** yes — a useful query result becomes a new page (default) / no, keep queries ephemeral.

→ *Maps to:* the query skill's output menu, graph config bootstrap, "file-back" default.

---

## Output of the grill

1. **`config.json`** — `{ domain, levels, taxonomy, naming, frontmatter_style, scopes, decay, workflow, outputs }`.
2. **`SCHEMA.md`** — the human-readable maintenance contract: directory map, page formats, index conventions, lookup flow (index → drill-in), promotion/demotion rules, ingest/query/lint workflows, contradiction policy. This is the file the agent re-reads every session.
3. **Vault scaffold** — folders + stub `index.md` per folder + empty `MNEMO.md`/`log.md`.
4. **First index build** — the FTS5 derived index over the (mostly empty) vault.
