# GOAL

## Objective

Finish the `public-prep` branch for publishing `dotagents` as a public repo, using the current branch changes and the captured session context stored at `research/public-prep-cc.md`. The next Codex goal should leave the branch in a mergeable, testable, public-safe state and produce a short final handoff for the user.

Current timestamp for this handoff: `2026-05-20` in `Asia/Tbilisi`.

## Starting State

- Target repo: `~/Workspace/dotagents`.
- Current branch: `public-prep`, tracking `origin/public-prep`.
- Relevant PR: [#65](https://github.com/yourconscience/dotagents/pull/65), titled `Redact personal data for public release`.
- PR #65 state when this handoff was written:
  - `OPEN`
  - base `main`
  - head `public-prep`
  - `mergeStateStatus: DIRTY`
  - no status checks reported on the PR
- Related PR: [#64](https://github.com/yourconscience/dotagents/pull/64), titled `Add commit-msg hook to strip agent co-author trailers`.
  - `OPEN`
  - `mergeStateStatus: CLEAN`
  - CI checks are green
- Review comments already applied locally after this handoff was first drafted:
  - removed `.antigravitycli/`;
  - moved root `cc.md` to ignored `research/public-prep-cc.md`;
  - removed untracked `cmd/dotagents/antigravity_test.go`;
  - removed the public-facing local default path from the `AGENTS.md` `$KNOWLEDGE_DIR` line.
- Remaining visible untracked files are expected to be only generated/local handoff files such as `GOAL.md` and `dotagents.yaml.tmp`, unless new work adds more.

## Context From Captured Session

The captured session did three things:

1. Prepared a sibling `dotvault` repo for public release.
   - `dotvault` was pushed to `github.com/yourconscience/dotvault`.
   - CI was reported green.
   - The recommended public positioning is: `dotvault` answers "where agents store my data"; `dotagents` answers "how agents stay in sync".
   - The decision was to keep `dotvault` and `dotagents` as separate repos, not a monorepo.

2. Created or referenced dotagents PR #64 for commit-message hygiene.
   - Adds `scripts/commit-msg`.
   - Strengthens `AGENTS.md` so agents must not add `Co-authored-by` trailers.
   - This is meant to address Droid/agent commits that append bot co-author trailers.

3. Created dotagents PR #65 for public-readiness redactions.
   - Replaces hardcoded personal knowledge paths with `$KNOWLEDGE_DIR` where appropriate.
   - Removes private framing from `README.md`.
   - Redacts a personal company/example from skill docs.
   - Replaces a local user-specific test fixture path with a generic path.
   - The session reported that critical Telegram credential files were untracked and not in git history, but still recommended rotating Telegram credentials as hygiene.

## Branch Changes To Preserve

`origin/main...HEAD` currently contains:

- `AGENTS.md`
  - `$KNOWLEDGE_DIR` docs for vault/profile paths, without advertising a local default path in public instructions.
  - Stronger commit-message rule and `scripts/commit-msg` install guidance.
- `README.md`
  - Public-facing description of `dotagents` as a cross-agent sync CLI.
- `cmd/dotagents/amp_memory_e2e_test.go`
  - Generic fixture path instead of user-specific absolute path.
- `scripts/commit-msg`
  - Hook that strips `Co-authored-by:` trailers.
- `skills/dotagents/references/memory-sync.md`
  - Redacted private example.
- `skills/humanizer/SKILL.md`
  - `$KNOWLEDGE_DIR` profile/work-history paths.
- `skills/jobs/SKILL.md`
  - Redacted private example and `$KNOWLEDGE_DIR` profile path.
- `skills/remote-access/SKILL.md`
  - `$KNOWLEDGE_DIR` cached fallback path.

## Non-Goals

- Do not edit the sibling `dotvault` repo unless the user explicitly asks.
- Do not push, force-push, merge, close, or delete branches unless the user asks.
- Do not delete untracked files without explicit approval.
- Do not commit `research/public-prep-cc.md` or `dotagents.yaml.tmp`.
- Do not broaden this into a full docs rewrite or product roadmap.
- Do not add new public-facing claims that are not verified in the repo or captured session context.

## Main Work

1. Re-check the repo and PR state.
   - Run `git status --short --branch`.
   - Run `git worktree list`.
   - Run `git diff --stat origin/main...HEAD`.
   - Run `gh pr view 65 --repo yourconscience/dotagents --json number,title,state,baseRefName,headRefName,mergeStateStatus,url,statusCheckRollup,reviewDecision`.
   - Run the same `gh pr view` for PR #64.

2. Resolve why PR #65 is `DIRTY`.
   - Inspect the conflict against `main`.
   - Prefer a clean rebase or merge from current `origin/main` into `public-prep`, depending on what the user asks and repo convention.
   - Preserve the two branch intents: public redactions and commit hook rule.
   - If PR #64 is already merged or changes overlap, avoid duplicating commits unnecessarily.

3. Keep Antigravity support out of scope.
   - Review feedback explicitly said to drop Antigravity support.
   - Do not reintroduce `.antigravitycli/`, `cmd/dotagents/antigravity_test.go`, Antigravity MCP support, Antigravity hooks, or Antigravity role rendering in this public-prep branch.

4. Re-run privacy checks from the captured session.
   - Verify tracked files do not include private paths, private company examples, credentials, or session files.
   - Suggested commands:
     - `git ls-files -z | xargs -0 rg -n "<user-home>|Inworld|korikov|conscience@|100\\.73\\."`
     - `git ls-files 'mcp/*/.env' 'mcp/*/.*.session' tmp/ research/ memsearch.conf .claude/ dotagents.yaml.tmp`
   - Treat `.env.example` files with empty placeholders as acceptable.

5. Run focused verification.
   - Minimum local checks:
     - `go test ./cmd/dotagents -count=1`
     - `go vet ./cmd/dotagents`
     - `git diff --check origin/main...HEAD`
   - If the branch is pushed or already on GitHub, inspect CI/check status before reporting.

6. Prepare the final user-facing handoff.
   - Include PR #65 status and URL.
   - State whether PR #65 is mergeable.
   - State whether tests passed locally.
   - State whether any untracked WIP remains.
   - State whether Telegram credential rotation is still recommended.
   - Mention that `dotvault` was treated only as captured context unless explicitly edited.

## Acceptance Criteria

- `public-prep` is based cleanly on current `origin/main` or the user explicitly accepts a different branch shape.
- PR #65 is no longer `DIRTY`, or the remaining conflict is documented with exact files and recommended resolution.
- Public-prep redactions remain intact.
- Commit hook guidance remains intact or is superseded by a merged PR #64.
- Local verification passes, or failures are documented precisely with exact files and commands.
- No private credentials or local-only sensitive files are added to git.
- `GOAL.md` itself can be used as the next Codex goal prompt without needing to reread the full captured session.

## Useful Commands

```sh
git -C ~/Workspace/dotagents status --short --branch
git -C ~/Workspace/dotagents worktree list
git -C ~/Workspace/dotagents diff --stat origin/main...HEAD
git -C ~/Workspace/dotagents diff --name-status origin/main...HEAD
gh pr view 65 --repo yourconscience/dotagents --json number,title,state,baseRefName,headRefName,mergeStateStatus,url,statusCheckRollup,reviewDecision
gh pr view 64 --repo yourconscience/dotagents --json number,title,state,baseRefName,headRefName,mergeStateStatus,url,statusCheckRollup,reviewDecision
go test ./cmd/dotagents -count=1
go vet ./cmd/dotagents
git diff --check origin/main...HEAD
```
