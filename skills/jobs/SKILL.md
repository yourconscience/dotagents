---
name: jobs
description: "Track a job search pipeline, analyze fit for postings, generate interview quizzes, and grade answers. Modes: `/jobs` syncs evidence and updates tracker; `/jobs check <url>` runs fit-gap analysis; `/jobs status` shows the pipeline."
---

# Jobs

Single skill for job search tracking and fit analysis. Replaces the old `jobsearch` and `jobcheck` skills.

## Modes

- **`/jobs`** (default): Pull evidence from Gmail/LinkedIn, update tracker, rewrite status dashboard, report changes and next actions.
- **`/jobs check <url>`**: Fit-gap analysis against a specific posting. Generates quiz, grades answers, updates tracker.
- **`/jobs status`**: Show current pipeline state from tracker without pulling new evidence.

## Workspace

- Root: `~/Workspace/jobsearch/`
- Tracker: `~/Workspace/jobsearch/opportunities.yaml` (canonical)
- Dashboard: `~/Workspace/jobsearch/status.md` (derived, rewritten on demand)
- CV/Resume: `~/Workspace/jobsearch/cv/`
- Interview prompts: `~/Workspace/jobsearch/interview-prompts/`
- Interview prep plan: `~/Workspace/knowledge/profile/interview_prep_plan.md`
- LinkedIn exports: `~/Workspace/jobsearch/linkedin-exports/` (historical seed only)
- Company research: `~/Workspace/jobsearch/company-research/`

## Tracker Schema

`opportunities.yaml` is a YAML list. Keep the schema flat:

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

Stages: `new`, `needs-action`, `active`, `waiting`, `low-priority`, `closed`.

Rules:
- One item per opportunity. Upsert by company + role.
- Leave unknown fields blank. Do not invent placeholders.
- Dates use `YYYY-MM-DD`.
- Put ambiguous facts in `notes`.
- Preserve manual notes unless new evidence clearly supersedes them.

## Sync Mode (`/jobs`)

1. Read `opportunities.yaml`.
2. Pull evidence from Gmail (`gws`) and LinkedIn MCP.
3. Update tracker conservatively from concrete evidence only.
4. Rewrite `status.md`.
5. Report: what changed, what needs action, what is stale.

### Gmail

Use `gws` read-only. Never send mail unless explicitly asked.

```bash
gws gmail users messages list --params '{"userId":"me","q":"newer_than:30d (recruiter OR interview OR application OR hiring OR linkedin OR greenhouse OR lever OR ashby)","maxResults":25}'
```

### LinkedIn

Use LinkedIn MCP for current reads. Historical exports for seed/backfill only.
- Prefer: `get_inbox`, `search_conversations`, `get_conversation`, `get_person_profile`
- Only create entries from real process signals (DMs, role mentions, outreach).
- Ignore: profile views, feed notifications, generic connection requests.
- Never send messages or connection requests unless explicitly asked.

## Check Mode (`/jobs check <url>`)

### 1. Gather evidence

Fetch the posting via LinkedIn MCP (`get_job_details`) or WebFetch. Read candidate profile from CV at `~/Workspace/jobsearch/cv/` and LinkedIn MCP (`get_person_profile`). Do not block on MCP unavailability - use whatever is available.

### 2. Fit-gap analysis

Produce:
- `Matches X of Y required qualifications` / `A of B additional`
- One line per requirement: check / miss / uncertain + evidence
- Top strengths with evidence
- Top gaps with evidence
- Gap questions: ask what the user has actually done (separate experience gaps from profile keyword gaps)
- Concrete items to add to resume and LinkedIn profile

Be specific. Not "highlight leadership" but "add the Inworld eval pipeline ownership story with team adoption metrics."

### 3. Interview quiz (gap-targeted)

- 5-10 questions, every question maps to a gap
- Mix of technical, system design, and experience-story questions
- Each question: why it matters, what a strong answer includes, depth 1-5
- Do not quiz on covered strengths

### 4. Grade answers

Per answer: score 1-5, strengths, gaps, how to improve, weakness type (knowledge / clarity / specificity / evidence).

Overall: grade, strongest areas, weakest areas, likely interviewer concerns, next-step prep.

### 5. Update tracker

Upsert the posting in `opportunities.yaml`. Preserve existing stage/contact/comp/dates/notes. Add jobcheck date and top gaps in notes. Set `next_action` only when analysis yields a clear user-side action.

## Status Mode (`/jobs status`)

Read `opportunities.yaml`, render a concise pipeline view. Answer:
- What needs action now?
- What is stale?
- What is most promising (role, location, comp, relocation fit)?

Do not pull new evidence. Do not rewrite `status.md` unless asked.

## User Context

Bias analysis toward senior ML / AI infrastructure roles: LLM infrastructure, evaluation, production AI systems, TTS, instruction tuning, search/retrieval. Candidate profile and CV live in the workspace.
