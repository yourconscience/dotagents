# Setup walkthrough

## What `dotagents setup` does

1. Creates `~/.agents` if it does not exist and copies in the starter content (two skills, six roles, memory hooks, a minimal `dotagents.yaml`).
2. Detects which harnesses are installed and records their native paths.
3. Scans each harness for skills, roles, and MCP servers you already have and shows a review screen: one row per item, share/keep/skip per row, items identical across harnesses shared automatically. Copy-only, originals untouched. Non-interactive runs fall back to sequential prompts; `--yes` imports everything without prompting, `--dry-run` prints the candidates and exits without changes, `--json` emits the detection result for scripting and exits.
4. Before its first sync touches a harness that already has content, shows exactly what would be removed or overwritten there and asks per harness. Declining keeps that harness's files.
5. Offers to `git init` the new repository, and runs the first sync.

The review screen in step 3 looks like this — `space` cycles share/keep/skip per row, `enter` applies:

```
dotagents setup — review 3 item(s)  (2 identical, shared automatically)

  skill   grilling   claude-code✓ codex✓ droid·   [share]
  skill   my-notes   claude-code✓ codex· droid✓   [share] (differ) from claude-code
  role    reviewer   claude-code✓ codex✓ droid✓   [skip]

↑↓ move  space cycle action  ←→ pick source  a share-all  s skip-all  enter apply  q abort
```

## Multi-machine

```bash
cd ~/.agents
git remote add origin <your-private-repository>
git push -u origin main
# on the next machine: clone it to ~/.agents, install dotagents, run dotagents setup
```

Subsequent syncs: `dotagents sync --pull` pulls the repo first, then reconciles. Machines without a Go toolchain skip the memory-tools build step; everything else syncs normally.

## Memory tier

Choose during setup or reconfigure later:

```bash
dotagents setup --memory basic      # default
dotagents setup --memory off
dotagents setup --memory memsearch
```

| Tier | Behavior | Dependency |
|---|---|---|
| `off` | no managed memory hooks | none |
| `basic` | appends a bounded digest of each session to `$KNOWLEDGE_DIR/sessions/YYYY-MM-DD.md` and injects recent digests as context at session start | Python 3 |
| `memsearch` | full-text indexed search over the knowledge vault | `memsearch` on PATH (`uv tool install memsearch`) |

## Root instructions

`~/.agents/AGENTS.md` is your single root instruction file. During sync, dotagents links it into each harness's native memory path — `~/.claude/CLAUDE.md` for Claude Code, `~/.codex/AGENTS.md` for Codex, `~/.factory/AGENTS.md` for Droid — so an edit in one place reaches every agent. `dotagents status` reports drift, and a file that exists but is not a symlink is never touched without your confirmation.
