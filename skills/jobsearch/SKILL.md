---
name: jobsearch
description: Track a personal job search pipeline from Gmail, LinkedIn MCP, and historical LinkedIn exports, maintain a single local YAML tracker plus Markdown status dashboard, surface stale conversations and next actions, and draft follow-ups without sending them. Use when an agent needs to review job-related email or LinkedIn activity, update the local tracker from concrete evidence, or summarize the hiring pipeline.
---

# Job Search

## Overview

Maintain one local source of truth under `~/Workspace/jobsearch/`. Treat Gmail, LinkedIn MCP, and historical LinkedIn exports as evidence inputs. `opportunities.yaml` is canonical. `status.md` is derived from it and rewritten on demand. Report changes in conversation instead of writing intermediate review files.

## Default Workflow

1. Confirm the workspace root exists at `~/Workspace/jobsearch/`.
2. Read `opportunities.yaml`. If it does not exist, create it as an empty YAML list: `[]`.
3. Pull fresh evidence from Gmail with `gws` and from LinkedIn with MCP. Use `linkedin-exports/` only to seed or backfill older history.
4. Update `opportunities.yaml` conservatively. Prefer blanks over guessed values.
5. Rewrite `status.md` from the YAML tracker.
6. Report what changed, what needs action, and which opportunities are stale.

## Workspace Layout

- Root: `~/Workspace/jobsearch/`
- Tracker YAML: `~/Workspace/jobsearch/opportunities.yaml`
- Status dashboard: `~/Workspace/jobsearch/status.md`
- LinkedIn export archive: `~/Workspace/jobsearch/linkedin-exports/`

`opportunities.yaml` is a YAML list. Use these fields and keep the schema flat:

```yaml
- company: Example Corp
  role: Senior Applied Scientist
  stage: new
  contact:
  comp:
  location: San Francisco, CA
  remote: false
  last_contact_date: 2026-04-01
  next_action: Reply to recruiter with availability
  next_action_due: 2026-04-03
  notes: Recruiter reached out on LinkedIn about an inference role.
```

Field rules:

- `remote` is `true`, `false`, or blank.
- `last_contact_date` and `next_action_due` use `YYYY-MM-DD`.
- Leave unknown fields blank instead of inventing placeholders.

## Gmail Workflow

Use `gws` in read-only mode by default.

1. Run `gws auth status`.
2. If auth is missing, tell the user to run:

```bash
gws auth setup
gws auth login
```

3. Use targeted Gmail queries to inspect job-related threads and update `opportunities.yaml` directly from concrete evidence.

```bash
gws gmail users messages list --params '{"userId":"me","q":"newer_than:30d (recruiter OR interview OR application OR hiring OR linkedin OR greenhouse OR lever OR ashby)","maxResults":25}'
gws gmail users messages list --params '{"userId":"me","q":"newer_than:90d from:linkedin.com","maxResults":25}'
```

4. Update the tracker only when the email provides evidence for a row change.
5. Treat alerts, newsletters, and generic account mail as noise even if they match broad search terms.
6. Never send mail unless the user explicitly asks. Drafts are allowed, but do not send without confirmation.

## LinkedIn Workflow

Use LinkedIn MCP for ongoing reads. Keep manual exports only as historical input.

1. Use existing files in `~/Workspace/jobsearch/linkedin-exports/` to seed the tracker or backfill old history when needed.
2. For current activity, prefer LinkedIn MCP reads such as:
   - `get_inbox(limit=...)`
   - `search_conversations(keywords=...)`
   - `get_conversation(...)`
   - `get_person_profile(..., sections="experience,contact_info")`
3. Create or update tracker entries only when the evidence shows a real process signal such as a direct message, invitation note, explicit role mention, company-specific outreach, or clear follow-up context.
4. Ignore passive signals such as profile views, feed notifications, generic connection requests, and job alerts with no actual conversation.
5. If the same company and contact already map to an existing active item, update that item instead of creating a duplicate.
6. Do not send LinkedIn messages or connection requests unless the user explicitly asks.

## Tracker Update Rules

Track one YAML item per opportunity or process.

Prefer these stages:

- `new`: A fresh inbound lead or thread that has not been triaged.
- `needs-action`: The user owes a reply, follow-up, or scheduling action.
- `active`: A live process is underway.
- `waiting`: The user already responded or completed a step and is waiting on the other side.
- `closed`: Rejected, withdrawn, declined, or otherwise complete.

Use these rules:

- Create a new item when a company or role is clearly distinct from existing entries.
- Reuse the item when the same company, contact, and thread represent the same process.
- Keep fields blank if the source does not support a reliable value.
- If LinkedIn evidence suggests a company or role but not both, keep the uncertain field blank and explain the ambiguity in `notes`.
- Put ambiguous facts in `notes` instead of forcing them into structured fields.
- Update `last_contact_date` whenever new evidence appears.
- Update `next_action` and `next_action_due` only when there is a clear user-side action.
- Move stale entries to `needs-action` when the user should follow up.
- Preserve manually added notes unless the new evidence clearly supersedes them.

## Rendering And Review

Rewrite `status.md` directly from `opportunities.yaml` after tracker edits. Keep it concise and action-oriented. Use the dashboard to answer:

- What changed since the last pass?
- Which opportunities need action now?
- Which active or waiting items are stale?
- Which opportunities are most promising based on role, location, comp, or relocation fit?

## Low-Maintenance Bias

Optimize for minimum human maintenance:

- Let the agent read Gmail, LinkedIn MCP, and historical exports, then update the tracker.
- Do not ask the user to fill in fields that can be inferred from evidence.
- Use optional fields freely. The YAML is intentionally sparse.
- Prefer concise notes and actionable next steps over complete CRM-style records.
