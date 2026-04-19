---
name: spec
description: Create or update a minimal SPEC.md through a short interview, then use it as the source of truth for complex features, refactors, or ambiguous requests. Use when the user asks for /spec, spec-driven development, or when SPEC.md is missing or stale before substantial work.
---

# Spec

## Overview

Run a short interview and produce or update a minimal `SPEC.md` that guides planning and implementation without bloating docs.

## Workflow

1) Decide if a spec is needed.
   - Use this flow for new features, refactors, or ambiguous work.
   - Also use it when `SPEC.md` is missing or stale, or the user asks for `/spec`.

2) Interview (5 questions max).
   - When the harness exposes a structured question tool, use it. Claude Code: `AskUserQuestion`. Codex or Codex-derived harnesses: `request_user_input` when available.
   - Otherwise ask plain text directly.
   - Skip questions already answered in the request or prior context.
   - Aim for concise, unambiguous answers.

Suggested questions:
1. What is the primary goal and user-visible behavior?
2. What is explicitly out of scope?
3. What are the acceptance tests or success criteria?
4. What constraints matter (stack, performance, data, integrations, policies)?
5. What code areas or existing patterns should be reused?

3) Draft `SPEC.md` at the project root (or update it if present).
   - Keep it short. One page if possible.
   - Use the template below and keep sections with content only.

4) Confirm with the user before planning or coding.

5) After implementation, append a short "Outcome / Deviations" section with what changed from the spec.

## Minimal SPEC.md template

```markdown
# SPEC

## Goal

## Non-goals

## User story / behavior

## Acceptance tests

## Constraints

## Dependencies / integrations

## Risks / open questions

## Codebase notes

## Outcome / Deviations
```
