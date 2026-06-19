# Optional: a global mnemo section for `~/.claude/CLAUDE.md`

The plugin's hooks already inject the Memory Protocol each session. If you also want a always-on
pointer in your **global** Claude Code instructions (loaded in every project, even without the
plugin's hooks), append this block to `~/.claude/CLAUDE.md` and set your vault path:

```markdown
<!-- mnemo:begin -->
## mnemo — knowledge memory

You have **mnemo**, a markdown knowledge memory (MCP server `mnemo`, vault at
`<YOUR_VAULT_PATH>`). Use it as your long-term brain.

- **Recall first:** when a request touches a past decision, project, person, idea, or problem,
  call `wiki_search` (or `wiki_list` a folder) before answering.
- **Save proactively — don't wait to be asked.** After a decision, bug fixed, non-obvious
  discovery, convention, new idea, or project status change, create or update a page.
  Format and rules live in the `mnemo-maintainer` skill. Tag context (e.g. `#work` / `#personal`).
- After saving a decision/architecture page, run `wiki_candidates` and record real
  relationships with `wiki_relate` — always with a reason.
- The **markdown is the source of truth**; never hand-edit `.mnemo/wiki.db`.
<!-- mnemo:end -->
```

This is optional and complements (does not replace) the plugin hooks.
