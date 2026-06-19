---
name: query
description: Answer a question against the mnemo wiki — search the index, read relevant pages, synthesize a cited answer, and optionally file the answer back as a new page. Use when the user asks to recall, compare, or synthesize knowledge from the vault.
argument-hint: "<question>"
---

# /query — answer from the wiki

## Flow

1. **Find** relevant pages: `wiki_search` for keywords, or `wiki_list <folder>` to browse a
   catalog. Use `CLAUDE.md` (already in context) for quick lookups first.
2. **Read** the top pages with `wiki_get <slug>` for full content.
3. **Synthesize** an answer that **cites the slugs** it draws from (e.g. "per [[jwt-auth-model]]").
   Pick the right shape: prose, a comparison table, a Marp deck, or a chart.
4. **File it back (optional but encouraged).** A useful result — a comparison, an analysis, a
   connection you discovered — shouldn't vanish into chat. Offer to save it as a new page
   (`concepts/` or `sources/`), set its `description:`, cross-link it, and `mnemo index`.
   This is how explorations compound alongside ingested sources.

## Notes

- If the wiki lacks the answer, say so and suggest `/ingest`-ing a source or running a search.
- Don't invent facts not in the vault; mark gaps explicitly so `/lint` can pick them up.
