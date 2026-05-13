---
name: gws
description: Use when working with Google Workspace through the `gws` CLI for Drive, Docs, Gmail, Sheets, Calendar, and more.
---

# gws

CLI for Google Workspace APIs. Binary: `gws`. Config: `~/.config/gws/`.

On Hermes, prefer the bundled native `google-workspace` skill. This shared skill exists so the same workflow remains available to agents that do not ship a first-party equivalent.

## Auth

```
gws auth setup              # one-time OAuth client setup if client_secret.json is missing
gws auth login              # OAuth browser flow
gws auth status             # check current auth
```

Env: `GOOGLE_WORKSPACE_CLI_TOKEN`, `GOOGLE_WORKSPACE_CLI_CREDENTIALS_FILE`

## Drive

```
gws drive files list --params '{"q": "name contains \"report\"", "pageSize": 10}'
gws drive files get --params '{"fileId": "ID"}'
gws drive files export --params '{"fileId": "ID", "mimeType": "text/plain"}' > out.txt
gws drive files create --params '{"name": "new.txt", "mimeType": "text/plain"}' --upload file.txt
```

## Docs

```
gws docs documents get --params '{"documentId": "ID"}'
```

## Google Doc To Markdown With Comments

When the user asks to download or fetch a specific Google Doc with comments in Markdown, use the skill-local helper instead of hand-assembling the pipeline:

```
go run ~/.agents/skills/gws/tools/google_doc_markdown/main.go \
  --doc 'https://docs.google.com/document/d/FILE_ID/edit' \
  --output ./doc-with-comments.md
```

Behavior:
- Accepts either a full Google Doc URL or a raw Doc ID.
- Checks `gws auth status` first and fails with an explicit setup/login instruction if auth is missing or the token lacks Drive access.
- Exports the Google Doc as HTML through `gws drive files export`.
- Removes Google comment reference markup and omits embedded base64 images that would otherwise pollute the Markdown.
- Converts the cleaned HTML to Markdown with `pandoc`.
- Fetches threaded comments through `gws drive comments list`.
- Appends a clean `## Comments` section with author, timestamps, quoted text, comment body, and replies.

Agent workflow for this task:
- Run `gws auth status` first.
- If auth is missing or invalid, stop and tell the user to run `gws auth setup` and `gws auth login`.
- Run the helper with the doc URL and an explicit output path in the current working directory unless the user asked for a different location.
- Report the saved Markdown path back to the user.

Use this helper for prompts like:
- `use gws to download this doc with comments https://docs.google.com/document/d/.../edit`
- `fetch this Google Doc with comments as markdown`

## Gmail

```
gws gmail users messages list --params '{"userId": "me", "q": "newer_than:7d", "maxResults": 10}'
gws gmail users messages get --params '{"userId": "me", "id": "MSG_ID"}'
gws gmail users messages send --params '{"userId": "me"}' --body '{"raw": "BASE64_RFC2822"}'
```

## Sheets

```
gws sheets spreadsheets get --params '{"spreadsheetId": "ID"}'
gws sheets spreadsheets.values get --params '{"spreadsheetId": "ID", "range": "Sheet1!A1:D10"}'
gws sheets spreadsheets.values update --params '{"spreadsheetId": "ID", "range": "Sheet1!A1", "valueInputOption": "USER_ENTERED"}' --body '{"values": [["a","b"]]}'
```

## Calendar

```
gws calendar events list --params '{"calendarId": "primary", "timeMin": "2026-01-01T00:00:00Z", "maxResults": 10}'
```

## Tips

- Use `gws schema <service.resource.method>` to discover params for any endpoint.
- Add `--resolve-refs` to schema for full type details.
- Confirm before sending emails or creating events.
