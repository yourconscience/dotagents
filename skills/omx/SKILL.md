---
name: omx
description: Delegate a substantial task to Codex (GPT-5) via oh-my-codex. Works from any agent - Claude Code, Hermes, or standalone terminal. Use when the user says /omx, asks to "delegate to Codex", "offload to GPT-5", or wants a cross-model perspective.
---

# omx

Hand off a well-defined task to Codex (GPT-5) via oh-my-codex. Works from any agent that can run shell commands.

## When to use

- Substantial task worth GPT-5's reasoning or a second-model perspective: refactor, review, multi-file change, deep research.
- Bad fits: tiny edits, sub-2-minute tasks, work tightly coupled to the caller's current context.

## Mental model

`omx exec` runs Codex non-interactively with AGENTS.md overlay and MCP servers. It blocks until Codex finishes, then returns. This is the preferred mode for agent-to-agent delegation.

`omx` (interactive) spawns Codex in a detached `omx-*` tmux session. Use when the user wants to watch or intervene. Warp is the stable primary terminal surface; cmux is supported for visible panes/workspaces when it is actually available.

## Write the prompt

Self-contained brief. The callee has no access to your conversation context.

```bash
PROMPT_FILE=$(mktemp /tmp/codex-prompt.XXXXXX.md)
cat > "$PROMPT_FILE" <<'EOF'
<goal, constraints, and exactly what "done" looks like>
EOF
```

## Non-interactive mode (omx exec) - preferred

```bash
cd <repo> && omx exec "$(cat $PROMPT_FILE)"
```

Blocks until done. No tmux session management. Works from any environment.

**From Claude Code:** Use the Bash tool directly. `omx exec` does not need the cmux tmux shim workaround.

**From Hermes:** Use the terminal tool to run the same command. No special setup needed.

**From standalone terminal:** Run directly.

## Interactive mode

Spawns Codex in a detached tmux session. The user can attach to watch from Warp or any regular terminal. For visible agent workspaces, use a dedicated cmux workspace/surface when `cmux` is available; otherwise stay with Warp or regular terminal instructions.

```bash
cd <repo> && omx --yolo --high "$(cat $PROMPT_FILE)"
OMX_TMUX=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
```

### cmux compatibility (Claude Code cmux shim contexts only)

cmux shims `tmux` at `~/.cmuxterm/claude-teams-bin/tmux`. This shim rejects `tmux show-options` which omx calls on interactive startup. Workarounds:

- `omx exec` works without any fix.
- Interactive `omx`: prepend `PATH="/opt/homebrew/bin:$PATH"` to bypass the shim.
- Never use `cmux omx` - it adds a second shim layer that breaks HUD auto-attach.

### Adding a cmux viewer surface (when cmux is available)

```bash
CLAUDE_PANE=$(cmux tree | awk -v s="${CMUX_SURFACE_ID}" '
  /pane pane:/ {match($0,/pane:[0-9]+/); p=substr($0,RSTART,RLENGTH)}
  $0 ~ s {print p; exit}')
CODEX_SURFACE=$(cmux new-surface --pane "$CLAUDE_PANE" --type terminal 2>&1 | awk '{print $2}')
CODEX_WS=$(cmux tree | awk -v s="$CODEX_SURFACE" '
  /workspace workspace:/ {match($0,/workspace:[0-9]+/); w=substr($0,RSTART,RLENGTH)}
  $0 ~ s {print w; exit}')
cmux send --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" "tmux attach -t $OMX_TMUX"
cmux send-key --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" Enter
```

## Poll progress (interactive mode)

Three complementary views. Don't poll more than every 60-120s.

```bash
# structured state
omx hud --json | jq '{turns: .hudNotify.turn_count, last: .metrics.last_activity, out: (.hudNotify.last_agent_output // "" | .[0:200])}'

# live transcript
tmux capture-pane -t "$OMX_TMUX" -p 2>&1 | tail -40

# filesystem ground truth
git -C <repo> status -sb && git -C <repo> diff --stat
```

### Detecting completion

| State | HUD | Transcript tail | Action |
|---|---|---|---|
| Working | `turn_count` increasing, `last_activity` < 60s | spinner, streamed text | wait |
| Done | `turn_count` stable, `last_activity` minutes old | `Stop hook (completed)`, idle prompt | read results, close |
| Stuck | `turn_count` stable, `last_activity` minutes old | spinner frozen or y/n prompt | answer prompt or ^C twice |

## Resume

If the tmux session is still alive and codex is idle:

```bash
FOLLOWUP=$(mktemp /tmp/codex-followup.XXXXXX.md)
cat > "$FOLLOWUP" <<'EOF'
<new instructions, self-contained>
EOF
tmux load-buffer -b omxfu "$FOLLOWUP"
tmux paste-buffer -b omxfu -t "$OMX_TMUX"
tmux send-keys -t "$OMX_TMUX" Enter
tmux delete-buffer -b omxfu
```

## Close and cleanup

```bash
# quit codex
tmux send-keys -t "$OMX_TMUX" C-c
tmux send-keys -t "$OMX_TMUX" C-c

# cleanup orphaned MCP processes and stale /tmp dirs
omx cleanup 2>/dev/null
```

Always run `omx cleanup` after closing a session.

## Flags reference

- `--high` (default recommended): standard reasoning depth
- `--xhigh`: deeper reasoning for ambiguous design/debug
- `--yolo`: skip approval prompts
- `--madmax`: bypasses sandbox - dangerous, never use on unvetted prompts
