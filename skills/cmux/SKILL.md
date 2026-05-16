---
name: cmux
description: Control cmux workspaces, panes, terminal and browser surfaces, markdown viewers, notifications, and visible agent workspaces.
---

# cmux

Use this skill when the current terminal is managed by cmux, or when the task needs cmux browser surfaces, workspace layout, markdown viewing, or cmux-specific agent launchers.

## Detection

```bash
env | grep '^CMUX_'
cmux current-workspace
cmux sidebar-state
```

If `CMUX_WORKSPACE_ID` and `CMUX_SURFACE_ID` are present, route pane and browser operations through `cmux`.

## Inspect Layout

```bash
cmux current-workspace
cmux tree --workspace workspace:N
cmux tree --all
cmux list-panes --workspace workspace:N
cmux list-pane-surfaces --workspace workspace:N --pane pane:N
cmux sidebar-state --workspace workspace:N
```

Always inspect layout first, then use explicit `workspace:N`, `pane:N`, and `surface:N` targets.

## Read Screen

```bash
cmux read-screen --workspace workspace:N --surface surface:N --lines 80
cmux read-screen --workspace workspace:N --surface surface:N --scrollback --lines 200
```

## Send Input

```bash
cmux send --workspace workspace:N --surface surface:N "command here"
cmux send-key --workspace workspace:N --surface surface:N "Enter"
cmux send-key --workspace workspace:N --surface surface:N "C-c"
```

Prefer sending full commands plus `Enter`; use keys for control sequences.

## Workspaces, Panes, and Surfaces

```bash
cmux new-workspace --name "title" --cwd /path
cmux select-workspace --workspace workspace:N
cmux rename-workspace --workspace workspace:N "title"
cmux new-pane --type terminal --direction right --workspace workspace:N
cmux new-pane --type browser --direction right --workspace workspace:N --url https://example.com
cmux new-split right --workspace workspace:N --surface surface:N
cmux new-surface --type terminal --pane pane:N --workspace workspace:N
cmux new-surface --type browser --pane pane:N --workspace workspace:N --url https://example.com
```

In cmux, panes hold surfaces. A pane can contain terminal and browser surfaces as tabs.

## Focus, Move, and Close

```bash
cmux focus-pane --pane pane:N --workspace workspace:N
cmux move-surface --surface surface:N --pane pane:N --workspace workspace:N
cmux move-tab-to-new-workspace --surface surface:N --workspace workspace:N --title "title" --focus false
cmux split-off --surface surface:N right --workspace workspace:N --focus false
cmux close-surface --surface surface:N --workspace workspace:N
cmux close-workspace --workspace workspace:N
```

To avoid recursive screen sharing during a video call, move the meeting browser surface into its own workspace instead of closing it:

```bash
cmux tree --workspace workspace:N
cmux move-tab-to-new-workspace --surface surface:N --workspace workspace:N --title meeting --focus false
```

## Browser Surfaces

```bash
cmux browser --surface surface:N url
cmux browser --surface surface:N snapshot --compact
cmux browser --surface surface:N navigate https://example.com --snapshot-after
cmux browser --surface surface:N click "button"
cmux browser --surface surface:N type "input[name=q]" "text"
cmux browser --surface surface:N screenshot --out /tmp/cmux-browser.png
```

Prefer browser subcommands for cmux browser surfaces. Do not use them to click call controls, close tabs, or submit forms unless the user has asked for that specific action.

## Markdown and Notifications

```bash
cmux markdown open /path/to/report.md --focus false
cmux notify --title "Done" --body "Details" --workspace workspace:N
cmux list-notifications
```

## Browse Project Files

Use `peek` to browse the current cmux workspace in the cmux browser via a local `code-server` instance:

```bash
peek
peek ~/some-project
```

`peek` is a local machine setup, not a portable cmux command. It must only use a loopback `code-server` URL when authentication is disabled. If it opens the wrong folder, verify `cmux sidebar-state` reports the expected `cwd`; if incorrect, recreate the workspace with the right `--cwd`.

## Agent Workflow

```bash
cmux new-workspace --name "agent task" --cwd /repo
cmux send --workspace workspace:N --surface surface:N "droid"
cmux send-key --workspace workspace:N --surface surface:N "Enter"
```

cmux-specific launchers include `cmux codex-teams`, `cmux claude-teams`, `cmux hermes`, `cmux omx`, and `cmux omo`. Use the normal agent CLI directly when you need exact command control.

## Safety Rules

- Inspect layout before reading, sending input, moving surfaces, or closing anything.
- Use explicit targets (`workspace:N`, `pane:N`, `surface:N`).
- Move browser/video surfaces to a separate workspace instead of closing them to avoid terminating active sessions.
- Do not click browser controls, close surfaces, or send destructive commands unless you have verified the target and the user asked for that action.
- Prefer a dedicated workspace for long-running delegated agents.
