# Knowledge sync tool placement and inspection

The dotagents repo owns the agent memory and knowledge-vault sync pipeline. If a standalone `knowledge-sync` helper exists, treat it as part of the `memory/` subsystem rather than a generic workspace tool.

## Known local pattern

- Active executable: `~/.local/bin/knowledge-sync`
- LaunchAgent: `~/Library/LaunchAgents/ai.knowledge-sync.plist`
- Typical interval: 1800 seconds (30 minutes) for the local LaunchAgent
- Vault: `~/Workspace/knowledge`
- Lock: `~/Workspace/knowledge/.git/knowledge-sync.lock`
- Preferred source location: `~/Workspace/dotagents/memory/tools/knowledge-sync/`

The LaunchAgent should point to the stable binary under `~/.local/bin`, not directly to source files.

## Safe inspection before moving source

1. Inventory the candidate source directory:
   - contents, size, symlinks, git status, nested repos.
2. Search for references before moving:
   - LaunchAgents, crontabs, shell rc files, agent configs, repo scripts, active processes, and open files.
3. Check the active binary:
   - `file ~/.local/bin/knowledge-sync`
   - `go version -m ~/.local/bin/knowledge-sync`
   - optionally `strings ~/.local/bin/knowledge-sync | grep Workspace/tools`
4. Check LaunchAgent state and logs:
   - `launchctl print gui/$(id -u)/ai.knowledge-sync`
   - recent stdout/stderr under the knowledge repo git logs if configured there.
5. Confirm the source path is not directly executed. If only the compiled binary is used, moving source is low risk after rebuild.

## Migration pattern

1. Move source to `memory/tools/knowledge-sync/` in `~/Workspace/dotagents`.
2. Rebuild the stable binary from the new source:
   - `cd ~/Workspace/dotagents/memory/tools/knowledge-sync`
   - `GOWORK=off go build -o ~/.local/bin/knowledge-sync .`
3. Run one manual sync.
4. Verify LaunchAgent still loads and logs show successful sync.
5. Remove the old empty source parent only after verification.

## Pitfalls

- Do not put tracked tool source under `memory/local/`; that directory is for ignored local runtime/generated state.
- Do not modify or unload the LaunchAgent unless the task explicitly requires changing scheduling.
- Old `git fetch` errors in logs may be environmental. Prefer current stdout/stderr timestamps and a manual run before treating them as migration blockers.