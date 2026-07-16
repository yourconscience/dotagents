# Agent roles

Canonical roles are editable Markdown files under `agents/<name>.md`. YAML frontmatter stores portable metadata; the Markdown body is the role prompt.

First-run setup copies the five public starter roles into the user's private `~/.agents/agents/` directory. A user edits or replaces those files directly. dotagents renders each role only for harnesses with a verified native role surface.

## Format

```markdown
---
name: reviewer
description: Reviews code and reports findings without editing.
model: opus
effort: high
tools: Read, Glob, Grep, Bash
color: yellow
codex:
  model: gpt-5.4
  model_reasoning_effort: high
droid:
  model: inherit
  reasoning_effort: high
  tools: [Read, LS, Grep, Glob]
---

Review the requested change against its specification.
Report concrete findings with file and line references.
```

Required fields:

- `name`: unique canonical role name.
- `description`: discovery text shown by harnesses.
- Markdown body: non-empty role instructions.

Optional portable fields:

- `model`
- `effort`
- `tools` as a YAML sequence or comma-separated scalar
- `color`

Optional `codex` and `droid` mappings override harness-specific model, reasoning, or tool values. Keep overrides only when the native harness needs them.

## Rendered surfaces

| Harness | Target |
|---|---|
| Claude Code | `~/.claude/agents/<name>.md` |
| Codex | `~/.codex/agents/<name>.toml` |
| Factory Droid | `~/.factory/droids/<name>.md` |
| OMP | `~/.omp/agent/agents/<name>.md` |

Hermes and Pi do not receive invented role files because dotagents has no verified native role adapter for them.

Generated native files contain a dotagents ownership marker. Sync updates or removes only files it owns; unrelated native roles remain untouched. During setup, existing native roles can be copied into the canonical directory. Codex TOML roles are converted to canonical Markdown, and source files are never moved or deleted.
