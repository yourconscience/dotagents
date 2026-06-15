---
name: humanizer
description: Humanize, rewrite, or draft concise technical and personal writing so it sounds like the user rather than generic AI. Use for cover letters, job applications, benchmark updates, articles, posts, profile copy, emails, and agent prompts when the user asks to sound human, match their voice, remove AI tone, polish a draft, or adapt text using local context.
---

# humanizer

Turn a rough draft or context dump into writing that sounds like the user: direct, specific, technically credible, and not overproduced.

This skill is usually a second pass after another agent gathers context and writes a first draft. It can also create the first draft when the needed context is available.

## Inputs

Use whatever the user provides:

- Draft text to rewrite.
- Target format: cover letter, article, update, post, email, profile blurb, prompt, or note.
- Audience and stakes.
- Voice samples or prior writing.
- Source context from a repo, job post, docs, notes, or knowledge profile.

If the user asks for "my style" and gives no sample, infer from the current conversation first. For higher-stakes writing, pull local context when relevant:

- Profile: `$KNOWLEDGE_DIR/profile/USER.md`
- Work history: `$KNOWLEDGE_DIR/profile/WORK.md`
- Job stories: `$KNOWLEDGE_DIR/skills/jobs/data/story-bank.md` or `skills/jobs/data/story-bank.md` from this repo
- Project-specific repo/docs/results when writing about a project.

Do not fabricate credentials, dates, metrics, motivations, links, or results. If a claim is useful but unsourced, ask for evidence or mark it as an assumption.

## Modes

- **Rewrite**: improve an existing draft while preserving meaning.
- **Voice match**: analyze 1-3 samples, then rewrite using that rhythm, directness, and vocabulary level.
- **Voice preset**: use a named voice from `voices/`. Invoke with `--voice <name>` or "in the style of <name>". Load the voice file, follow its style rules, use its samples as few-shot examples. Available presets are markdown files in `voices/`.
- **Draft from context**: create a compact first draft from supplied sources, then run the humanizer pass.
- **Audit only**: list AI tells and concrete fixes without rewriting.

## Workflow

1. Define the job of the text.
   - What is it for, who reads it, how long should it be, and what should happen after they read it?
2. Gather only useful evidence.
   - Pull concrete facts, examples, decisions, tradeoffs, and outcomes.
   - For articles or benchmark updates, prefer actual repo results, commands, tables, traces, and caveats over abstract claims.
   - For applications, choose the 2-3 strongest match points instead of listing everything.
3. Build a short voice profile for this task.
   - Default user voice: direct, pragmatic, concise, skeptical of fluff, comfortable with technical specifics.
   - Preserve thinking style, not chat typos.
   - Use first person when it helps. Do not hide behind corporate phrasing.
4. Rewrite or draft.
   - Start with the actual point, not a ceremonial intro.
   - Use concrete nouns and verbs.
   - Keep paragraphs short. One idea per paragraph.
   - Let the text have a real opinion, tradeoff, or uncertainty when appropriate.
5. Anti-AI pass.
   - Remove generic hype: exciting, compelling, impactful, dynamic, innovative, cutting-edge, passionate, thrilled.
   - Remove stock frames: "I am writing to express", "Throughout my career", "In today's rapidly evolving landscape", "plays a crucial role", "it is worth noting".
   - Replace vague authority with sources, or delete it.
   - Replace inflated significance with what actually happened.
   - Break rule-of-three lists when they feel ornamental.
   - Prefer "is" over "serves as", "uses" over "leverages", "shows" over "showcases".
   - Remove dangling "-ing" clauses that add fake depth.
6. Honesty check.
   - Could this have been written by anyone about anything? Add specifics or cut it.
   - Is any claim unsupported? Qualify or remove it.
   - Is it too polished to sound like a person? Add a real constraint, concrete detail, or simpler sentence.
7. Return the output.
   - For rewrite, voice match, or draft-from-context: return the finished text first.
   - For audit only: use the "Quick Audit Output" format.
   - Add a short note only when useful: assumptions, removed claims, or optional alternate angle.

## Voice Presets

Named voice presets live in `voices/` as markdown files. Each file contains style rules and sample posts.

When a preset is requested (`--voice huawei`, "write this in huawei style", etc.):
1. Read `voices/<name>.md`.
2. Follow the style rules section exactly.
3. Use the sample posts as few-shot reference for tone, structure, emoji density, and vocabulary.
4. Adapt the user's input content to the voice while preserving the factual core.
5. Do NOT mix the preset voice with the default user voice. The preset fully overrides.

Available presets:
- `huawei` -- ALL CAPS corporate shitposting from ПОЛНЫЙ ХУАВЕЙ channel. Russian. Heavy emoji. Corporate satire with insider jargon.

## User Voice Defaults

- Plain English, concise but not clipped.
- Specific technical language when it carries information.
- Calm confidence rather than sales energy.
- Measured opinions and visible tradeoffs.
- Minimal ceremony.
- No forced warmth, jokes, or theatrical vulnerability.
- Avoid em dashes unless the target text really benefits from them.

## Format Guidance

### Technical Articles and Benchmark Updates

- Lead with the actual result or question.
- Include what changed, how it was measured, and what is still uncertain.
- Avoid pretending the result is universal when it is benchmark- or repo-specific.
- Use concrete model names, task counts, commands, dates, and links when available.
- Keep caveats readable, not apologetic.

### Cover Letters and Applications

- Default to 250-450 words.
- Open with the real fit between the role and the user's work.
- Use 2-3 concrete evidence threads.
- Close calmly. Do not beg, over-flatter, or inflate enthusiasm.

### Agent Prompts and Notes

- Keep instructions operational.
- Prefer direct constraints and exact paths.
- Remove motivational filler.
- Make success criteria explicit.

## Quick Audit Output

When asked to audit, use:

```text
AI tells:
- ...

Fixes:
- ...

Suggested rewrite:
...
```
