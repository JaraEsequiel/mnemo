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

BIN="${MNEMO_BIN:-mnemo}"

# If the binary isn't runnable (cowork arch mismatch / build still in flight), the
# wiki_* tools are absent — operate the vault with file tools instead of failing silently.
if [[ "$MNEMO_BIN" == */* ]] && ! "$BIN" version >/dev/null 2>&1; then
  cat <<'FALLBACK'

### ⚠️ MARKDOWN-ONLY MODE — the wiki_* MCP tools are NOT available
The mnemo binary is not runnable here, so persist/recover the compacted work with file tools:
read/`Grep` over `Memory/` and write/edit `Memory/<type>/<slug>.md` directly. Do this anyway —
the vault markdown is the source of truth.
FALLBACK
else
  "$BIN" context 2>/dev/null || true
fi
exit 0
