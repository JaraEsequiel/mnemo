#!/usr/bin/env bash
# mnemo — UserPromptSubmit hook.
# First message of a session: load the deferred wiki_* tools via ToolSearch.
# Later messages: nudge to save if nothing has been written in a while (debounced).
# MUST exit 0 and print valid JSON (or {}).

INPUT="$(cat)"

# Extract session_id without requiring jq.
SESSION_ID=""
if [[ "$INPUT" =~ \"session_id\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
  SESSION_ID="${BASH_REMATCH[1]}"
fi
KEY="$(printf '%s' "${SESSION_ID:-$PPID}" | tr -c 'a-zA-Z0-9_-' '_')"
STATE="${TMPDIR:-/tmp}/mnemo-${KEY}-init"

# ── First message: load the memory tools ──────────────────────────────────────
if [[ ! -f "$STATE" ]]; then
  : > "$STATE" 2>/dev/null || true
  printf '%s\n' '{"systemMessage":"FIRST ACTION — load mnemo memory tools now:\nselect:mcp__mnemo__wiki_search,mcp__mnemo__wiki_get,mcp__mnemo__wiki_list,mcp__mnemo__wiki_candidates,mcp__mnemo__wiki_relate,mcp__mnemo__wiki_hot\n\nThen, if this message references past work, call wiki_search before responding."}'
  exit 0
fi

# ── Subsequent messages: best-effort save nudge ───────────────────────────────
LAST="$(mnemo last-activity 2>/dev/null)"
NOW="$(date +%s 2>/dev/null)"
NUDGE="${STATE%-init}-nudge"

if [[ "$LAST" =~ ^[0-9]+$ && "$NOW" =~ ^[0-9]+$ && "$LAST" -gt 0 ]]; then
  AGE=$(( NOW - LAST ))
  LASTN=0
  [[ -f "$NUDGE" ]] && LASTN="$(cat "$NUDGE" 2>/dev/null)"
  [[ "$LASTN" =~ ^[0-9]+$ ]] || LASTN=0
  # nudge if >20min since last save AND not nudged in the last 15min
  if [[ "$AGE" -gt 1200 && $(( NOW - LASTN )) -ge 900 ]]; then
    printf '%s' "$NOW" > "$NUDGE" 2>/dev/null || true
    printf '%s\n' '{"systemMessage":"mnemo reminder: it has been a while since you saved to memory. If you have made decisions, discoveries, or finished work, save it now."}'
    exit 0
  fi
fi

printf '%s\n' '{}'
exit 0
