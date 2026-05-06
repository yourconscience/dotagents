---
name: cmux
description: cmux CLI reference. Workspaces, panes, terminals, browser, markdown, agent launchers.
---

# cmux

## Environment

cmux sets `CMUX_WORKSPACE_ID`, `CMUX_SURFACE_ID`, `CMUX_TAB_ID` in managed terminals. Commands default to the caller's workspace/surface when set.

## Workspaces

```bash
cmux list-workspaces
cmux current-workspace
cmux select-workspace --workspace workspace:N
cmux tree --workspace workspace:N          # pane/surface tree
cmux tree --all
cmux new-workspace --name "title" --cwd /path
cmux close-workspace --workspace workspace:N
```

## Reading / Sending

```bash
cmux read-screen --workspace workspace:N --surface surface:N --lines 80
cmux read-screen --workspace workspace:N --surface surface:N --scrollback --lines 200
cmux send --workspace workspace:N --surface surface:N "command here"
cmux send-key --workspace workspace:N --surface surface:N "Enter"
cmux send-key --workspace workspace:N --surface surface:N "C-c"
```

## Panes and Surfaces

Panes hold tabs (surfaces). `new-split` creates a pane (visual split); `new-surface` adds a tab.

```bash
cmux list-panes --workspace workspace:N
cmux new-split right --workspace workspace:N --surface surface:N
cmux new-pane --type terminal --direction right --workspace workspace:N
cmux new-surface --type terminal --pane pane:N --workspace workspace:N
cmux focus-pane --pane pane:N --workspace workspace:N
cmux close-surface --surface surface:N --workspace workspace:N
cmux move-surface --surface surface:N --pane pane:N --workspace workspace:N
```

## Browser

```bash
cmux browser open "https://example.com"              # new tab
cmux browser open-split "https://example.com"         # new split
cmux browser navigate "https://..." --surface surface:N
cmux browser snapshot --surface surface:N --compact    # DOM snapshot
cmux browser snapshot --surface surface:N --interactive # with [ref=eN] refs
cmux browser screenshot --surface surface:N --out /tmp/shot.png
cmux browser click "[ref=eN]" --surface surface:N
cmux browser type "[ref=eN]" "text" --surface surface:N
cmux browser fill "[ref=eN]" "text" --surface surface:N
cmux browser press "Enter" --surface surface:N
cmux browser scroll --dy 500 --surface surface:N
cmux browser wait --selector ".results" --surface surface:N
cmux browser eval "document.title" --surface surface:N
cmux browser url --surface surface:N
cmux browser back --surface surface:N
cmux browser reload --surface surface:N
```

`[ref=eN]` refs come from `browser snapshot --interactive`.

## Markdown

```bash
cmux markdown open plan.md                    # new tab (default)
cmux markdown open plan.md --direction right  # split instead
```

When an agent generates a markdown file (research reports, specs, plans), open it in the viewer so the user can read the formatted output immediately:
```bash
cmux markdown open /path/to/generated.md --direction right
```

## Visible delegation workflow

When the user asks for visible delegation, prefer a dedicated cmux workspace, surface, or tab for the delegated agent instead of shrinking the user's main TUI. Check that `cmux` is available before relying on it; if unavailable, give Warp or regular-terminal instructions instead of assuming cmux.

## Agent Launchers

```bash
cmux claude-teams [args...]    # Claude Code with teams
cmux omx [args...]             # Codex (prefer plain `omx` from agents)
cmux omo [args...]             # opencode
cmux hermes [args...]          # Hermes Agent
```

## Other

```bash
cmux identify
cmux notify --title "Done" --body "Details" --workspace workspace:N
cmux wait-for "build-done" --timeout 30
cmux ssh user@host --name "server"
cmux set-buffer --name scratch "text"
cmux paste-buffer --name scratch --surface surface:N
```

## Tips

- `cmux tree` first to understand layout before sending input.
- `read-screen` is the main tool for monitoring other agents.
- Always specify `--workspace` and `--surface` explicitly when targeting other workspaces.
