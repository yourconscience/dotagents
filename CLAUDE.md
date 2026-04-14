# Repo: dotagents

Public GitHub remote at `github.com/yourconscience/dotagents`. Contains cross-agent rules, voice, skills, scripts, docs. No personal content ever lands here.

## Cadence
Push straight to main. No PRs, no feature branches, no CI gate. Commits should still pass whatever pre-commit hooks exist locally.

## Scope
- `AGENTS.md` and `SOUL.md` are symlinked into `~/.agents/` and act as global rules for every session. Edits here affect all future agent work. Treat them carefully.
- `skills/` is symlinked into `~/.claude/skills/`. Skill changes are live immediately.
- `scripts/` holds ad-hoc tooling. Go preferred over bash or Python for anything non-trivial.
- `docs/` is for specs, plans, and design notes that are not themselves rules.

## Style
Short declarative sentences. No duplication across files. No new top-level files without clear justification. Prefer editing existing sections over creating new ones.

## Pre-push check
`git diff` must contain zero personal names, sync logs, vault paths, or references to private notes before pushing. This repo is public.
