#!/usr/bin/env bash
# mnemo — SessionStart hook: inject the Memory Protocol + recent context.
# stdout becomes additionalContext for the session.

BIN="${MNEMO_BIN:-mnemo}"

# ── Ensure a runnable binary (cowork / cloud sandbox) ──────────────────────────
# In Cowork, `mnemo setup --cowork` syncs the *host* binary into .mnemo-bin/mnemo.
# If the sandbox is a different architecture it won't execute, so the MCP server
# (the wiki_* tools) never starts. Go is preinstalled in the sandbox, so rebuild
# the binary for this arch — but in the BACKGROUND: a cold build of modernc.org/
# sqlite far exceeds the hook timeout, so we can't block on it. The MCP becomes
# available next session; this session runs in markdown-only mode.
#
# Guarded on MNEMO_BIN being an explicit path: that is only set in Cowork, so a
# local plugin install (MNEMO_BIN unset → "mnemo" on PATH) never enters here.
mnemo_mode="ready"
if [[ "$MNEMO_BIN" == */* ]] && ! "$BIN" version >/dev/null 2>&1; then
  if command -v go >/dev/null 2>&1; then
    bindir="$(dirname "$BIN")"
    mkdir -p "$bindir" 2>/dev/null || true
    bindir="$(cd "$bindir" 2>/dev/null && pwd)"          # absolute (go install needs it)
    lock="$bindir/.build.lock"
    log="$bindir/.build.log"
    module="${MNEMO_MODULE:-github.com/JaraEsequiel/mnemo}"
    ref="${MNEMO_INSTALL_REF:-main}"
    if [[ -n "$bindir" && ! -f "$lock" ]]; then
      : > "$lock" 2>/dev/null || true
      # Build then atomically place the binary; clear the lock on the way out.
      build_script='
        bd="$1"; mod="$2"; rf="$3"; lk="$4"; lg="$5"; tgt="$6"
        { GOBIN="$bd" go install "${mod}/cmd/mnemo@${rf}"; } >"$lg" 2>&1
        built="$bd/mnemo"
        [ "$built" != "$tgt" ] && [ -f "$built" ] && mv -f "$built" "$tgt" 2>/dev/null
        rm -f "$lk"
      '
      # Fully detach so the hook returns immediately and the child never holds the
      # hook's stdout (the context pipe) open. Prefer setsid; fall back to nohup.
      if command -v setsid >/dev/null 2>&1; then
        ( setsid bash -c "$build_script" _ "$bindir" "$module" "$ref" "$lock" "$log" "$BIN" </dev/null >/dev/null 2>&1 & )
      else
        ( nohup bash -c "$build_script" _ "$bindir" "$module" "$ref" "$lock" "$log" "$BIN" </dev/null >/dev/null 2>&1 & )
      fi
    fi
    mnemo_mode="building"
  else
    mnemo_mode="no-go"
  fi
fi

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

# ── Markdown-only mode notice (binary not runnable yet) ────────────────────────
if [[ "$mnemo_mode" != "ready" ]]; then
  cat <<'FALLBACK'

### ⚠️ MARKDOWN-ONLY MODE — the wiki_* MCP tools are NOT available this session
The mnemo binary is not runnable in this sandbox, so the MCP server did not start. The vault
markdown is still the source of truth — operate it directly with file tools:
- RECALL: `Grep`/`Glob` over the vault (`Memory/`), and read `Memory/<type>/index.md` catalogs.
- SAVE: write/edit `Memory/<type>/<slug>.md` directly (same frontmatter + What/Why/Where/Learned).
- RELATIONS: edit the `## Related` block by hand if needed (no wiki_candidates/wiki_relate).
Do NOT skip recall or saving — only the search ranking and relation detection are degraded.
FALLBACK
  if [[ "$mnemo_mode" == "building" ]]; then
    printf '%s\n' "A correct binary is being built in the background and will be available next session."
  else
    printf '%s\n' "Go is not installed here, so the binary cannot be auto-built. Staying in markdown-only mode."
  fi
fi

# Best-effort recent context — only if the binary actually runs; never fail the hook.
if [[ "$mnemo_mode" == "ready" ]]; then
  "$BIN" context 2>/dev/null || true
fi
exit 0
