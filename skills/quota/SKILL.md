---
name: quota
description: Track remaining quota, reset windows, and historical usage for Claude, Codex, and similar coding agents using provider-specific CLIs through a provider-agnostic workflow.
version: 1.0.0
author: Hermes Agent
license: MIT
---

# Quota

Use this when the user asks for remaining quota, usage limits, reset windows, credits, or historical usage for coding agents such as Claude Code, Codex, Gemini CLI, Kiro, Copilot, or similar tools.

## Principle
Use a provider-agnostic workflow first. Pick the best local CLI or log-based tool for the provider instead of hardcoding one vendor-specific path.

Priority order:
1. Current quota / reset windows from the provider's own CLI or a purpose-built quota CLI.
2. Historical usage from local session logs.
3. Heavier dashboards only if the simple CLI path is insufficient.

Prefer lightweight CLI tools such as `codexbar`, `ccusage`, related `@ccusage/*` packages, or provider-native `/status` and `/usage` commands.

## What to answer
Separate these two questions clearly:
- **Remaining quota now** - current session window, weekly window, monthly credits, next reset.
- **Historical usage** - daily, monthly, session, and cost analytics.

Do not confuse historical token analysis with current remaining quota.

## Standard workflow

### 1. Detect what operator/provider the user means
Examples:
- Codex
- Claude Code
- Gemini CLI
- Kiro
- Copilot
- unknown or mixed provider setup

If ambiguous but obvious from local installs, inspect the machine before asking.

### 2. Check local tools first
Run checks like:
```bash
command -v codex || true
command -v claude || true
command -v gemini || true
command -v kiro || true
command -v codexbar || true
command -v ccusage || true
command -v node || true
command -v npm || true
command -v npx || true
```

### 3. Check auth / live state for the target provider
Examples:
```bash
codex login status
claude --version
codex --version
```
If the provider has a native status command, prefer that before installing anything.

### 4. Choose the least heavy working path
- Need **remaining quota now**: prefer quota-aware CLI.
- Need **historical usage/cost**: prefer local log analyzer.
- Need both: run both and label each result.

## Tool selection by provider

### Codex
**Primary for remaining quota now:** CodexBar
```bash
codexbar --provider codex --source cli --format json --pretty
```
Expected useful fields:
- 5h/session usage
- weekly usage
- reset timestamps
- credits

**Secondary for historical usage:**
```bash
npx @ccusage/codex@latest
```
Use for:
- daily/monthly/session analytics
- token and cost summaries

Notes:
- On Linux, prefer `codexbar --source cli`.
- Do not claim Codex CLI itself exposes reliable remaining quota unless verified live.

### Claude Code
**Primary:** try the simplest native path first.
- In interactive Claude Code, `/status` and `/usage` may expose the most direct limit information.
- If a quota-aware cross-provider CLI is already installed, use it.

**Historical usage:**
```bash
npx ccusage@latest
```
Use for:
- daily/monthly/session analytics
- block or window summaries if supported
- cost and token analysis from local logs

If CodexBar supports Claude on the current machine and is installed, it can also be used for current windows.

### Other providers
Use the same policy:
1. native provider status or usage command
2. quota-aware cross-provider CLI if available
3. local log analyzer
4. heavier dashboard only if necessary

Examples of possible tools:
- `codexbar`
- `ccusage`
- `@ccusage/codex`
- `agentsview`
- `splitrail`

Prefer the smallest tool that directly answers the user's question.

## Installation policy
Before installing, confirm the tool is actually needed and supported on the current OS.

### CodexBar
Good default when the question is current remaining quota for Codex, and sometimes Claude or other providers if supported.

Linux install approach:
1. Detect architecture.
2. Download matching GitHub release tarball.
3. Extract binary.
4. Put `codexbar` on PATH.
5. Verify with `codexbar --version`.

### ccusage family
Good default when the question is historical usage and the provider stores local logs.

Examples:
```bash
npx ccusage@latest
npx @ccusage/codex@latest
```

## Reporting format
When answering, split output into:

### Remaining quota now
- provider
- source tool
- session/5h remaining
- weekly remaining
- credits remaining
- reset times

### Historical usage
- source tool
- date range
- key totals
- caveats about missing local logs

## Pitfalls
- Do not present historical token usage as current remaining quota.
- Do not assume local session logs exist.
- Do not overfit the skill to Codex only.
- Do not install a heavy dashboard when a simple CLI already answers the question.
- Do not claim cross-provider support without checking the current tool version and local environment.
- On Linux, prefer CLI paths over browser-cookie or GUI-only paths.

## Recommendation policy
For future skills, prefer provider-agnostic workflows where practical:
- detect provider/tooling first
- choose the simplest local tool that works
- separate current quota from historical usage
- avoid hardcoding one vendor if the task generalizes cleanly
