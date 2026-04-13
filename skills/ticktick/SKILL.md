---
name: ticktick
description: TickTick task management - list tasks, projects, and notes via CLI (read-only, V1 API).
---

# ticktick

Read-only CLI for TickTick using the official V1 OAuth2 API. Wraps [pyticktick](https://github.com/sebpretzer/pyticktick).

## Prerequisites

Required environment variables:
```
TICKTICK_CLIENT_ID=...      # from developer.ticktick.com
TICKTICK_CLIENT_SECRET=...  # from developer.ticktick.com
TICKTICK_ACCESS_TOKEN=...   # obtained via OAuth flow (see Auth below)
```

No username/password needed - V1 API uses OAuth2 only.

## Auth

One-time setup to obtain an access token:

1. Create an app at https://developer.ticktick.com/manage
2. Set OAuth redirect URL to `http://127.0.0.1:8080/callback`
3. Export `TICKTICK_CLIENT_ID` and `TICKTICK_CLIENT_SECRET` in shell
4. Run auth flow (any tool that does V1 OAuth works):
   ```bash
   uv run --with ticktick-sdk ticktick-sdk auth
   ```
5. Copy the token to `TICKTICK_ACCESS_TOKEN`. Valid for ~180 days.

## Usage

```bash
TT="uv run --with pyticktick python ~/.agents/skills/ticktick/tools/tt.py"

# List all projects
$TT projects

# List tasks in all projects
$TT tasks

# Filter by project name (substring match)
$TT tasks --project "Заметки"

# Filter by status
$TT tasks --status incomplete

# Search task titles and content
$TT search "keyword"

# JSON output for any command
$TT --json tasks
$TT --json projects
```

## Agent Workflow

When user asks about tasks or notes:
1. Run `tt.py projects` to find relevant project
2. Run `tt.py tasks --project NAME` to list tasks in that project
3. Use `--json` for programmatic processing
4. For free-text search, use `tt.py search "query"`

## Limitations

- Read-only: no create/update/complete/delete
- V1 API only: no tags, folders, habits, or focus data
- V1 API fetches tasks per-project; listing all tasks iterates over projects
- Access token expires after ~180 days - refresh via OAuth flow
