# GitHub MCP wrapper

Dotagents uses this wrapper for the managed `github` MCP entry in `dotagents.yaml`.

Prerequisites:

- `github-mcp-server` on `PATH`
- `GITHUB_PERSONAL_ACCESS_TOKEN` in the environment used by the target agent

The wrapper intentionally exposes narrow toolsets:

```bash
github-mcp-server stdio --toolsets repos,pull_requests,actions
```

Do not put tokens in this repo. Keep secrets in local environment or the agent's local config.
