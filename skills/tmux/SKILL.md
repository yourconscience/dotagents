---
name: tmux
description: Generic terminal multiplexer reference. Use cmux when running inside cmux, otherwise tmux for sessions, windows, panes, reading screens, and sending input.
---

# tmux

Use this skill for queryable terminal panes and visible agent workspaces. Prefer **cmux** when the current terminal is managed by cmux; otherwise use **tmux**.

## Detection

```bash
env | grep '^CMUX_'         # cmux managed terminal
test -n "$TMUX" && echo tmux
command -v tmux
```

Routing:
- If `CMUX_WORKSPACE_ID` and `CMUX_SURFACE_ID` are set, use `cmux`.
- Else if `TMUX` is set, target the current tmux server/session.
- Else if `tmux` exists, create or attach a tmux session before relying on pane operations.
- Else, no supported queryable pane manager is active.

## Mental Model

| Generic | cmux | tmux |
| --- | --- | --- |
| Workspace | workspace | session |
| Tab | surface/tab | window |
| Split pane | pane | pane |
| Screen target | surface | pane |

The mapping is close enough for terminal panes, but not exact. In cmux, panes hold surfaces; in tmux, windows hold panes.

## Inspect Layout

cmux:

```bash
cmux current-workspace
cmux tree --workspace workspace:N
cmux tree --all
cmux list-panes --workspace workspace:N
```

tmux:

```bash
tmux list-sessions
tmux list-windows -a -F '#{session_name}:#{window_index} #{window_name} active=#{window_active}'
tmux list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_id} active=#{pane_active} cwd=#{pane_current_path} cmd=#{pane_current_command} title=#{pane_title}'
tmux display-message -p '#{session_name}:#{window_index}.#{pane_index} #{pane_id}'
```

Always inspect layout first, then use explicit targets.

## Read Screen

cmux:

```bash
cmux read-screen --workspace workspace:N --surface surface:N --lines 80
cmux read-screen --workspace workspace:N --surface surface:N --scrollback --lines 200
```

tmux:

```bash
tmux capture-pane -p -t %pane_id -S -80
tmux capture-pane -p -t %pane_id -S -200
tmux capture-pane -p -t session:window.pane -S -80
```

Use `%pane_id` when possible; it remains stable if windows/panes are rearranged.

## Send Input

cmux:

```bash
cmux send --workspace workspace:N --surface surface:N "command here"
cmux send-key --workspace workspace:N --surface surface:N "Enter"
cmux send-key --workspace workspace:N --surface surface:N "C-c"
```

tmux:

```bash
tmux send-keys -t %pane_id "command here" Enter
tmux send-keys -t %pane_id Enter
tmux send-keys -t %pane_id C-c
```

Prefer sending full commands plus `Enter`; use keys for control sequences.

## Create Sessions, Tabs, and Panes

cmux:

```bash
cmux new-workspace --name "title" --cwd /path
cmux new-split right --workspace workspace:N --surface surface:N
cmux new-pane --type terminal --direction right --workspace workspace:N
cmux new-surface --type terminal --pane pane:N --workspace workspace:N
```

tmux:

```bash
tmux new-session -d -s name -c /path
tmux attach-session -t name
tmux new-window -t name -n title -c /path
tmux split-window -h -t %pane_id -c /path
tmux split-window -v -t %pane_id -c /path
```

Direction mapping: cmux `right` ≈ tmux `split-window -h`; cmux `down` ≈ tmux `split-window -v`.

## Focus and Close

cmux:

```bash
cmux select-workspace --workspace workspace:N
cmux focus-pane --pane pane:N --workspace workspace:N
cmux close-surface --surface surface:N --workspace workspace:N
cmux close-workspace --workspace workspace:N
```

tmux:

```bash
tmux switch-client -t session
tmux select-window -t session:window
tmux select-pane -t %pane_id
tmux kill-pane -t %pane_id
tmux kill-window -t session:window
tmux kill-session -t session
```

## Move Panes and Tabs

cmux:

```bash
cmux move-surface --surface surface:N --pane pane:N --workspace workspace:N
```

tmux:

```bash
tmux move-window -s source:window -t target:window
tmux move-pane -s %source_pane -t %target_pane
tmux join-pane -s %source_pane -t %target_pane
tmux break-pane -s %pane_id
```

## Buffers and Signals

cmux:

```bash
cmux set-buffer --name scratch "text"
cmux paste-buffer --name scratch --surface surface:N
cmux wait-for "build-done" --timeout 30
cmux notify --title "Done" --body "Details" --workspace workspace:N
```

tmux:

```bash
tmux set-buffer -b scratch "text"
tmux paste-buffer -b scratch -t %pane_id
tmux wait-for build-done
tmux wait-for -S build-done
```

tmux has no exact `cmux notify` equivalent; use the terminal/app notification system only if explicitly needed.

## Agent Workflow

For visible agent work:

cmux:

```bash
cmux new-workspace --name "agent task" --cwd /repo
cmux send --workspace workspace:N --surface surface:N "droid"
```

tmux:

```bash
tmux new-session -d -s agent-task -c /repo
tmux send-keys -t agent-task:0.0 "droid" Enter
tmux attach-session -t agent-task
```

Run normal agent CLIs in tmux panes (`droid`, `omx`, `hermes`, `codex`). cmux-specific launchers such as `cmux omx` are conveniences, not portable commands.

## Browse Project Files In cmux

Use `peek` to browse the current cmux workspace in the cmux browser via a local `code-server` instance:

```bash
peek
peek ~/some-project
```

`peek` is a local machine setup, not a portable cmux command. It should only use a loopback `code-server` URL when authentication is disabled. If it opens the wrong folder, verify `cmux sidebar-state` reports the expected `cwd` for the active workspace.

## cmux-only Features

These do not have tmux equivalents:

- Browser surfaces and DOM automation: `cmux browser ...`
- Markdown viewer: `cmux markdown open ...`
- cmux notifications: `cmux notify ...`
- cmux agent launchers: `cmux omx`, `cmux hermes`, `cmux claude-teams`

When these are needed and `cmux` is available, use cmux directly.

## Safety Rules

- Inspect layout before reading or sending input.
- Use explicit targets (`workspace:N`/`surface:N` for cmux, `%pane_id` for tmux).
- Do not send destructive commands to a pane unless you have verified the target.
- Prefer a dedicated session/workspace for long-running delegated agents.
