# Python
Use uv for everything: uv run, uv pip, uv venv.

# Ad-hoc tooling
When writing ad-hoc CLI scripts, background daemons, or tools outside of a repo context, use Go instead of Python or bash - fast startup, static binaries, and reliable LLM-generated code.

# Style
No emojis.
No em dashes - use hyphens or colons instead.
Write concise, explicit, actionable instructions.

# Workflow
Prefer execution over discussion: research, implement, verify, then report.
Clarify ambiguous requests with targeted questions only when needed, then execute end-to-end.
For complex tasks, research the codebase, confirm understanding when needed, then execute.
Only ask for help when scripts timeout (>2 min), sudo is required, or a genuine blocker remains after reasonable attempts.
Before building custom solutions or scripts, first check whether existing tools, scripts, or skills already solve the problem.

# Epistemology
Never guess numerical values - benchmark instead of estimating.
When uncertain, measure. Say "this needs to be measured" rather than inventing statistics.

# Scaling
Validate at small scale before scaling up. Run a sub-minute version first to verify the full pipeline works. When scaling, only the scale parameter should change.

# Environment
User works locally on Mac with cmux (manaflow-ai/cmux) - a native macOS terminal built for AI agent workflows, tmux-compatible, Ghostty-based rendering.
Shell is zsh with oh-my-zsh.
Common tools: neovim, ripgrep, bat, fd, fzf, zoxide, eza, codex, bun.
Common aliases: `cc`, `cx`, `oc`, `ls`, `ll`, `cat`.
Local `~` is usually shell, tooling, or research context rather than a repo.
Before editing, restate the target repo or path.
Do not assume environment details like terminal, GPU type, or network access when they are unclear.
VPS host alias exists in ~/.ssh/config and is available for remote access when needed.

# User Questions
When asking structured questions, prefer the harness-native question tool.
OpenCode: `question`.
Claude Code: `AskUserQuestion`.
Codex or Codex-derived harnesses: `request_user_input` when available.
If the current harness does not expose a structured question tool, ask a plain-text question directly.
Default to one question per message unless batching clearly reduces back-and-forth.

# Compatibility
During refactoring, always consider whether to keep compatibility with the existing interface or make breaking changes. For production systems compatibility is often crucial. For experimental or research code simplicity is much more important - prefer clean breaks over backward-compat shims.

# Instructions
Keep durable repo-specific constraints in the nearest repo AGENTS.md so they stay discoverable.
When the user states durable constraints like "never X" or "always Y", persist them in the appropriate AGENTS.md.
Before editing in a repo, read the nearest repo AGENTS.md if present.

# Git
Never include "Co-Authored-By" lines in commit messages or PR descriptions.
Commit messages must be short single lines - no multi-line bodies unless explicitly requested.
Use the user's configured git identity for all commits and pushes - never override git config.
For PR work, check the user's existing comments and review state before taking autonomous action on review feedback.
Never merge a PR into `main` or `master` without the user's explicit approval, even when you have permission to push directly.

# Configuration
When modifying config files, use targeted edits or patches.
Do not rewrite the entire file unless the user explicitly asks for that.

# Skills
All new skills for Claude Code, Codex, and other agent harnesses should be installed in `dotagents/skills/`.
The skill directory is symlinked: `~/.agents/skills/` -> `dotagents/skills/`.
Claude Code skills are symlinked from `~/.claude/skills/skillname` -> `~/.agents/skills/skillname`.
To add a new skill:
1. Copy or create the skill in `dotagents/skills/skillname/`
2. Create symlink: `ln -s ~/.agents/skills/skillname ~/.claude/skills/skillname`
Do not install skills directly to `~/.claude/skills/` as copies - always use the dotagents source.
