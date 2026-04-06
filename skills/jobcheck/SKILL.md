---
name: jobcheck
description: Analyze fit for a job posting, generate a gap-focused interview quiz, grade interview answers against the role, and update the local jobsearch tracker. Use when the user shares a job post and candidate profile or resume, wants LinkedIn-style fit-gap analysis, wants missing profile points identified, wants interview questions generated, or wants answers graded with strengths and weaknesses.
---

# Job Check

Use this skill to assess a candidate against a job, pressure-test readiness, produce actionable profile improvements, and keep the local opportunity tracker current.

## Inputs

Typical inputs:

- a job posting URL or job ID
- a LinkedIn profile URL or username
- resume text, resume file, or pasted experience summary
- optional focus areas such as role family, geography, seniority, or compensation

## Retrieval

Prefer LinkedIn MCP when it is available.

Useful MCP reads:

- `get_job_details(job_id)`
- `get_person_profile(linkedin_username, sections=[...])`

Recommended profile sections:

- `experience`
- `education`
- `languages`
- `honors`
- `interests`
- `contact_info`
- `posts` only when recent public activity is relevant

If LinkedIn MCP is not available, ask for one of:

- pasted job text
- pasted profile text
- frozen artifacts captured earlier
- resume or profile files in the workspace

Do not block useful analysis just because live MCP is unavailable.

## Safety And Scope

- Default to read-only analysis.
- Do not send LinkedIn messages, connection requests, or any other write action unless the user explicitly asks.
- Treat LinkedIn MCP as personal-use scraping infrastructure with reliability and ToS caveats.
- Call out uncertainty whenever job requirements or profile evidence are incomplete.

## Workflow

### 1. Normalize the evidence

Extract the core facts into a stable structure:

- company
- role title
- location / remote expectations
- seniority
- must-have skills
- nice-to-have skills
- domain expectations
- evidence from the candidate profile and resume
- unknowns

For fair tool comparisons, prefer frozen artifacts over repeated live MCP calls. If benchmarking tools, capture once and reuse the same normalized artifacts.

### 2. Fit-gap analysis

Produce:

- a LinkedIn-style checklist with:
  - `Matches X of Y required qualifications`
  - `Matches A of B additional qualifications`
- one check, miss, or uncertain line per requirement with evidence from the profile or resume
- overall fit summary
- top strengths with evidence
- top gaps with evidence
- uncertain areas that need confirmation
- missing points to add to the resume
- missing points to add to the LinkedIn profile

Be concrete. Do not just say "highlight leadership". Say which project, system, metric, scope, or technology should be surfaced.

When suggesting missing points, prioritize:

- direct matches to must-have requirements
- domain-relevant production systems
- scale, reliability, latency, or quality metrics
- cross-functional leadership
- hiring-manager-friendly wording that is specific but not inflated

For each gap, ask the user what they actually know or have done in that area. The goal is to separate true experience gaps from profile keyword gaps and surface concrete evidence they can add to LinkedIn or the resume.

### 3. Generate interview quiz

Create a single-pass quiz that targets gaps only. Do not use subagents.

Quiz design rules:

- 5-10 questions total unless the user asks otherwise
- every question must map to a current gap, miss, or unresolved requirement
- use a mix of technical, system design, and experience-story questions only when they directly probe a gap
- prefer open questions over trivia unless the role clearly demands recall
- for each question include why it matters, what a strong answer should contain, and a depth score from 1-5
- do not spend quiz space on already-covered strengths

### 4. Grade answers

When the user answers the quiz, score each answer against the role and the evidence quality.

For each answer, provide:

- score from 1-5
- what was strong
- what was weak or missing
- what would improve the answer
- whether the weakness is knowledge, clarity, specificity, or lack of experience evidence

After all answers, produce:

- overall grade
- strongest areas
- weakest areas
- likely interviewer concerns
- concrete next-step prep recommendations

### 5. Auto-update jobsearch tracker

Every `jobcheck` run should create or update an entry in `~/Workspace/jobsearch/opportunities.yaml`.

Tracker rules:

- upsert by company plus role when both are known
- preserve an existing entry's `stage`, `contact`, `comp`, dates, and manual notes unless new evidence clearly supports a change
- on a new entry, set `stage: new` unless the current evidence supports a later stage
- fill `company`, `role`, `location`, and `remote` when the job posting supports them
- never invent `contact`, `comp`, `last_contact_date`, or `next_action_due`
- add a concise note with the jobcheck date, the posting source if known, and the top gaps or follow-up angle
- set `next_action` only when the current analysis yields a clear user-side action, such as updating a profile section or tailoring resume bullets

## Recommended Output Shapes

### Fit-gap analysis

```markdown
## Fit Summary

Matches X of Y required qualifications
Matches A of B additional qualifications

## Required Qualifications
- requirement - check/miss/uncertain - evidence

## Additional Qualifications
- requirement - check/miss/uncertain - evidence

## Strengths
- point - evidence

## Gaps
- point - evidence

## Gap Questions
- gap - ask what the user has actually done or knows here

## Add To Resume
- concrete bullet idea

## Add To LinkedIn
- concrete profile improvement
```

### Quiz

```markdown
## Interview Quiz

1. Question
Why it matters:
Strong answer should include:
Depth: 1-5
```

### Grading

```markdown
## Answer Review

Question:
Score: 1-5
Strengths:
Gaps:
How to improve:

## Overall Assessment
```

## Evaluation Protocol For Model Comparisons

If the user wants to compare Claude Code vs Codex on job analysis quality:

- capture job/profile artifacts once
- reuse the same normalized artifacts for both tools
- keep prompts and output schema identical
- score the outputs blind when possible
- treat live MCP retrieval as a separate integration benchmark, not the core reasoning benchmark

## Notes For This User

Bias the analysis toward senior ML / AI infrastructure roles, especially LLM infrastructure, evaluation, production AI systems, TTS, and instruction tuning, unless the specific posting clearly points elsewhere.
