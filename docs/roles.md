# Agent roles

A role is a Markdown file in `~/.agents/agents/` with frontmatter (`name`, `description`, `model`, `effort`, `tools`, optional per-harness overrides) and the system prompt as body. dotagents renders it into each harness's native format — e.g. TOML for Codex. Six generic starter roles ship with the tool: `architect` `builder` `general` `researcher` `reviewer` `tester`. A same-name file in your `~/.agents/agents/` always wins over the starter.

## Model tiers and overrides

Claude Code and Droid render the tier natively in their own model family. Codex neutralizes the tier and uses its own default unless an exact per-harness `codex.model` override is set. Harnesses without a tier concept use native inheritance or an exact per-harness override:

```yaml
model: opus
claude:
  model: claude-opus-4-6
codex:
  model: gpt-5.6-sol
omp:
  model: gpt-5.6-luna-high
opencode:
  model: openai/gpt-5.6
qwen:
  model: qwen3-coder-plus
  approval_mode: plan

### Centralized model pin

To pin one model for all rendered roles that have no explicit model, set `role_model` on the agent entry in `dotagents.yaml`. Roles (or per-harness overrides) that declare a model always win over `role_model`. Exact model names belong in role frontmatter or `role_model` — never in tool code.

## Rendering targets

| Harness | Format | Path |
|---|---|---|
| Claude Code | Markdown | `~/.claude/agents/<name>.md` |
| Codex | TOML | `~/.codex/agents/<name>.toml` |
| Factory Droid | Markdown | `~/.factory/droids/<name>.md` |
| OMP | YAML frontmatter | `~/.omp/agent/agents/<name>.md` |
| Qwen Code | YAML frontmatter | `~/.qwen/agents/<name>.md` |

Roles are regenerated on every `dotagents sync`; edit the canonical `.md`, never the rendered output.
