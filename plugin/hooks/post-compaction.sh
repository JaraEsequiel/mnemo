#!/usr/bin/env bash
# mnemo — SessionStart (compact) hook: persist what was done, then recover context.

cat <<'PROTOCOL'
## mnemo Persistent Memory — POST-COMPACTION

You have mnemo memory tools (wiki_search, wiki_get, wiki_list, wiki_relate, wiki_candidates,
wiki_hot). The markdown vault is the source of truth.

CRITICAL after compaction — follow IN ORDER:
1. PERSIST what was accomplished before compaction: create/update mnemo pages for the key
   decisions, discoveries, and project changes from the compacted summary above. Without this,
   that work is lost from memory.
2. RECOVER context: call `wiki_search` / `wiki_list` for the topics you were working on.
3. Only THEN continue with the user's request.

Keep saving proactively from here on: after any decision, bugfix, discovery, idea, or project
status change, write it to mnemo.
PROTOCOL

mnemo context 2>/dev/null || true
exit 0
