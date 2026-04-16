# Repo: dotagents

Public GitHub remote at `github.com/yourconscience/dotagents`. Contains cross-agent rules, voice, skills, scripts, docs. No personal content ever lands here.

## Cadence
Push straight to main. No PRs, no feature branches, no CI gate. Commits should still pass whatever pre-commit hooks exist locally.

## Style
Short declarative sentences. No duplication across files. No new top-level files without clear justification. Prefer editing existing sections over creating new ones.

## Pre-push check
`git diff` must contain zero personal names, sync logs, vault paths, or references to private notes before pushing. This repo is public.
