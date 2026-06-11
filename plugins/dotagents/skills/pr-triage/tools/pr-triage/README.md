# pr-triage tool

Deterministic CLI tool for the `pr-triage` skill and hook runtime.

Agent workflow should prefer the configured GitHub MCP server when MCP tools are available. This tool uses `gh` as a local CLI backend because shell hooks cannot call host in-process MCP tools directly.

```bash
go run ~/.agents/skills/pr-triage/tools/pr-triage inspect --format markdown
go run ~/.agents/skills/pr-triage/tools/pr-triage inspect --format json
go run ~/.agents/skills/pr-triage/tools/pr-triage hook stop
```

The Stop hook is read-only. It may block on merge conflicts, failed checks, unresolved human comments, or high-severity bot threads; it does not push, merge, edit PR bodies, or resolve threads.
