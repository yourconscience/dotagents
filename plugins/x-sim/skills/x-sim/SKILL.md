---
name: x-sim
description: Offline X audience simulation and evaluation. Use when evaluating draft tweets, handle bios, pinned-post ideas, or promotion angles against real scraped X context without posting or mutating X state.
---

# x-sim

Use `x-sim` to simulate how real X audiences may react to a draft tweet, bio, pinned post, or handle-promotion angle.

This packaged copy mirrors the canonical dotagents skill at `skills/x-sim/SKILL.md`. The CLI lives in the dotagents checkout (`~/.agents`), not in this plugin tree.

Hard boundary: never post, like, reply, follow, DM, or mutate X state. Read X data through `x-cli --json`, store local context in SQLite, and write markdown reports.

Core workflow:

```bash
go run ~/.agents/skills/x-sim/tools/x-sim init
go run ~/.agents/skills/x-sim/tools/x-sim source add-account @handle
go run ~/.agents/skills/x-sim/tools/x-sim source add-search "agent evals"
go run ~/.agents/skills/x-sim/tools/x-sim sync --since 6m --limit-per-source 200
go run ~/.agents/skills/x-sim/tools/x-sim brief --topic "agent evals" --out /tmp/x-sim-brief.md
go run ~/.agents/skills/x-sim/tools/x-sim eval-tweet --text "draft tweet" --topic "agent evals" --out /tmp/x-sim-report.md
go run ~/.agents/skills/x-sim/tools/x-sim eval-handle --bio "..." --promotion "..." --out /tmp/x-sim-handle.md
```

Return a short markdown review with verdict, likely audience reaction, best revision, risks, and evidence from local scraped tweets.
