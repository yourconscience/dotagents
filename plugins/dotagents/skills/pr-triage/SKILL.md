---
name: pr-triage
description: Inspect PR failed checks and unresolved review comments, fix valid feedback, push, and safely handle publish-via-PR workflows. Use when user says /pr-triage, asks about PR status, CI failures, review comments, or wants changes published through a PR.
---

# pr-triage

Inspect, fix, commit, push, wait, reinspect. One bounded cycle per invocation.

Prefer the configured GitHub MCP server for PR/check/thread data when the agent can access MCP tools. Use the local `pr-triage` tool for deterministic CLI inspection and hook runtime. `$SKILL_DIR` below means this skill's own directory (the base directory reported when the skill loads); the tool ships with the skill, so this works from any install location and from inside any Go module (`-C` runs in the tool dir; `PR_TRIAGE_PWD` keeps gh on the session repo):

```bash
PR_TRIAGE_PWD="$PWD" go -C "$SKILL_DIR"/tools/pr-triage run . inspect --format markdown
PR_TRIAGE_PWD="$PWD" go -C "$SKILL_DIR"/tools/pr-triage run . inspect --format json
```

The read-only Stop hook entrypoint is:

```bash
"$SKILL_DIR"/hooks/stop.sh
```

## Creation and merge gate

Use this flow when publishing through a PR, not just fixing an existing PR.

1. If the current branch has no PR, create one first, for example with `gh pr create --fill`.
2. Immediately inspect merge state, checks, body, and unresolved active review threads.
3. Never merge immediately after opening a PR.
4. Never merge into `main` or `master` without explicit user approval.
5. Merge only when the inspect step is clean or the user explicitly accepts remaining issues.

If local commits were created on unrelated history while the remote base has commits, do not open that PR directly. Create a fresh branch from the remote base, cherry-pick local commits onto it, then open the PR.

## Inspect

Run the tool first:

```bash
PR_TRIAGE_PWD="$PWD" go -C "$SKILL_DIR"/tools/pr-triage run . inspect --format markdown
```

If MCP tools are available, use GitHub MCP to cross-check the same surface:

- PR merge state and base/head refs
- check runs and failed checks
- changed files
- unresolved active review threads
- review thread IDs for bot-only resolution
- PR body

The local tool currently uses `gh` as its CLI backend because shell hooks cannot directly call the host's in-process MCP tools.

## Review policy

Bot families: `gemini*`, `copilot*`, `cursor*`, `claude*`, `codex*`, `coderabbitai*`.

Bot comments: think first. Bots are frequently wrong. Fix the code if valid, then resolve the bot thread silently. Wrong or low-value bot comments may be resolved silently without code changes. Do not reply to bot comments unless the user explicitly asks.

Human comments: never resolve, reply to, or comment on human threads autonomously. Fix the code silently and leave the thread for the human reviewer to verify.

Ambiguous comments: ask the user before merging.

## Fix and push workflow

1. Fix identified valid issues in code.
2. Stage and commit with a short one-line message.
3. Do not run pre-commit manually; it runs on commit.
4. If pre-commit fails, read the error, fix it, stage, and commit again as a new commit.
5. Push.
6. Wait for bot reviews to arrive before resolving bot threads or declaring clean.

After every push, wait at least 60 seconds, then poll review/check state until stable for two consecutive polls or about 3 minutes total. Re-run the full inspect step after the wait.

Repeat fix-push-wait-inspect up to 3 cycles to avoid infinite loops.

## Summary output

After completing work, output a compact status:

```text
## PR #<number> Status
Merge: <MERGEABLE or CONFLICTING>
CI: <PASS or failures>
Reviews: <N> unresolved (<B> bot, <H> human)

Fixed (<count>): <critical/high individually, medium collapsed>
Ignored (<count>): <critical/high with reason, medium collapsed>
Needs your decision (<count>): <each file:line + trade-off>
Human comments (<count>): <each file:line + summary>
```

Always include "Needs your decision" and "Human comments", even if empty.
