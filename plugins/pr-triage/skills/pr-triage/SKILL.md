---
name: pr-triage
description: Inspect PR failed checks and unresolved review comments, fix valid feedback, push, and safely handle publish-via-PR workflows.
---

# pr-triage

This packaged skill is a thin pointer to the canonical dotagents skill at `~/.agents/skills/pr-triage`.

Use the canonical skill instructions and tool:

```bash
go run ~/.agents/skills/pr-triage/tools/pr-triage inspect --format markdown
```

The plugin package is a distribution layer. Do not edit this copy as source of truth.
