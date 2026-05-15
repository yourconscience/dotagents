---
name: tmux
description: Generic tmux reference for sessions, windows, panes, screen capture, and input. If CMUX_* environment variables are present, use the cmux skill instead.
---

# tmux

Use this skill for terminal multiplexing when the current terminal is not managed by cmux.

## Detection

```bash
env | grep '^CMUX_'         # if present, switch to the cmux skill
test -n "$TMUX" && echo tmux
command -v tmux
```

Routing:
- If `CMUX_WORKSPACE_ID` and `CMUX_SURFACE_ID` are set, use the `cmux` skill.
- Else if `TMUX` is set, target the current tmux server/session.
- Else if `tmux` exists, create or attach a tmux session before relying on pane operations.
- Else, no supported queryable pane manager is active.

## Inspect Layout

```bash
tmux list-sessions
tmux list-windows -a -F '#{session_name}:#{window_index} #{window_name} active=#{window_active}'
tmux list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_id} active=#{pane_active} cwd=#{pane_current_path} cmd=#{pane_current_command} title=#{pane_title}'
tmux display-message -p '#{session_name}:#{window_index}.#{pane_index} #{pane_id}'
```

Always inspect layout first, then use explicit targets. Prefer `%pane_id`; it remains stable if windows are rearranged.

## Read Screen

```bash
tmux capture-pane -p -t %pane_id -S -80
tmux capture-pane -p -t %pane_id -S -200
tmux capture-pane -p -t session:window.pane -S -80
```

## Send Input

```bash
tmux send-keys -t %pane_id "command here" Enter
tmux send-keys -t %pane_id Enter
tmux send-keys -t %pane_id C-c
```

Prefer sending full commands plus `Enter`; use keys for control sequences.

## Create Sessions, Windows, and Panes

```bash
tmux new-session -d -s name -c /path
tmux attach-session -t name
tmux new-window -t name -n title -c /path
tmux split-window -h -t %pane_id -c /path
tmux split-window -v -t %pane_id -c /path
```

## Focus and Close

```bash
tmux switch-client -t session
tmux select-window -t session:window
tmux select-pane -t %pane_id
tmux kill-pane -t %pane_id
tmux kill-window -t session:window
tmux kill-session -t session
```

## Move Panes and Windows

```bash
tmux move-window -s source:window -t target:window
tmux move-pane -s %source_pane -t %target_pane
tmux join-pane -s %source_pane -t %target_pane
tmux break-pane -s %pane_id
```

## Buffers and Signals

```bash
tmux set-buffer -b scratch "text"
tmux paste-buffer -b scratch -t %pane_id
tmux wait-for build-done
tmux wait-for -S build-done
```

## Agent Workflow

```bash
tmux new-session -d -s agent-task -c /repo
tmux send-keys -t agent-task:0.0 "droid" Enter
tmux attach-session -t agent-task
```

Run normal agent CLIs in tmux panes (`droid`, `hermes`, `codex`).

## Safety Rules

- Inspect layout before reading or sending input.
- Use explicit targets (`%pane_id` or `session:window.pane`).
- Do not send destructive commands to a pane unless you have verified the target.
- Prefer a dedicated session for long-running delegated agents.
