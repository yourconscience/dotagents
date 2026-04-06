---
name: newskill
description: Create or update a local skill for this machine. Use when the user wants a new skill, wants to revise an existing skill, wants help deciding whether it should live in ~/.agents, ~/.codex, ~/.claude, or ~/.config/opencode, or wants trigger wording/resources improved.
---

# New Skill

Create or update local skills for this machine's multi-harness setup.

## Goals

- Keep the workflow simple and local-first.
- Reuse or refine existing skills before inventing new ones.
- Put the skill in the correct location for the intended tools.
- Keep `SKILL.md` concise and put only necessary detail into supporting files.

## Setup-Specific Locations

Choose placement deliberately:

- Shared across Codex, Claude, and OpenCode: `~/.agents/skills/<name>/SKILL.md`
- Codex-only: `~/.codex/skills/<name>/SKILL.md`
- Claude-only: `~/.claude/skills/<name>/SKILL.md`
- OpenCode-only: `~/.config/opencode/skills/<name>/SKILL.md`

In this setup, shared skills should usually live in `~/.agents/skills/`.

If you create a shared skill, also wire visibility for tools that do not read `~/.agents/skills` directly:

- create `~/.codex/skills/<name>` as a symlink to `~/.agents/skills/<name>`
- create `~/.claude/skills/<name>` as a symlink to `~/.agents/skills/<name>`

OpenCode reads shared skills from `~/.agents/skills` directly, so no extra symlink is needed there.

## Workflow

### 1. Start with a short interview

When the harness exposes a structured question tool, use it for the interview. OpenCode: `question`. Claude Code: `AskUserQuestion`. Codex or Codex-derived harnesses: `request_user_input` when available. Otherwise ask plain text directly.

Ask only the minimum needed. Start with these questions, and stop early if the request already answers them:

1. What is the short prompt description of the skill?
2. Should it be shared, Codex-only, Claude-only, or OpenCode-only?
3. What should trigger it, and what should definitely not trigger it?
4. Is `SKILL.md` enough, or does it need `scripts/`, `references/`, or `assets/`?

Prefer concrete examples over abstract discussion.

### 2. Search existing local skills first

Before drafting anything, inspect existing skills in:

- `~/.agents/skills`
- `~/.codex/skills`
- `~/.claude/skills`
- `~/.config/opencode/skills`

Look for:

- direct duplicates
- near-duplicates that should be updated instead of recreated
- naming collisions
- harness-specific wording that should be generalized if the skill will be shared

If a good local match already exists, prefer updating it.

### 3. Use find-skills only as search/reference

If local search is inconclusive or the user wants inspiration from community skills, use the Vercel `find-skills` workflow only as a discovery mechanism.

Allowed:

- inspect `skills.sh`
- inspect leaderboard or catalog results
- search for relevant public skills
- summarize candidate skills, links, trust signals, and relevance

Not allowed unless the user explicitly asks:

- install a skill
- add a skill from the catalog
- update catalog-installed skills
- run `skills add`, `skills update`, `skills init`, `skills check`, or any other command that mutates skill state
- copy external skills into local directories without explicit user approval

Treat external skills as references only by default.

### 4. Draft the smallest useful skill

Default to an instruction-only skill first.

Only add supporting files when there is a clear need:

- `scripts/` for repetitive or fragile deterministic work
- `references/` for detailed material that should not live in `SKILL.md`
- `assets/` only when the skill needs files used in outputs

Keep these principles:

- Put trigger logic in `description`
- Keep `SKILL.md` lean
- Avoid duplicate content between `SKILL.md` and references
- Preserve an existing skill name/interface when updating unless the user wants a breaking rename

### 5. Validate before finishing

Check:

- skill name matches the directory name
- `description` clearly says when to use it
- the body is concise and actionable
- placement matches the requested scope
- shared skills do not contain harness-specific tool names unless unavoidable

Then propose:

- 2-3 prompts that should trigger the skill
- 1 near-miss prompt that should not trigger it

## Search-Only Reference Sources

Use these as reference material, not as things to install by default:

- `https://github.com/vercel-labs/skills/blob/main/skills/find-skills/SKILL.md`
- `https://github.com/openai/skills/blob/main/skills/.system/skill-creator/SKILL.md`
- `https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md`

Use them to borrow:

- good trigger wording
- concise creation flow
- helpful guardrails

Do not mirror their full complexity if a simpler local skill is enough.

## Output Shape

When creating or updating a skill, report:

1. chosen location and why
2. new vs updated skill
3. any supporting files added
4. test prompts that should and should not trigger
