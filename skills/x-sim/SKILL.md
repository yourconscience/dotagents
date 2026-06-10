---
name: x-sim
description: Offline X audience simulation and evaluation. Use when evaluating draft tweets, handle bios, pinned-post ideas, or promotion angles against real scraped X context without posting or mutating X state.
---

# x-sim

Use `x-sim` to simulate how real X audiences may react to a draft tweet, bio, pinned post, or handle-promotion angle.

This skill is offline-only. It reads scraped X data through `x-cli`, stores local context in SQLite, and produces evaluation reports. Never post, like, reply, follow, DM, or mutate X state from this skill.

## Example: Predicting Model-Announcement Tweets

Real e2e run (2026-06-10): predict how a frontier-model announcement would land, using followed AI accounts as the audience corpus.

```bash
x-cli following @yourhandle --count 100 --json   # pick relevant accounts from who you follow
go run ./tools/x-sim init
for h in AnthropicAI _catwu trq212 karpathy thehypedotnews steipete; do
  go run ./tools/x-sim source add-account @$h
done
go run ./tools/x-sim source add-search "claude fable"
go run ./tools/x-sim sync --since 3m --limit-per-source 50   # synced 215 attributed tweets
go run ./tools/x-sim brief --topic "fable" --out /tmp/x-sim-fable-brief.md
go run ./tools/x-sim eval-tweet --text "Introducing Claude Fable 5, our most capable model yet..." \
  --topic "fable" --out /tmp/x-sim-pred.md
```

The brief surfaced the real announcement (`@claudeai`: "a Mythos-class model that we've made safe for general use") plus audience echo (`@karpathy`: "same underlying model as Mythos but with added safeguards"). Comparing the draft against that evidence showed the predicted capability framing matched, but the actual hook was the safety-derivative angle - the kind of gap this skill exists to catch before posting.

## Core Rules

- Use `x-cli --json` for X reads; do not call raw X endpoints when `x-cli` covers the task.
- Keep the canonical store local: `${XDG_DATA_HOME:-~/.local/share}/dotagents/x-sim/x-sim.sqlite`, or `X_SIM_DB` when set.
- Default context window is the last 6 months.
- Treat simulation scores as directional, not truth. Ground every claim in local scraped examples when possible.
- Run the final voice pass through `humanizer` rules: remove generic hype, keep concrete evidence, and preserve the user's direct style.
- If asked to publish, schedule, or perform account actions, stop and explain that this skill only evaluates offline.

## CLI

From this skill directory:

```bash
go run ./tools/x-sim init
go run ./tools/x-sim source add-account @handle
go run ./tools/x-sim source add-search "agent evals"
go run ./tools/x-sim sync --since 6m --limit-per-source 200
go run ./tools/x-sim brief --topic "agent evals" --out /tmp/x-sim-brief.md
go run ./tools/x-sim eval-tweet --text "draft tweet" --topic "agent evals" --out /tmp/x-sim-report.md
go run ./tools/x-sim eval-handle --bio "..." --promotion "..." --out /tmp/x-sim-handle.md
```

The CLI is intentionally deterministic. Use the model's judgment to add richer audience emulation on top of the report, but keep the report grounded in the local DB.

## Workflow

1. **Initialize and verify**
   - Run `x-cli auth status`.
   - Run `go run ./tools/x-sim init`.
2. **Add sources**
   - Add relevant accounts and searches with `source add-account` and `source add-search`.
   - For broad topics, prefer focused search strings over generic keywords.
3. **Sync context**
   - Run `sync --since 6m`.
   - If sync fails, fix `x-cli` auth or query shape first; do not use account-mutating commands.
4. **Create a brief**
   - Run `brief --topic ...`.
   - Read the source handles, recurring terms, and evidence tweets.
5. **Evaluate**
   - Use `eval-tweet` for draft tweets.
   - Use `eval-handle` for bio, pinned-post, and promotion positioning.
   - Compare scores across personas: technical builder, skeptical founder/operator, AI-agent power user, casual X reader, and contrarian critic.
6. **Humanize**
   - Rewrite the best candidate so it sounds like the user.
   - Prefer concrete nouns, actual measurements, and visible tradeoffs.
   - Cut generic hype and vague "AI thought leader" positioning.

## Output Shape

Return a short markdown review:

```text
Verdict:
- ...

Likely audience reaction:
- ...

Best revision:
...

Why:
- ...

Risks:
- ...

Evidence:
- @handle: source tweet summary/link
```

## Safety Boundary

Allowed:

- `x-cli auth status`
- `x-cli timeline user ... --json`
- `x-cli search ... --json`
- local SQLite reads/writes
- markdown reports

Disallowed:

- posting, replying, liking, reposting, following, unfollowing, bookmarking, or DMing
- reading or printing X credential files
- pretending simulated reactions are measured engagement
