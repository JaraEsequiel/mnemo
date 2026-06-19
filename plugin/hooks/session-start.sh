#!/usr/bin/env bash
# mnemo — SessionStart hook: inject the Memory Protocol + recent context.
# stdout becomes additionalContext for the session.

cat <<'PROTOCOL'
## mnemo Persistent Memory — ACTIVE PROTOCOL

You have mnemo, a markdown knowledge memory via MCP tools (wiki_search, wiki_get, wiki_list,
wiki_relate, wiki_candidates, wiki_hot). The markdown vault is the source of truth.

### RECALL — before answering anything that touches past work
If the user references a past decision, project, person, idea, or problem, call `wiki_search`
(or `wiki_list` a folder) FIRST, then answer with that context.

### SAVE proactively — do NOT wait to be asked
Create or update a page IMMEDIATELY after any of these:
- a decision (architecture, tool, convention), a bug fixed (with root cause), a non-obvious
  discovery, a pattern established, a new idea/proposal, a project status change, or a user
  preference/constraint learned.
Format: frontmatter (slug/title/type/tags/description) + What/Why/Where/Learned. Tag the
context (#work / #personal). One page per concept; revise in place when a topic evolves.

### RELATE
After saving a decision/architecture page, run `wiki_candidates` on it and record real
relationships (supersedes / conflicts_with / …) with `wiki_relate` — always with a reason.

Self-check after every task: "Did I or the user just decide something, fix a bug, learn
something non-obvious, or change a project's status? If yes → save it to mnemo now."
PROTOCOL

# Best-effort recent context — never fail the hook.
mnemo context 2>/dev/null || true
exit 0
