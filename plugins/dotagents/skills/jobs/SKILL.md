---
name: jobs
description: "Use when tracking a job search pipeline, analyzing fit for postings, generating interview quizzes, or grading answers. Modes: `/jobs` syncs evidence and updates tracker; `/jobs check URL` runs fit-gap analysis; `/jobs scan` discovers new roles; `/jobs status` shows the pipeline."
---

# Jobs

Single skill for job search tracking and fit analysis.

## Modes

- **`/jobs`** (default): Pull evidence from Gmail/LinkedIn, update tracker, report changes and next actions.
- **`/jobs check <url>`**: Fit-gap analysis against a specific posting. Generates quiz, grades answers, updates tracker.
- **`/jobs scan`**: Run portal scanner to discover new roles at tracked companies via ATS APIs (zero tokens).
- **`/jobs status`**: Show current pipeline state from tracker without pulling new evidence.

## Workspace

Private data lives in the knowledge vault (`$KNOWLEDGE_DIR/skills/jobs/`).
Code and prompts stay in the skill root (`skills/jobs/`).

- Tracker: `$KNOWLEDGE_DIR/skills/jobs/opportunities.yaml` (canonical, single source of truth)
- CV/Resume: `$KNOWLEDGE_DIR/skills/jobs/cv/`
- Interview prep: `$KNOWLEDGE_DIR/skills/jobs/interview-prep/`
- Company research: `$KNOWLEDGE_DIR/skills/jobs/company-research/`
- Story bank: `$KNOWLEDGE_DIR/skills/jobs/story-bank.md`
- Portal config: `$KNOWLEDGE_DIR/skills/jobs/portals.yml`
- Cover letters: `$KNOWLEDGE_DIR/skills/jobs/cover_letters/`
- Prompts: `prompts/` (tracked, reusable voice mode interview prompts)
- Scanner: `tools/portals-scan/` (see its README for flags and config format)

## Tracker Schema

`opportunities.yaml` is a YAML list. Keep the schema flat:

```yaml
- company: Example Corp
  role: Senior Applied Scientist
  archetype: ml-infra
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

Archetypes (set during `/jobs check`): `ml-infra` (LLMOps, inference, training), `eval-research` (evaluation, benchmarks, applied research), `applied-ml` (product ML, RAG, agents, recommendations), `search-retrieval` (ranking, IR, query understanding). Hybrid is fine: `ml-infra / eval-research`.

Stages: `new`, `needs-action`, `active`, `waiting`, `low-priority`, `closed`.

Rules:
- One item per opportunity. Upsert by company + role.
- Leave unknown fields blank. Do not invent placeholders.
- Dates use `YYYY-MM-DD`.
- Put ambiguous facts in `notes`.
- Preserve manual notes unless new evidence clearly supersedes them.

## Sync Mode (`/jobs`)

1. Read `opportunities.yaml`.
2. Pull evidence from Gmail (`google-workspace`, using the `gws` CLI where relevant) and LinkedIn MCP.
3. Update tracker conservatively from concrete evidence only.
4. Report: what changed, what needs action, what is stale.

### Gmail

Use `google-workspace` / `gws` read-only. Never send mail unless explicitly asked.

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

Fetch the posting via LinkedIn MCP (`get_job_details`) or WebFetch. Read candidate profile from CV at `data/cv/` and LinkedIn MCP (`get_person_profile`). Do not block on MCP unavailability - use whatever is available.

### 2. Fit-gap analysis

Produce:
- Archetype classification (per Tracker Schema)
- `Matches X of Y required qualifications` / `A of B additional`
- One line per requirement: check / miss / uncertain + evidence
- Top strengths with evidence
- Top gaps with evidence
- Gap questions: ask what the user has actually done (separate experience gaps from profile keyword gaps)
- Concrete items to add to resume and LinkedIn profile

Be specific. Not "highlight leadership" but "add the eval pipeline ownership story with team adoption metrics."

### 3. Comp research

Run 1-2 WebSearch queries for market compensation data:
- `"{company}" "{role}" salary levels.fyi glassdoor`
- `"{role}" compensation {location} 2026`

Report what's available. If no data, say so - do not invent numbers.

### 4. Ghost job detection

Assess whether the posting is likely a real, active opening. Check:
- **Posting age**: extract date if visible. Over 60 days is a yellow flag.
- **Company hiring signals**: WebSearch for `"{company}" layoffs 2026` or `"{company}" hiring freeze`. If layoffs found, note whether they hit the same department.
- **Description quality**: does it name specific technologies, team size, scope? Generic boilerplate correlates with ghost postings.
- **Role-company fit**: does this role make sense for this company's business?

Output one of three tiers:
- **High Confidence**: multiple signals suggest a real, active opening
- **Proceed with Caution**: mixed signals worth noting
- **Suspicious**: multiple ghost indicators, investigate before investing time

Present signals as observations, not accusations. Always note legitimate explanations.

### 5. Interview quiz (gap-targeted)

- 5-10 questions, every question maps to a gap
- Mix of technical, system design, and experience-story questions
- Each question: why it matters, what a strong answer includes, depth 1-5
- Do not quiz on covered strengths

### 6. STAR+R story suggestions

For the top 2-3 gaps, suggest STAR+R stories from existing experience:
- **S**ituation, **T**ask, **A**ction, **R**esult, **R**eflection (what would you do differently)
- Check `data/story-bank.md` for reusable stories first. Append new ones.

### 7. Grade answers (after user responds to quiz)

Per answer: score 1-5, strengths, gaps, how to improve, weakness type (knowledge / clarity / specificity / evidence).

Overall: grade, strongest areas, weakest areas, likely interviewer concerns, next-step prep.

### 8. Update tracker

Upsert the posting in `opportunities.yaml`. Set `archetype` field. Preserve existing stage/contact/comp/dates/notes. Add jobcheck date, ghost job assessment, and top gaps in notes. Set `next_action` only when analysis yields a clear user-side action.

## Scan Mode (`/jobs scan`)

Run `go run ./tools/portals-scan` from the skill root. See `tools/portals-scan/README.md` for flags and config format. The scanner uses zero LLM tokens - direct ATS API calls only.

1. Run the scanner via Bash.
2. Review the new roles found.
3. For interesting roles, run `/jobs check <url>` for fit-gap analysis.
4. Scanner deduplicates against `data/opportunities.yaml` automatically.

## Status Mode (`/jobs status`)

Read `opportunities.yaml`, render a concise pipeline view. Answer:
- What needs action now?
- What is stale?
- What is most promising (role, location, comp, relocation fit)?

Do not pull new evidence.

## User Context

Bias fit-gap analysis toward the candidate's profile and job search direction in `$KNOWLEDGE_DIR/profile/USER.md`.
