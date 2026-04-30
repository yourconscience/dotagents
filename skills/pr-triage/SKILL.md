---
name: pr-triage
description: Inspect PR failed checks and unresolved review comments, fix issues, push, and safely handle publish-via-PR workflows. Use when user says /pr-triage, asks about PR status, CI failures, review comments, or wants changes published through a PR.
---

# pr-triage

Inspect, fix, commit, push. One cycle per invocation.

If no PR exists yet, create it first, then inspect it before any merge decision.

## Creation and merge gate

Use this flow when the user wants to publish changes through a PR, not just fix an existing one.

1) If the current branch does not have a PR yet, create one first, for example with `gh pr create --fill`.
2) After PR creation, immediately run the full inspect step.
3) Never merge immediately after opening a PR.
4) If there are unresolved bot threads, triage them first:
   - valid: fix, push, reply, resolve
   - wrong or low-value: resolve silently
   - ambiguous: ask the user before merging
5) If there are unresolved human threads, do not merge autonomously.
6) Never merge into `main` or `master` without the user's explicit approval.
7) Merge only when the inspect step is clean or the user explicitly accepts the remaining issues.

Special case: if the remote base branch already has commits but the local work was created as an unrelated root commit, do not open a PR from that history directly. Create a fresh branch from the remote base branch, cherry-pick the local commit(s) onto it, then open a PR so the review has a normal merge base.

## Detect context

```bash
REMOTE_URL="$(git remote get-url origin)"
OWNER="$(echo "$REMOTE_URL" | sed -E 's#.+[:/]([^/]+)/([^/.]+)(\.git)?$#\1#')"
REPO="$(echo "$REMOTE_URL" | sed -E 's#.+[:/]([^/]+)/([^/.]+)(\.git)?$#\2#')"
PR="${1:-$(gh pr view --json number --jq '.number')}"
```

## Merge conflicts

Always check merge status before inspecting reviews or checks:
```bash
gh pr view "$PR" --json mergeable,mergeStateStatus --jq '{mergeable, mergeStateStatus}'
```

If `mergeable` is `CONFLICTING`: fetch the base branch, merge it locally, resolve conflicts, commit, and push before proceeding with the rest of the inspect. Stash any uncommitted local changes first if needed, restore after.

```bash
BASE=$(gh pr view "$PR" --json baseRefName --jq '.baseRefName')
git fetch origin "$BASE"
git stash --include-untracked -m "pr-triage: stash before conflict resolution" 2>/dev/null || true
git merge "origin/$BASE" --no-edit || true
# Resolve any conflict markers in files, then:
git add <resolved-files>
git commit -m "Merge $BASE: resolve conflicts"
git push
git stash pop 2>/dev/null || true
```

Do not skip this step. A PR with conflicts cannot be merged regardless of review or CI status.

## Sync from base branch

After resolving any conflicts (or if the PR was already clean), ensure the branch is up to date with the base branch before inspecting. A stale branch means reviewers see diffs that base already fixed, and merge conflicts at merge time.

```bash
BASE=$(gh pr view "$PR" --json baseRefName --jq '.baseRefName')
git fetch origin "$BASE"
BEHIND=$(git rev-list --count HEAD..origin/$BASE)
if [ "$BEHIND" -gt 0 ]; then
  git rebase origin/$BASE || (git rebase --abort && git merge origin/$BASE --no-edit)
  git push --force-with-lease
  # The push triggers a new review round - apply the full wait-for-reviews step before inspecting
fi
```

## Inspect

1) All checks:
```bash
gh pr checks "$PR" || true
```

2) Failed checks only (exclude neutral/skipped):
```bash
gh pr view "$PR" --json statusCheckRollup --jq '.statusCheckRollup[] | select(.status == "COMPLETED") | select(.conclusion != "SUCCESS" and .conclusion != "NEUTRAL" and .conclusion != "SKIPPED") | "\(.name)\t\(.conclusion)\t\(.detailsUrl)"'
```

3) Failed job logs (extract run_id/job_id from detailsUrl):
```bash
XDG_CACHE_HOME=/tmp/ghcache gh run view <run_id> --job <job_id> --log-failed
```

