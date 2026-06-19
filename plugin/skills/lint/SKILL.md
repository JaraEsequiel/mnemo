---
name: lint
description: Health-check the mnemo wiki — find contradictions, stale pages, orphans, missing pages, missing cross-references, and hot-cache drift, then propose fixes. Use periodically to keep the knowledge base healthy as it grows.
---

# /lint — keep the wiki healthy

The maintenance burden is what kills human wikis. mnemo's job is to make it near-zero. Run a
health pass and propose (don't silently apply) fixes.

## Checks

1. **Contradictions** — for active pages, call `wiki_candidates(slug)` to pull lexically-similar
   pages, judge each, and record verdicts with `wiki_relate(source, target, type, reason)`
   (`supersedes` / `conflicts_with`, always with a reason). Surface unresolved conflicts to the user.
2. **Stale** — pages past their `review_after`, or untouched for a long time while their topic
   stayed active. Propose review or demotion from `CLAUDE.md`.
3. **Orphans** — pages with no inbound `[[links]]`. Propose where to link them.
4. **Missing pages** — entities/concepts referenced repeatedly but lacking their own page.
5. **Missing cross-references** — pages that clearly relate but aren't linked.
6. **Catalog drift** — pages missing a `description:` (so the `index.md` shows "—"). Fill them.
7. **Hot-cache drift** — `CLAUDE.md` over ~100 lines, or containing stale/finished items.
   Propose promote/demote moves; use `mnemo stats` to see what's active.

## Output

Group findings by check, each with a concrete suggested edit. After the user approves, apply
the edits and run `mnemo index` to refresh the FTS index and the folder catalogs.

## Headless judge (no active agent)

For CI or automated runs where no interactive agent is judging, `mnemo lint --semantic` shells
out to your `claude`/`opencode` CLI (set `MNEMO_AGENT_CLI`) to judge candidate pairs — newest
pages first — and writes strong relations (`supersedes`/`conflicts_with`) with the model's reason.
`--verbose` prints every verdict; `--max N` caps calls. $0 on a Pro/Max subscription.

## Tip

`grep '^## \[' log.md | tail -10` shows recent activity — useful context for what's gone stale.
