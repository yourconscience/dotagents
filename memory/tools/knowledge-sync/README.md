# knowledge-sync

Small Go helper that keeps the local knowledge vault synchronized with its configured git remote.

Runtime is intentionally separate from this source tree:

- source: `memory/tools/knowledge-sync/`
- installed executable: `~/.local/bin/knowledge-sync`
- user LaunchAgent: `~/Library/LaunchAgents/ai.knowledge-sync.plist`

The LaunchAgent should point at the stable installed executable, not at this source directory. Build and install with:

```bash
GOWORK=off go build -o ~/.local/bin/knowledge-sync .
```

`GOWORK=off` keeps this small standalone module independent from the parent dotagents Go workspace.

Configuration is read from environment variables, with defaults in `main.go`:

- `KNOWLEDGE_REPO`, default `$KNOWLEDGE_DIR` or `~/Workspace/knowledge`
- `KNOWLEDGE_REMOTE`, default `vps`
- `KNOWLEDGE_BRANCH`, default `main`

The helper uses a lock file under the knowledge repo git directory, commits dirty vault changes, fetches/merges from the remote branch, and pushes back to the same remote branch.
