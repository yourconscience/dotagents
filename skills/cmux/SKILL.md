---
name: cmux
description: Reference for cmux CLI subcommands. Workspace management, screen reading, input sending, browser automation, markdown viewer, and agent teams.
---

# cmux

Use when you need to interact with cmux workspaces, read terminal output from other panes, send input to other agents, use the built-in browser, view markdown files, or manage agent teams.

## Environment

cmux sets these env vars in terminals it manages:
- `CMUX_WORKSPACE_ID` - current workspace UUID
- `CMUX_SURFACE_ID` - current surface UUID
- `CMUX_TAB_ID` - current tab (optional)
- `CMUX_SOCKET_PATH` - override Unix socket path (auto-discovered by default)

Commands default to the caller's workspace/surface when these are set.

## Handle Inputs

Use UUIDs, short refs (`window:1`, `workspace:2`, `pane:3`, `surface:4`), or indexes where commands accept window, workspace, pane, or surface inputs.

## Workspace Management

```bash
cmux list-workspaces                                  # list all workspaces with refs
cmux current-workspace                                # show current workspace
cmux select-workspace --workspace workspace:N         # switch to workspace
cmux tree --workspace workspace:N                     # show pane/surface tree
cmux tree --all                                       # show all workspaces
cmux new-workspace --name "title" --cwd /path         # create workspace
cmux close-workspace --workspace workspace:N          # close workspace
cmux rename-workspace --workspace workspace:N "title" # rename workspace
```

## Window Management

```bash
cmux list-windows
cmux current-window
cmux new-window
cmux focus-window --window window:N
cmux close-window --window window:N
cmux move-workspace-to-window --workspace workspace:N --window window:N
```

## Reading Terminal Output

```bash
# read current screen content
cmux read-screen --workspace workspace:N --surface surface:N --lines 80

# include scrollback buffer
cmux read-screen --workspace workspace:N --surface surface:N --scrollback --lines 200

# tmux alias
cmux capture-pane --workspace workspace:N --surface surface:N --scrollback --lines 200
```

This is the primary way to monitor what other agents are doing.

## Sending Input

```bash
# send text (as if typed) - appends newline
cmux send --workspace workspace:N --surface surface:N "command here"

# send a key
cmux send-key --workspace workspace:N --surface surface:N "Enter"
cmux send-key --workspace workspace:N --surface surface:N "C-c"
```

## Pane and Surface Management

Panes hold tabs. Surfaces are individual tabs within a pane.

```bash
cmux list-panes --workspace workspace:N
cmux list-pane-surfaces --workspace workspace:N --pane pane:N
cmux list-panels --workspace workspace:N

# split: creates a new pane
cmux new-split right --workspace workspace:N --surface surface:N
cmux new-pane --type terminal --direction right --workspace workspace:N
cmux new-pane --type browser --direction down --workspace workspace:N --url "https://..."

# tab: adds surface to existing pane (no split)
cmux new-surface --type terminal --pane pane:N --workspace workspace:N
cmux new-surface --type browser --pane pane:N --workspace workspace:N --url "https://..."

cmux focus-pane --pane pane:N --workspace workspace:N
cmux focus-panel --panel panel:N --workspace workspace:N
cmux close-surface --surface surface:N --workspace workspace:N
cmux move-surface --surface surface:N --pane pane:N --workspace workspace:N
cmux rename-tab --surface surface:N "title"

# tmux compat
cmux resize-pane --pane pane:N --workspace workspace:N -R --amount 20
cmux swap-pane --pane pane:N --target-pane pane:M --workspace workspace:N
cmux break-pane --workspace workspace:N --surface surface:N    # surface -> own workspace
cmux join-pane --target-pane pane:N --surface surface:N        # merge into target
cmux last-pane --workspace workspace:N
cmux find-window --content --select "query"
```

## Browser Automation

cmux has a built-in browser. Auth state (cookies, sessions) persists within the cmux process lifetime.

### Opening and Navigation
```bash
cmux browser open "https://example.com"                # open URL as new tab
cmux browser open-split "https://example.com"          # open URL in new split pane
cmux browser navigate "https://example.com" --surface surface:N
cmux browser url --surface surface:N                   # get current URL
cmux browser back --surface surface:N
cmux browser forward --surface surface:N
cmux browser reload --surface surface:N
```

