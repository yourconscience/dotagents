# Tooling
Use uv for everything python-related: uv run, uv pip, uv venv.
When writing ad-hoc CLI scripts, background daemons, or tools outside of a repo context, use Go instead of Python or bash - fast startup, static binaries, and reliable LLM-generated code.

# Workflow
Prefer execution over discussion: research, implement, verify, then report.
Clarify ambiguous requests with targeted questions only when needed, then execute end-to-end.
For complex tasks, research the codebase, confirm understanding when needed, then execute.
Only ask for help when scripts timeout (>2 min), sudo is required, or a genuine blocker remains after reasonable attempts.
Before building custom solutions or scripts, first check whether existing tools, scripts, or skills already solve the problem.

# Epistemology
Never guess numerical values - benchmark instead of estimating.
When uncertain, measure. Say "this needs to be measured" rather than inventing statistics.
Validate at small scale before scaling up. 
Run a sub-minute version first to verify the full pipeline works. When scaling, only the scale parameter should change.

# Environment
User works locally on Mac with cmux (manaflow-ai/cmux), shell is zsh with oh-my-zsh.
Common tools: neovim, ripgrep, bat, fd, fzf, zoxide, eza, codex, bun.
Local `~` is usually shell, tooling, or research context rather than a repo.
Before editing, restate the target repo or path.
Do not assume environment details like terminal, GPU type, or network access when they are unclear.

# User Questions
When asking structured questions, prefer the harness-native question tool.
OpenCode: `question`.
Claude Code: `AskUserQuestion`.
Codex or Codex-derived harnesses: `request_user_input` when available.

# Compatibility
During refactoring, always consider whether to keep compatibility with the existing interface or make breaking changes. 
For production systems compatibility is often crucial. 
For experimental or research code simplicity is much more important - prefer clean breaks over backward-compat shims.

# Git
Never include "Co-Authored-By" lines in commit messages or PR descriptions.
Commit messages must be short single lines - no multi-line bodies unless explicitly requested.
Use the user's configured git identity for all commits and pushes - never override git config.
For PR work, check the user's existing comments and review state before taking autonomous action on review feedback.

# Configuration
When modifying config files, use targeted edits or patches.
Do not rewrite the entire file unless the user explicitly asks for that.

# Skills
All new skills should be installed in this repo's `skills/` directory.
The skill directory is symlinked from `~/.agents/skills/` to this repo's `skills/` directory.
To add a new skill:
1. Copy or create the skill in `skills/skillname/` within this repo
2. Create symlink: `ln -s ~/.agents/skills/skillname ~/.claude/skills/skillname`
Do not install skills directly to `~/.claude/skills/` as copies - always use this repo as the source.

# Canonical stores
- Notes and agent memory vault: `~/Documents/knowledge/` (iCloud-synced markdown)
- Durable cross-agent rules: `~/Workspace/dotagents/AGENTS.md` (this file)
- User profile and long-term context: `~/Documents/knowledge/profile/`
- Memory search layer: `memsearch` collection `ai` spanning the whole vault; index is derived and rebuildable, markdown is canonical.

# Durable memory capture
After each user message, check whether it contains a durable preference, rule, or context fact worth persisting across sessions. 
When it does, at the end of your next response propose a concrete patch and ask for explicit approval before writing. 
Skip the proposal entirely if nothing new came up or the content is already present - do not shadow-write memory.
Destinations are specified in canonical stores section.

# Destructive actions
Pause before destructive or hard-to-reverse operations (`rm -rf`, force push, `git reset --hard`, dropping DB tables, modifying shared configs). 
Explain what will happen and confirm before proceeding.
Same for changes to hook lifecycles, child-process spawning, or background daemons - recursion and fork bombs are easy to ship accidentally. Delegate to `omx` for a second pass before executing.
