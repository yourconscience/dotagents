# Tooling
Use uv for everything python-related: uv run, uv pip, uv venv.
When writing ad-hoc CLI scripts, background daemons, or tools outside of a repo context, use Go instead of Python or bash - fast startup, static binaries, and reliable LLM-generated code.

# Coding
Think before coding. State ambiguity explicitly, do not silently pick an interpretation. Present alternatives when the request is ambiguous. Push back if a simpler approach exists. Stop and ask rather than guess.
Simplicity first. No features beyond what was asked. No abstractions for single-use code. No flexibility that was not requested. No error handling for impossible scenarios. Would a senior engineer call this overcomplicated?
Surgical changes. Do not improve adjacent code. Do not refactor what is not broken. Match existing style. Mention unrelated dead code, do not delete it. Every changed line should trace directly to the request.
Goal-driven execution. Transform "fix the bug" into "write a failing test, then make it pass." Define success criteria before looping.
Research, implement, verify, then report. Execution over discussion.

# Compatibility
During refactoring, always consider whether to keep compatibility with the existing interface or make breaking changes.
For production systems compatibility is often crucial.
For experimental or research code simplicity is much more important - prefer clean breaks over backward-compat shims.

# Epistemology
Never guess numerical values - benchmark instead of estimating.
When uncertain, measure. Say "this needs to be measured" rather than inventing statistics.
Validate at small scale before scaling up.
Run a sub-minute version first to verify the full pipeline works. When scaling, only the scale parameter should change.

# Environment
User works locally on Mac, shell is zsh with oh-my-zsh.
Before editing, restate the target repo or path.
Do not assume environment details like terminal, GPU type, or network access when they are unclear.
Check for existing tools, scripts, or skills before building a custom solution.
Only ask for help when scripts timeout (>2 min), sudo is required, or a genuine blocker remains.

# User Questions
When asking structured questions, prefer the harness-native question tool.
Claude Code: `AskUserQuestion`.
Codex or Codex-derived harnesses: `request_user_input` when available.
Hermes: `clarify`.
OpenClaw: ask plain text (no structured question tool).

# Git
Commit messages must be short single lines - no multi-line bodies unless explicitly requested.
Use the user's configured git identity for all commits and pushes - never override git config, never include "Co-Authored-By" lines in commit messages or PR descriptions.
For PR work, check the user's existing comments and review state before taking autonomous action on review feedback.
Never post GitHub issue or PR comments directly via `gh` by default (exception: replies to bot comments like Gemini / Claude / Copilot can go direct). Write the draft, `pbcopy` it, then `open` the issue/PR URL in the browser - the user reviews and submits from there.

# Canonical stores
- ~/.agents is symlinked with dotagents repo by design to be a single source of truth for all agents. Edit the repo to make changes.
- All skills are stored in skills/ subdirectory of this repo. Use the `dotagents` skill when you need repo-level status/sync for `~/.agents` and supported agent skill roots; its skill-local CLI tool owns that workflow.
- Notes and agent memory vault: `~/Workspace/knowledge/` (local git, no remote)
- User profile and long-term context: `~/Workspace/knowledge/profile/`
- Memory search layer: `memsearch` collection `ai` spanning the whole vault; index is derived and rebuildable, markdown is canonical.

# Destructive actions
When modifying config files, use targeted edits or patches.
Pause before destructive or hard-to-reverse operations (`rm -rf`, force push, `git reset --hard`, dropping DB tables, modifying shared configs).
Explain what will happen and confirm before proceeding.
Same for changes to hook lifecycles, child-process spawning, or background daemons - recursion and fork bombs are easy to ship accidentally. Delegate to `omx` for a second pass before executing.

# Durable memory capture
After each user message, check whether it contains a durable preference, rule, or context fact worth persisting across sessions.
When it does, at the end of your next response propose a concrete patch and ask for explicit approval before writing.
Skip the proposal entirely if nothing new came up or the content is already present - do not shadow-write memory.
For long or substantive conversations on a specific topic, also consider proposing a short write-up note at the end. Trigger only when the conversation produced decisions, trade-offs, or research worth recovering later - operational back-and-forth is not a candidate. Draft for explicit approval, do not auto-write.
Destinations are specified in canonical stores section.