### Reading Content
```bash
# compact DOM snapshot (best for finding elements)
cmux browser snapshot --surface surface:N --compact

# with depth limit or CSS selector filter
cmux browser snapshot --surface surface:N --compact --max-depth 6
cmux browser snapshot --surface surface:N --compact --selector "article"

# interactive snapshot (includes [ref=eN] for clicking)
cmux browser snapshot --surface surface:N --interactive

# get specific data
cmux browser get title --surface surface:N
cmux browser get text --selector "article" --surface surface:N
cmux browser get html --selector "main" --surface surface:N
cmux browser get url --surface surface:N

# take a screenshot
cmux browser screenshot --surface surface:N --out /tmp/screenshot.png
```

### Waiting
```bash
cmux browser wait --load-state complete --timeout-ms 10000 --surface surface:N
cmux browser wait --selector ".results" --surface surface:N
cmux browser wait --text "Loading complete" --surface surface:N
cmux browser wait --url-contains "/dashboard" --surface surface:N
```

### Interaction
```bash
cmux browser click "[ref=eN]" --surface surface:N
cmux browser type "[ref=eN]" "text to type" --surface surface:N
cmux browser fill "[ref=eN]" "text" --surface surface:N       # empty text clears input
cmux browser press "Enter" --surface surface:N
cmux browser select "[ref=eN]" "value" --surface surface:N
cmux browser scroll --dy 500 --surface surface:N               # scroll page
cmux browser scroll --selector ".panel" --dy 300 --surface surface:N
```

Note: `[ref=eN]` references come from `browser snapshot --interactive` output.

### Browser Tabs
```bash
cmux browser tab list --surface surface:N
cmux browser tab new --surface surface:N
cmux browser tab switch 2 --surface surface:N
cmux browser tab close --surface surface:N
```

### Advanced
```bash
cmux browser eval "document.title" --surface surface:N
cmux browser cookies get --surface surface:N
cmux browser cookies clear --surface surface:N
cmux browser console list --surface surface:N
cmux browser errors list --surface surface:N
cmux browser frame "iframe-selector" --surface surface:N    # switch to iframe
cmux browser frame main --surface surface:N                 # back to main frame
cmux browser dialog accept --surface surface:N
cmux browser state save /tmp/state.json --surface surface:N
cmux browser state load /tmp/state.json --surface surface:N
cmux browser is visible ".element" --surface surface:N
```

## Markdown Viewer

```bash
# open markdown in a new tab (default)
cmux markdown open plan.md

# open as a split instead
cmux markdown open plan.md --direction right
cmux markdown open plan.md --direction down

# target specific workspace
cmux markdown open docs/design.md --workspace workspace:N
```

The viewer renders formatted markdown with live file watching (auto-reloads on save).

## SSH / Remote

```bash
cmux ssh user@host --name "server"
cmux ssh user@host --port 2222 --identity ~/.ssh/id_ed25519
cmux remote-daemon-status
```

## Agent Launchers

```bash
cmux claude-teams [claude-args...]    # Claude Code with teams enabled
cmux omx [omx-args...]               # launch omx (Codex wrapper)
cmux omo [opencode-args...]           # launch opencode
```

## Notifications and Signals

```bash
cmux notify --title "Task done" --body "Details here" --workspace workspace:N
cmux list-notifications
cmux clear-notifications

# tmux-style signal (for coordination between agents)
cmux wait-for --signal "build-done"
cmux wait-for "build-done" --timeout 30
```

## Identification

```bash
cmux identify                         # show current workspace/surface/pane
cmux identify --workspace workspace:N --surface surface:N
```

## Clipboard Buffers

```bash
cmux set-buffer --name scratch "some text"
cmux list-buffers
cmux paste-buffer --name scratch --surface surface:N
```

## Tips

- Use `cmux tree` first to understand the pane layout before sending input.
- `read-screen` is your main observability tool for monitoring other agents.
- Always specify `--workspace` and `--surface` explicitly when targeting other workspaces.
- `new-split` creates a new pane (visual split); `new-surface` adds a tab to an existing pane.
- `browser open` opens as a tab; `browser open-split` creates a split.
- `markdown open` opens as a tab by default; add `--direction` for a split.
- Use `--snapshot-after` on browser interaction commands to get the DOM state after the action.
