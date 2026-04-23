# Repo: dotagents

Private GitHub remote at `github.com/yourconscience/dotagents`. Contains cross-agent rules, voice, skills, scripts, docs.

## Cadence
Push straight to main. No PRs, no feature branches, no CI gate. Commits should still pass whatever pre-commit hooks exist locally.

## Style
Short declarative sentences. No duplication across files. No new top-level files without clear justification. Prefer editing existing sections over creating new ones.

## Pre-push check
`git diff` should still avoid accidental leaks of private notes, secrets, raw vault exports, or machine-local noise before pushing.
