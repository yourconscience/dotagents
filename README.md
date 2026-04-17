# dotagents

Private repo for my shared authored `~/.agents` layer.

This repo is under active development and currently holds shared rules plus cross-tool skills for Claude Code, Codex, and OpenCode.

## Skills

- `grill-me` - pressure-test a plan one question at a time until scope and decisions are concrete.
- `gws` - use the Google Workspace CLI for Gmail, Drive, Docs, Sheets, and Calendar workflows.
- `jobcheck` - analyze fit for a job posting, generate a focused interview quiz, and grade answers.
- `jobsearch` - keep a lightweight local job-search tracker updated from Gmail, LinkedIn MCP, and exports.
- `omx` - spawn oh-my-codex in a detached tmux session and delegate a substantial task to it.
- `pr-triage` - inspect PR failures and unresolved review threads, then drive a single fix-commit-push loop.
- `repo-eval` - find, triage, and deep-evaluate GitHub repos for a given need.
- `spec` - produce a small `SPEC.md` for complex or ambiguous work before implementation.
- `tech-search` - gather high-signal opinions from tech communities and blogs on a topic.
- `x-cli` - unofficial CLI for `x` tooling.

## dotagents CLI

This repo ships a small `dotagents` CLI for two things:
- `status` reports whether `~/.agents` and the configured agent skill roots match this repo.
- `sync` fixes only the symlink scope owned by this repo and reports managed vs external skills.

The default agent targets live in [dotagents.yaml](./dotagents.yaml). `--agents` is a one-run override.

Check status with:

```bash
go run ./bin/dotagents/main.go status
```

Sync the configured agent roots with this repo:

```bash
go run ./bin/dotagents/main.go sync
```

Limit a single run to specific agents:

```bash
go run ./bin/dotagents/main.go status --agents=codex
go run ./bin/dotagents/main.go sync --agents=claude-code,codex
```

Ownership rules:
- `~/.agents` is expected to be a symlink to this repo and `sync` fixes a missing or drifted symlink.
- Repo skills under `skills/` are managed in each selected agent's skill root, including `omx`.
- Anything else already present under an agent skill root is reported as `external` and left untouched.
- If a managed skill path already exists as a real file or directory instead of a symlink, `sync` stops and reports a conflict.
