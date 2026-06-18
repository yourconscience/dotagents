---
name: review
description: Multi-persona code review with structured findings. Use when the user says /review, asks for a code review, wants to review a diff/PR/branch before merging, or needs a pre-submit quality check.
---

# review

Proactive multi-persona code review. Produces structured findings before a PR is submitted or merged.

Not the same as `/pr-triage`, which reacts to existing PR comments and CI failures. Use `/review` when you want fresh review findings on a diff.

## Usage

```
/review                          # review staged + unstaged changes
/review HEAD~3..HEAD             # review last 3 commits
/review main..HEAD               # review current branch vs base
/review --fix                    # review and fix findings (iterative)
/review --persona security       # single-persona review
```

## Phase 1: Scope

Determine the diff to review.

If the user provided a range, use it. Otherwise detect:

```bash
# If there are staged/unstaged changes, review those
git diff HEAD

# If clean working tree, review current branch vs default base
BASE="$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's|refs/remotes/origin/||')"
BASE="${BASE:-main}"
git log --oneline "${BASE}..HEAD"
git diff "${BASE}...HEAD"
```

Identify relevant context files: CONTRIBUTING.md, coding standards, project CLAUDE.md/AGENTS.md, any spec or PRD referenced in recent commit messages.

If the diff is large (>2000 lines), ask the user whether to review everything or focus on specific files/directories.

## Phase 2: Parallel Persona Review

Spawn 2+ reviewer sub-agents in parallel. Each persona writes free-form prose -- do not constrain their output format.

### Default personas

**Standards reviewer:** Does the code follow project conventions? Check naming, error handling patterns, test coverage expectations, import ordering, existing abstractions. Reference CONTRIBUTING.md and project style if available. Ignore style issues that a linter would catch.

**Correctness reviewer:** Are there bugs, edge cases, race conditions, or logic errors? Does the code handle failures correctly? Are there security issues (injection, auth bypass, secret leaks)? Does it do what the commit messages claim?

### Optional personas (user-requested or auto-detected)

**Security reviewer:** Activated when the diff touches auth, crypto, user input handling, or API endpoints. Focus on OWASP top 10, secret leaks, permission checks.

**Performance reviewer:** Activated when the diff touches hot paths, database queries, or algorithms. Focus on complexity, unnecessary allocations, N+1 queries.

Each persona prompt should include:
- The full diff
- Relevant context files (standards, spec)
- Instruction to write freely, be specific (file:line), and focus on what matters

## Phase 3: Normalize Findings

After all persona reviews return, extract structured findings from each persona's prose. Use a separate, cheaper model call for this extraction when possible.

For each finding, extract:

```
File: <path>
Lines: <start>-<end>
Severity: critical | high | medium | low
Confidence: <0.0-1.0>
Problem: <one paragraph -- what is wrong and why it matters>
Fix: <one paragraph -- concrete fix, not "add tests" or "consider refactoring">
```

Normalization rules:
- Do not invent findings that the reviewer did not mention
- Do not fabricate line numbers -- if the reviewer was vague, mark lines as "unknown"
- Vague fix text ("add tests", "consider refactoring", "improve error handling") must be expanded to a concrete action or the finding is downgraded to low severity
- If a reviewer flagged the same issue multiple times, deduplicate

## Phase 4: Aggregate

Combine findings from all personas. Deduplicate findings that reference the same file:line range with overlapping descriptions.

Upgrade severity when multiple personas independently flag the same issue. Downgrade when only one persona flagged it and confidence is below 0.5.

### Output format

```
## Review: <range or description>

### Must Fix (<count>)
Critical severity. Each with file:line, problem, fix.

### Major Issues (<count>)
High severity. Each with file:line, problem, fix.

### Review Carefully (<count>)
Medium severity. Collapsed unless user asks for detail.

### Minor (<count>)
Low severity. One-line summaries only.

### Summary
- Total findings: <N> (<critical> critical, <high> high, <medium> medium, <low> low)
- Personas used: <list>
- Files reviewed: <count>
```

## Phase 5: Fix Loop (only with --fix)

Only runs when the user passes `--fix` or explicitly asks to fix findings.

For each finding rated critical or high with a concrete fix:

1. Apply the fix (use the builder role if available, otherwise fix directly)
2. Run tests or build if available to catch regressions before re-review
3. Re-run the persona that originally flagged the finding on the changed file
3. If the reviewer confirms the fix, mark as resolved
4. If the reviewer finds a new issue with the fix, iterate (max 3 rounds per finding)

After all fixes, re-run all personas on the final diff to catch issues introduced by fixes. The final aggregate uses only fresh findings from this re-run, not stale output from the original review.

## Rules

- Review is read-only by default. Only modify files in `--fix` mode.
- Do not post review comments to GitHub. Output findings to the conversation. The user decides what to do with them.
- When spawning persona sub-agents, use the `reviewer` role if available. Sub-agents inherit the parent model unless the user specifies otherwise.
- If the harness does not support sub-agents, run personas sequentially instead of in parallel.
- Be specific. Every finding must reference a file and ideally a line range. Findings without file references are noise.
- Do not nitpick formatting, whitespace, or style issues that a linter handles. Focus on correctness, security, and spec compliance.
- Large diffs (>5000 lines): warn the user that review quality degrades with size. Suggest splitting by directory or concern.
