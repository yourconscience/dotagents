---
name: x-cli
description: Use this skill when the task is to authenticate with X, fetch timelines, tweets, users, search results, followers, or following data through x-cli, or to troubleshoot x-cli auth, rate limits, and GraphQL query ID drift.
license: MIT
metadata:
  author: Gladium AI
  version: 1.0.0
  category: developer-tools
  tags:
    - x
    - twitter
    - cli
    - graphql
    - scraping
    - golang
---

# X CLI

Use this skill to operate `x-cli` safely and consistently.

## Use This Skill For

- Logging into X through the browser-based auth flow
- Fetching home timelines, user timelines, tweets, user profiles, search results, followers, or following lists
- Using `--json` output for downstream parsing or automation
- Diagnosing auth failures, rate limits, pagination, or GraphQL query ID rotation
- Updating or validating the CLI's endpoint/query ID registry when X changes its internal API

## Core Rules

- Prefer `x-cli` over ad hoc `curl` requests to X when the CLI surface already covers the task.
- Prefer `--json` when another tool, agent, or script will consume the output.
- Treat `~/.x-cli/credentials.json` as managed state. Use `x-cli auth login`, `status`, and `logout` instead of editing it manually.
- Use `tweet get` with either a tweet ID or full X URL.
- Expect cursor-based pagination on list commands and preserve returned cursors when continuing a session.
- If multiple commands start returning `404` responses, suspect GraphQL query ID drift in `internal/api/endpoints.go`.

## Quick Workflow

1. Verify the binary and auth state.
2. Authenticate if needed.
3. Run the smallest command that answers the task.
4. Switch to `--json` for parsing or handoff.
5. Follow pagination cursors or `--all` only when the user wants broader retrieval.
6. Use `--verbose` when debugging HTTP failures or endpoint drift.

## Prerequisites

- `x-cli` available on `PATH`, or use the repo-local binary.
- Google Chrome installed for browser-based login.
- A valid X account session.

For repo-local build and install commands, see [references/install-and-build.md](references/install-and-build.md).

## Standard Command Path

For a typical session:

```bash
x-cli auth status
x-cli auth login
x-cli timeline home --count 20
x-cli tweet get https://x.com/jack/status/20 --json
x-cli user get @jack
x-cli search "golang" --type latest --json
```

For follower graph inspection:

```bash
x-cli followers @jack --count 50
x-cli following @jack --count 50 --json
```

Load [references/command-map.md](references/command-map.md) when you need exact command shapes, pagination flags, or a quick reminder of the command surface.

## Troubleshooting

- Start with `x-cli auth status` to confirm stored credentials are present.
- Use `--verbose` to inspect request URLs, HTTP codes, and raw response details.
- If pagination stalls or you need the next page later, reuse the emitted cursor exactly.
- If commands fail after a long idle period, re-run `x-cli auth login`.
- If several commands return `404` or start failing at once, inspect `internal/api/endpoints.go` and refresh the query IDs from X's current web bundle.

For auth, rate-limit, and endpoint-drift notes, read [references/auth-and-debugging.md](references/auth-and-debugging.md).
