# dotagents

Private repo for my shared authored `~/.agents` layer.

This repo is under active development and currently holds shared rules plus cross-tool skills for Claude Code, Codex, and OpenCode.

## Skills

- `ghpr` - inspect PR failures and unresolved review threads, then drive a single fix-commit-push loop.
- `grillme` - pressure-test a plan one question at a time until scope and decisions are concrete.
- `gws` - use the Google Workspace CLI for Gmail, Drive, Docs, Sheets, and Calendar workflows.
- `jobcheck` - analyze fit for a job posting, generate a focused interview quiz, and grade answers.
- `jobsearch` - keep a lightweight local job-search tracker updated from Gmail, LinkedIn MCP, and exports.
- `newskill` - create or refine local skills for this multi-harness setup with minimal ceremony.
- `spec` - produce a small `SPEC.md` for complex or ambiguous work before implementation.
- `techsearch` - gather high-signal opinions from tech communities and blogs on a topic.

## Next

- Cut over `~/.agents` to this repo after review.
- Keep the shared layer simple and only add an adapter or generated-view layer if tool formats truly diverge.
