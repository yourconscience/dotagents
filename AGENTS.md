# Tooling

- Python: use `uv run`, `uv pip`, and `uv venv`.
- Ad-hoc CLIs, daemons, and repo-external tools: prefer Go over Python or bash.
- Check existing repo tools, scripts, and skills before building anything new.

# Work Style

- Think before coding. State ambiguity; do not silently choose a risky interpretation.
- Simplicity first. No extra features, abstractions, or flexibility unless asked.
- Keep changes surgical. Do not refactor adjacent code or delete unrelated dead code.
- Research, implement, verify, then report.
- For bugs: define success, add a focused failing test when practical, then make it pass.
- During refactors, consider compatibility. Production favors compatibility; experimental code can take clean breaks.

# Measurement

- Never guess numerical values. Measure or say it needs measurement.
- Validate at small scale before scaling. When scaling, change only the scale parameter.
- For new external package/tool references in ongoing projects, use versions published at least 3 days earlier. Verify publish age from the registry or say it still needs measurement.

# Environment

- User is in Tbilisi, Georgia (UTC+4). Use `YYYY-MM-DD` dates and 24h times with explicit timezone.
- User works locally on macOS with zsh.
- Before editing, restate the target repo or path.
- Ask for help only for sudo, >2 minute timeouts, or a real blocker.

# Canonical Stores

- `~/.agents` is symlinked to this repo. Edit this repo, not generated mirrors.
- Skills live under `skills/`.
- Use the `dotagents` skill/tool for status and sync across the primary stack: Claude Code, Codex, Hermes, Droid, and Pi/OMP. Amp, OpenClaw, OpenCode, and similar harnesses are compatibility-only unless explicitly requested.
- Knowledge vault: `$KNOWLEDGE_DIR`.
- User profile: `$KNOWLEDGE_DIR/profile/`.
- Search index: `memsearch` collection `ai`; markdown is canonical, index is derived.

# Local Sync

- Do not symlink whole agent config files. Use dotagents targeted sync for skills, native agents, and MCP entries.
- Memory hooks live at `~/.agents/memory/hooks/`; hook approval is lifecycle-sensitive and must be changed deliberately.

# Generated Artifacts

- Put generated reports, specs, and plans in markdown files.
- Report the path clearly; the user can open it with the appropriate local tool.
- Do not create reference docs with machine-specific paths, migration checklists, or inspection commands that only apply to one machine. Use env vars and conventional paths in tool READMEs instead.

# Git

- Start each new fix or feature in its own git worktree by default; keep the main checkout clean. Work directly on the main checkout only for trivial, single-commit edits.
- Preserve unrelated worktree changes: inspect `git -C <target> status` and `git worktree list` before removing, pruning, or switching worktrees, and only touch worktrees explicitly in scope.
- Create new git worktrees only under the local repository directory at `.worktrees/$worktree_name`; do not use `/tmp`, `/private/tmp`, sibling workspace dirs, or tool-specific worktree dirs unless explicitly requested.
- Commit messages: short single line; no `Co-Authored-By` or `Co-authored-by` trailers. Agents must not add bot co-author trailers to commits. Install `hooks/commit-msg` as a git hook to strip them automatically.
- Use the configured git identity. Do not override author or committer fields.
- PR descriptions: do not include "Generated with Claude Code" or similar bot attribution footers.
- Push only when asked.
- PR work defaults to `/pr-triage`: inspect comments/checks, fix valid feedback, and stop before merge unless merge is explicitly approved.
- Do not post GitHub issue/PR comments directly by default. Draft, copy, and open the URL for user review, except direct replies to bot comments are allowed.
- Destructive commands require explicit approval: `git reset --hard`, force push, dropping data, broad `rm`, config rewrites, hook lifecycle changes, background daemons.

# Durable Memory

- After each user message, decide whether it contains a durable preference/rule/context fact worth persisting.
- If yes, propose a concrete memory patch at the end of the response and ask for explicit approval before writing.
- Do not shadow-write memory.