4) Unresolved review threads:
```bash
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query='query($owner:String!, $repo:String!, $number:Int!){ repository(owner:$owner, name:$repo){ pullRequest(number:$number){ reviewThreads(first:100){ nodes { id isResolved isOutdated path line comments(first:20){ nodes { author{ login } body url createdAt } } } } } } }' \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | {path, line, author: .comments.nodes[0].author.login, url: .comments.nodes[0].url, body: .comments.nodes[0].body}'
```

## Review policy

Bot families (auto-resolvable): `gemini*`, `copilot*`, `cursor*`, `claude*`, `codex*`, `coderabbitai*`.

**Bot comments**: think first - bots are frequently wrong. Valid and fixed: reply "Fixed", resolve. Wrong/low-value: resolve silently.

**Human comments**: **NEVER** resolve, reply to, or comment on human threads. Fix the code silently, leave the thread for the human to verify.

To resolve a single thread programmatically (use the `id` field from the inspect query above):
```bash
gh api graphql -f query='mutation { resolveReviewThread(input: {threadId: "THREAD_ID"}) { thread { isResolved } } }'
```

To batch-resolve multiple threads:
```bash
for tid in $THREAD_IDS; do
  gh api graphql -f query='mutation { resolveReviewThread(input: {threadId: "'"$tid"'"}) { thread { isResolved } } }'
done
```

Filters:
```bash
# bot
select(.comments.nodes[0].author.login | test("^(gemini|copilot|cursor|claude|codex|coderabbitai)"; "i"))
# human
select((.comments.nodes[0].author.login | test("^(gemini|copilot|cursor|claude|codex|coderabbitai)"; "i")) | not)
```

## Fix and push workflow

After inspecting: fix the code, then commit and push. Do NOT run pre-commit manually - it triggers automatically on commit.

1) Fix the identified issues in code.
2) Stage and commit with a one-line message:
```bash
git add <files>
git commit -m "fix: address PR review feedback"
```
3) If pre-commit hook fails: read the error, fix the issue, then stage and commit again (new commit, not amend).
4) Push:
```bash
git push
```
5) Wait for new bot reviews to arrive. A push triggers fresh review rounds from CodeRabbit, Gemini, etc. that take 30-90 seconds. Enforce a minimum 60s wait, then poll until the comment count is stable for 2 consecutive polls, or 3 minutes max:
```bash
sleep 30  # first mandatory 30s
PREV_COUNT=$(gh api repos/$OWNER/$REPO/pulls/$PR/comments --jq 'length')
sleep 30  # second mandatory 30s (total >= 60s minimum)
STABLE=0
for i in 1 2 3 4; do
  COUNT=$(gh api repos/$OWNER/$REPO/pulls/$PR/comments --jq 'length')
  if [ "$COUNT" = "$PREV_COUNT" ]; then
    STABLE=$((STABLE + 1))
    if [ "$STABLE" -ge 2 ]; then break; fi
  else
    STABLE=0
  fi
  PREV_COUNT=$COUNT
  [ "$i" -lt 4 ] && sleep 30
done
```
This applies after the initial push AND after every fix-push cycle.
6) Re-run the full inspect step (checks + threads). Triage any new bot threads the same way. Repeat fix-push-wait-inspect if new valid issues are found, up to 3 cycles max to avoid infinite loops.

## Summary output

After completing all work, output a compact summary:

```
## PR #<number> Status
Merge: <MERGEABLE or CONFLICTING (resolved/unresolved)>
CI: <PASS or list failures with one-line cause>
Reviews: <N> total, <M> unresolved

Fixed (<count>): <list critical/high individually, collapse medium into count>
Ignored (<count>): <list critical/high with reason, collapse medium into count>
Needs your decision (<count>): <always list each - file:line + trade-off>
Human comments (<count>): <always list each - file:line + summary>
```

- Use reviewer's own severity when available (Gemini provides critical/high/medium). For others, infer: critical = security/data-loss/broken-logic, high = bugs/perf, medium = style/naming.
- Always show "Needs your decision" and "Human comments" even if empty.
- Focus on critical/high items and ambiguous decisions. Keep medium items collapsed.
