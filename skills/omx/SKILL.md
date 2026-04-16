---
name: omx
description: Delegate a substantial coding, review, or research task to Codex (GPT-5) via oh-my-codex. Codex always runs in a detached omx-* tmux session; use hidden mode (default) for background delegation or interactive mode for a visible cmux surface the user can watch. Use when the user says /omx, asks to "delegate to Codex", "offload to GPT-5", or wants a long-running codex task running alongside Claude.
---

# omx

Hand off a well-defined task to Codex running alongside Claude. Codex always runs inside a detached `omx-*` tmux session regardless of mode. The only question is whether to attach a visible viewer.

## When to use

- Substantial task worth GPT-5's longer reasoning or a second-model perspective: refactor, review pass, multi-file change, long-running test/debug loop, deep-research report.
- Bad fits: tiny edits, sub-2-minute tasks, work tightly coupled to Claude's current context.

## Mental model

- **Source of truth**: the `omx-*` tmux session spawned by every `omx` launch.
  `omx` sees `$TMUX` (Claude runs inside the outer `cc` tmux session) and puts codex in its own detached sub-session. Claude's Bash tool returns immediately; codex keeps running there until done.
- **Viewer (optional)**: a cmux surface tab that runs `tmux attach -t $OMX_TMUX` so the user can watch. Interactive mode adds this; hidden mode skips it.

All interaction — send-keys, capture, close — targets the tmux session, never the viewer surface.

## Modes

| Mode | Visible codex UI? | Use when |
|---|---|---|
| Hidden (default) | No — Claude polls HUD + transcript + git | Background delegation, user doing other work, task is straightforward |
| Interactive | Yes — sibling cmux surface runs `tmux attach` | User wants to watch, intervene, or the task is fragile |

Default to hidden unless the user explicitly asks to see codex.

**Interactive mode requires cmux.** In other environments (Claude Desktop, bare terminal, Ghostty, Warp), use hidden mode and have the user `tmux attach -t $OMX_TMUX` from their own terminal when they want to watch.

## Write the prompt

Same for both modes. Self-contained brief, no hidden context:

```bash
PROMPT_FILE=$(mktemp /tmp/codex-prompt.XXXXXX.md)
cat > "$PROMPT_FILE" <<'EOF'
<goal, constraints, and exactly what "done" looks like>
EOF
```

Codex commits inherit git identity from `~/.zshrc` (`GIT_AUTHOR_*` / `GIT_COMMITTER_*` are exported globally — same identity used by `cc`, `cx`, `oc`). No per-launch wiring.

## Launch

### Hidden mode (default)

```bash
cd <repo> && omx --yolo --high "$(cat $PROMPT_FILE)"
OMX_TMUX=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
echo "codex running detached in $OMX_TMUX"
```

Claude's Bash tool runs `omx`, omx detaches codex to its own tmux session, Bash returns. Hold `$OMX_TMUX` in conversation context — that's the session handle for everything below.

### Interactive mode

Same launch, plus a sibling cmux surface that attaches the detached session:

```bash
# 1. launch (same as hidden)
cd <repo> && omx --yolo --high "$(cat $PROMPT_FILE)"
OMX_TMUX=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')

# 2. spawn a sibling surface in Claude's pane
CLAUDE_PANE=$(cmux tree | awk -v s="${CMUX_SURFACE_ID}" '
  /pane pane:/ {match($0,/pane:[0-9]+/); p=substr($0,RSTART,RLENGTH)}
  $0 ~ s {print p; exit}')
CODEX_SURFACE=$(cmux new-surface --pane "$CLAUDE_PANE" --type terminal 2>&1 | awk '{print $2}')
CODEX_WS=$(cmux tree | awk -v s="$CODEX_SURFACE" '
  /workspace workspace:/ {match($0,/workspace:[0-9]+/); w=substr($0,RSTART,RLENGTH)}
  $0 ~ s {print w; exit}')

# 3. attach the detached session inside the new surface
cmux send --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" "tmux attach -t $OMX_TMUX"
cmux send-key --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" Enter
```

The surface is now a live viewer. Closing it detaches but does NOT stop codex; the `omx-*` session keeps running until codex finishes or is explicitly killed.

## Poll progress

Three complementary views — use whichever Claude needs. Don't poll more than every 60-120s (HUD JSON is ~10-40 KB per read).

```bash
# structured state
omx hud --json | jq '{turns: .hudNotify.turn_count, last: .metrics.last_activity, out: (.hudNotify.last_agent_output // "" | .[0:200])}'

# live transcript from the detached session
tmux capture-pane -t "$OMX_TMUX" -p 2>&1 | tail -40

# filesystem ground truth
git -C <repo> status -sb && git -C <repo> diff --stat
```

For long-running tasks use Claude's `run_in_background` Bash with a `sleep 90` loop rather than repeated foreground polls.

### Stuck vs. idle post-task

Don't assume codex is stuck when output stops. Check all three columns:

| State | HUD | Transcript tail | Action |
|---|---|---|---|
| Working | `turn_count` increasing, `last_activity` within 60s | spinner, streamed text, `• Working (Nm Ns)` | wait |
| Idle post-task | `turn_count` stable, `last_activity` minutes old | `Stop hook (completed)` recent, `› ...` placeholder, `gpt-5.4 high · N% left` footer | task is done, read transcript, close or resume |
| Stuck | `turn_count` stable, `last_activity` minutes old | spinner never advances, half-printed `• Ran ...`, or waiting on y/n prompt | answer the prompt via `tmux send-keys`, or ^C twice and retry |

The `› Summarize recent commits` line is codex's idle-prompt placeholder, NOT a running task. If you see it, codex is done.

## Resume (send another turn)

If `$OMX_TMUX` is still listed and codex is idle post-task:

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

If `$OMX_TMUX` is gone, relaunch with `omx resume "$(cat $FOLLOWUP)"` to pick a prior session id — same launch pattern as above, new `$OMX_TMUX` handle.

## Close

```bash
# 1. quit codex in the detached session (this ends the omx-* tmux session)
tmux send-keys -t "$OMX_TMUX" C-c
tmux send-keys -t "$OMX_TMUX" C-c

# 2. interactive mode only: close the viewer surface
[ -n "${CODEX_SURFACE:-}" ] && cmux close-surface --workspace "$CODEX_WS" --surface "$CODEX_SURFACE"
```

`^C` to the viewer surface does NOT reach codex — it just detaches the viewer. Always target `$OMX_TMUX` directly.

## Caveats

- **Never use `cmux omx`**, always plain `omx`. The `cmux omx` wrapper adds an extra tmux compat shim that makes HUD auto-attach leak into Claude's pane (shrinking it) and breaks `omx team` workers (shim missing `show-options`). Plain `omx` uses the real outer tmux cleanly.
- **`omx --madmax` is actually dangerous** — bypasses both approvals AND the sandbox. Never use on prompts whose contents Claude has not fully vetted, and never in the user's real home directory without explicit opt-in.
- **First run touches `~/.codex/`**: `omx setup` overwrites `~/.codex/config.toml`. Back up any hand-tuned codex config before running it.
- **Default to `--high` reasoning**. Escalate to `--xhigh` for ambiguous design/debug work. `omx --help` lists the rest of the flags.
