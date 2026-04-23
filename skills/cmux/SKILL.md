---
name: cmux
description: Reference for cmux CLI subcommands. Workspace management, screen reading, input sending, browser automation, and agent teams.
---

# cmux

Use when you need to interact with cmux workspaces, read terminal output from other panes, send input to other agents, use the built-in browser, or manage agent teams.

## Environment

cmux sets these env vars in terminals it manages:
- `CMUX_WORKSPACE_ID` - current workspace UUID
- `CMUX_SURFACE_ID` - current surface UUID
- `CMUX_TAB_ID` - current tab (optional)

Commands default to the caller's workspace/surface when these are set.

## Workspace Management

```bash
cmux list-workspaces                    # list all workspaces with refs
cmux current-workspace                  # show current workspace
cmux select-workspace --workspace workspace:N  # switch to workspace
cmux tree --workspace workspace:N       # show pane/surface tree
cmux new-workspace --name "title" --cwd /path  # create workspace
```

## Reading Terminal Output

```bash
# read current screen content (like tmux capture-pane)
cmux read-screen --workspace workspace:N --surface surface:N --lines 80

# include scrollback buffer
cmux read-screen --workspace workspace:N --surface surface:N --scrollback --lines 200
```

This is the primary way to monitor what other agents are doing.

## Sending Input

```bash
# send text (as if typed) - appends newline
cmux send --workspace workspace:N --surface surface:N "command here"

# send a key (Enter, Escape, Ctrl+C, etc.)
cmux send-key --workspace workspace:N --surface surface:N "Enter"
cmux send-key --workspace workspace:N --surface surface:N "C-c"
```

## Pane Management

```bash
cmux list-panes --workspace workspace:N
cmux new-pane --type terminal --direction right --workspace workspace:N
cmux new-pane --type browser --direction down --workspace workspace:N
cmux new-split right --workspace workspace:N --surface surface:N
cmux focus-pane --pane pane:N --workspace workspace:N
```

## Browser Automation

cmux has a built-in Playwright-based browser. Commands require `--surface surface:N` pointing to a browser surface.

### Navigation
```bash
cmux browser open "https://example.com"           # open URL in new/reused browser pane
cmux browser navigate "https://example.com" --surface surface:N
cmux browser url --surface surface:N               # get current URL
cmux browser back --surface surface:N
cmux browser reload --surface surface:N
cmux browser wait --load-state complete --timeout-ms 10000 --surface surface:N
```

### Reading Content
```bash
# compact DOM snapshot (best for finding elements)
cmux browser snapshot --surface surface:N --compact

# with depth limit
cmux browser snapshot --surface surface:N --compact --max-depth 6

# filter to specific element
cmux browser snapshot --surface surface:N --compact --selector "article"

# take a screenshot (agents can read images)
cmux browser screenshot --surface surface:N --out /tmp/screenshot.png
```

### Interaction
```bash
cmux browser click "[ref=eN]" --surface surface:N
cmux browser type "[ref=eN]" "text to type" --surface surface:N
cmux browser fill "[ref=eN]" "text" --surface surface:N
cmux browser press "Enter" --surface surface:N
```

Note: `[ref=eN]` references come from `browser snapshot` output.

### Known Limitations
- `browser scroll` does not support `--dy` or `--surface` flags directly. Use `browser eval "window.scrollBy(0, 500)"` as workaround, though JS eval can be flaky on some sites.
- `browser eval` may fail with "js_error" on pages with strict CSP.
- For scrolling, `browser press PageDown` is another option but also has reliability issues.
- Auth state (cookies, sessions) persists within the cmux process lifetime.

## Agent Teams

```bash
# start Claude Code with teams enabled
cmux claude-teams [claude-args...]

# check if teams are enabled
echo "$CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"  # should be "1"
```

## Notifications

```bash
cmux notify --title "Task done" --body "Details here" --workspace workspace:N
```

## Identifying Surfaces

```bash
# find which workspace/surface/pane you're in
cmux identify
cmux identify --workspace workspace:N --surface surface:N
```

## Tips

- Use `cmux tree --workspace workspace:N` first to understand the pane layout before sending input.
- `read-screen` is your main observability tool for monitoring other agents.
- Always specify `--workspace` and `--surface` explicitly rather than relying on env vars when targeting other workspaces.
- Browser surfaces and terminal surfaces have different ref formats. Use `list-pane-surfaces` to see both.
