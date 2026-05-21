---
name: peekaboo
description: Drive macOS apps via Peekaboo CLI — see, click, type, scroll, hotkey, menu, window, app, agent. Use when the user asks to automate a macOS GUI task (e.g. "open Notes and create a TODO", "click the Save button", "navigate Safari to example.com"). Peekaboo is higher-level than cua-driver — prefer it for agentic multi-step automation, natural-language tasks, and cases where cua-driver's pid/window_id loop is overkill.
---

# Peekaboo — macOS Screen + UI Automation

Peekaboo is a macOS CLI (and MCP server) that captures screenshots, maps UI elements, and drives apps via accessibility + synthetic input. Version 3.2.1+, installed via Homebrew.

## Quick reference

| Task | Command |
|---|---|
| Capture + annotate Safari | `peekaboo see --app Safari --annotate` |
| Capture all screens | `peekaboo image --mode screen --retina --path ~/Desktop/screen.png` |
| Click by element ID | `peekaboo click --on B1 --snapshot <ID>` |
| Click by label text | `peekaboo click --on "Submit" --snapshot <ID>` |
| Type text | `peekaboo type --text "hello" --on T1 --snapshot <ID>` |
| Set AX value directly | `peekaboo set-value --on T1 --value "hello" --snapshot <ID>` |
| Keyboard shortcut | `peekaboo hotkey cmd,shift,t` |
| Scroll element | `peekaboo scroll --on S1 --direction down --amount 3` |
| Launch app | `peekaboo app launch --name Notes` |
| Switch to app | `peekaboo app switch --to Safari` |
| List windows of app | `peekaboo window list --app Safari` |
| Focus a window | `peekaboo window focus --app Safari --title Hacker` |
| List/click menus | `peekaboo menu list --app Safari` → `peekaboo menu click "File,New Window"` |
| List menubar items | `peekaboo menubar list` |
| Click menubar item | `peekaboo menubar click --on "WiFi"` |
| Interact with dialogs | `peekaboo dialog click "Save"` |
| Dock interaction | `peekaboo dock launch --name Safari` |
| Drag and drop | `peekaboo drag --from B1 --to B2` |
| Swipe gesture | `peekaboo swipe --from 100,200 --to 300,400 --duration 500` |
| Move cursor | `peekaboo move --to 100,200` |
| List apps/windows | `peekaboo list apps` / `peekaboo list windows` |
| Check permissions | `peekaboo permissions status` |
| Grant permissions | `peekaboo permissions grant` |
| Clipboard read/write | `peekaboo clipboard get` / `peekaboo clipboard set "text"` |
| Open URL/file | `peekaboo open https://example.com` |
| Natural-language agent | `peekaboo agent "Open Notes and create a TODO list with three items"` |
| Run automation script | `peekaboo run path/to/script.peekaboo.json` |
| MCP server | `peekaboo mcp serve` (or `npx -y @steipete/peekaboo`) |
| Inspect AX tree only | `peekaboo inspect_ui --app Safari` |
| Analyze image with AI | `peekaboo analyze --path screenshot.png --prompt "What's in this image?"` |
| Chrome browser control | `peekaboo browser` (Chrome DevTools MCP integration) |
| Manage Spaces | `peekaboo space list` / `peekaboo space switch --to 2` |
| Config providers | `peekaboo config init` |
| Shell completions | `eval "$(peekaboo completions $SHELL)"` |
| AI agent guide | `peekaboo learn` |

## The core loop

```
peekaboo see --app Safari --json  →  get snapshot_id + element map
peekaboo click --on B1 --snapshot <ID>  →  act by element ID
peekaboo see --app Safari --json  →  verify
```

**Always** re-capture after any mutating action. Element IDs from `see` are valid only for the current screen state.

## Observation tools — when to use which

| Tool | Returns | Use when |
|---|---|---|
| `see` | Screenshot + annotated element map with IDs (e.g., B1, T2, S3) | You need visual context AND clickable element IDs. This is the default for most tasks. |
| `inspect_ui` | AX tree only (text, labels, roles, no image) | You only need element IDs and text; screenshot is unnecessary overhead. |
| `image` | Screenshot only (no element map) | You need a raw screenshot file saved to disk, optionally with AI analysis. |
| `list` | System state (apps, windows, screens) | Discovery: what's running, what's visible. |

## Element interaction

### Clicking

```bash
# By element ID (preferred — unambiguous)
peekaboo click --on B1 --snapshot abc123

# By label text (fuzzy match)
peekaboo click --on "Submit" --snapshot abc123

# By coordinates (last resort)
peekaboo click --coords 100,200

# Right-click, double-click
peekaboo click --on B1 --right
peekaboo click --on B1 --double

# Wait for element to appear (ms)
peekaboo click --on "Loading complete" --wait-for 5000
```

### Typing

```bash
# Type into element
peekaboo type --on T1 --text "hello world" --snapshot abc123

# With delay between keystrokes (ms, for human-like pacing)
peekaboo type --text "hello" --delay 50 --on T1 --snapshot abc123

# Clear field first, then type
peekaboo type --text "new value" --clear --on T1 --snapshot abc123
```

### Setting values directly (no keystrokes)

```bash
# Direct AX value write — faster than type, works on minimized windows
peekaboo set-value --on T1 --value "new text" --snapshot abc123
```

### Scrolling

```bash
peekaboo scroll --on S1 --direction down --amount 5
peekaboo scroll --direction up --amount 3  # no element = scroll at cursor
```

### Keyboard

```bash
# Modifier combos
peekaboo hotkey cmd,s       # Save
peekaboo hotkey cmd,shift,4 # Screenshot region

# Individual keys
peekaboo press return
peekaboo press escape
peekaboo press tab
```

## Application management

```bash
# Launch (idempotent — no-op if already running)
peekaboo app launch --name Notes
peekaboo app launch --bundle-id com.apple.Notes

# Switch focus
peekaboo app switch --to Safari

# Quit, relaunch, hide, unhide
peekaboo app quit --name Calculator
peekaboo app relaunch --name Safari
peekaboo app hide --name Notes
peekaboo app unhide --name Notes

# List running apps
peekaboo app list
peekaboo list apps
```

## Window management

```bash
# List windows of an app
peekaboo window list --app Safari

# Focus a specific window
peekaboo window focus --app Safari --title "GitHub"

# Move/resize
peekaboo window move --app Terminal --x 100 --y 200
peekaboo window resize --app Terminal --width 800 --height 600
peekaboo window set-bounds --app Terminal --x 0 --y 0 --width 1280 --height 720

# Close, minimize, maximize
peekaboo window close --app Calculator
peekaboo window minimize --app Notes
peekaboo window maximize --app Safari
```

## Menu and menubar

```bash
# List available menus
peekaboo menu list --app Safari

# Click a menu item by path
peekaboo menu click "File,New Window"

# List menubar status items
peekaboo menubar list

# Click a menubar item
peekaboo menubar click --on "WiFi"
peekaboo menubar click --index 3
```

## Dialogs

```bash
# List dialog elements
peekaboo dialog list

# Click a dialog button
peekaboo dialog click "Save"

# Type into a dialog text field
peekaboo dialog input --text "filename.txt"

# Handle file dialogs
peekaboo dialog file --path ~/Documents/report.pdf

# Dismiss
peekaboo dialog dismiss
```

## Dock interaction

```bash
# Launch from Dock
peekaboo dock launch --name Safari

# Right-click Dock item (context menu)
peekaboo dock right-click --name Finder

# Hide/show Dock
peekaboo dock hide
peekaboo dock show

# List Dock items
peekaboo dock list
```

## Spaces (virtual desktops)

```bash
# List all Spaces
peekaboo space list

# Switch to Space 2
peekaboo space switch --to 2

# Move a window to a different Space
peekaboo space move-window --app Terminal --to 3
```

## Gestures

```bash
# Drag from element A to element B
peekaboo drag --from B1 --to B2 --snapshot abc123

# Drag from coordinates to coordinates
peekaboo drag --from 100,200 --to 500,600

# Swipe (smooth gesture)
peekaboo swipe --from 100,200 --to 500,200 --duration 500 --steps 20

# Move cursor (no click)
peekaboo move --to 100,200
```

## Clipboard

```bash
# Read clipboard
peekaboo clipboard get

# Write clipboard
peekaboo clipboard set "text to copy"

# Clear clipboard
peekaboo clipboard clear

# Save/restore clipboard state
peekaboo clipboard save
peekaboo clipboard restore
```

## Permissions

```bash
# Check permission status
peekaboo permissions status

# Prompt user to grant permissions (opens System Settings)
peekaboo permissions grant
```

Required permissions:
- **Screen Recording** — for `see`, `image`, screenshots
- **Accessibility** — for `click`, `type`, `scroll`, `inspect_ui`, all UI automation

Without these, most commands fail with a permission error.

## Natural-language agent

```bash
# Simple task
peekaboo agent "Open Safari and navigate to github.com"

# Complex multi-step
peekaboo agent "Open Notes, create a new note titled 'Shopping List', and add: milk, eggs, bread"

# With specific model
peekaboo agent "..." --model "anthropic/claude-opus-4-7"

# Dry run (see what steps it would take)
peekaboo agent "..." --dry-run

# Resume a previous session
peekaboo agent "..." --resume <session-id>

# Limit steps
peekaboo agent "..." --max-steps 10
```

## MCP server

Run Peekaboo as an MCP server for Claude Code, Codex, Cursor, or Hermes:

```bash
# Via Homebrew-installed CLI
peekaboo mcp serve

# Via npm (no install needed, Node 22+)
npx -y @steipete/peekaboo
```

MCP client config snippet:

```json
{
  "mcpServers": {
    "peekaboo": {
      "command": "npx",
      "args": ["-y", "@steipete/peekaboo"],
      "env": {
        "PEEKABOO_AI_PROVIDERS": "openai/gpt-5.5,anthropic/claude-opus-4-7"
      }
    }
  }
}
```

## AI provider configuration

Peekaboo's `agent` and `analyze` commands need AI providers. Configure via env var or config file:

```bash
# Environment variable
export PEEKABOO_AI_PROVIDERS="openai/gpt-5.5,anthropic/claude-opus-4-7"

# Or config file
peekaboo config init
peekaboo config add --provider openai --model gpt-5.5
peekaboo config models
```

Supported providers: OpenAI, Anthropic, xAI/Grok, Google Gemini, OpenRouter, Ollama, LM Studio, and compatible custom endpoints.

## Chrome browser control

Peekaboo integrates Chrome DevTools MCP for web page automation:

```bash
peekaboo browser  # status, connect, inspect, console, network, etc.
```

Requires Chrome with remote debugging enabled. See `peekaboo learn` for full browser tool help.

## Pitfalls

- **Element IDs are ephemeral.** After any action that changes the UI (click, type, scroll), element IDs from the previous `see` call are stale. Always re-capture.
- **Snapshot IDs matter.** The `--snapshot` flag ties a click/type to a specific `see` output. If omitted, Peekaboo uses the latest snapshot — which may be wrong if another operation happened.
- **`open -a` activates the target app.** Use `peekaboo app launch` instead to avoid focus-stealing. Same for `osascript activate`.
- **Menu bar commands require the target to be frontmost** for many document/editor actions (items go DISABLED otherwise).
- **Chromium right-click on web content** coerces to left-click in some cases. Use AX-based `right_click` for context menus.
- **Keyboard commits (Return/Space) fail on minimized windows** — use `set-value` or click a button instead.
- **Large AX trees** can be truncated. Use `--max-children`, `--max-depth`, `--max-elements` flags on `see` to adjust.
- **`peekaboo agent` needs AI provider configured.** Without `PEEKABOO_AI_PROVIDERS` or `peekaboo config`, agent mode fails.

## Proven workflows

### Open a website in Safari

```bash
peekaboo app launch --name Safari
# If you need a specific URL, use the open tool:
peekaboo open https://github.com
```

### Fill a form

```bash
SNAPSHOT=$(peekaboo see --app Safari --json | jq -r '.data.snapshot_id')
peekaboo click --on "Name" --snapshot "$SNAPSHOT"
peekaboo type --text "Kirill" --snapshot "$SNAPSHOT"
SNAPSHOT=$(peekaboo see --app Safari --json | jq -r '.data.snapshot_id')
peekaboo click --on "Submit" --snapshot "$SNAPSHOT"
```

### Navigate Finder to a folder

```bash
peekaboo app launch --name Finder
peekaboo hotkey cmd,shift,g          # Go to Folder
peekaboo type --text "~/Documents"
peekaboo press return
```

### Create a note in Apple Notes via agent

```bash
peekaboo agent "Open Notes, create a new note titled 'Meeting Notes', and type: Agenda: Q2 review"
```

## Differences from cua-driver

If you know cua-driver, here's the mapping:

| cua-driver | Peekaboo |
|---|---|
| `get_window_state(pid, window_id)` | `peekaboo see --app <name>` |
| `click({pid, element_index})` | `peekaboo click --on <id> --snapshot <id>` |
| `type_text({pid, text})` | `peekaboo type --text "..." --on <id>` |
| `scroll({pid, direction})` | `peekaboo scroll --direction down --on <id>` |
| `hotkey({pid, keys})` | `peekaboo hotkey cmd,s` |
| `launch_app({bundle_id})` | `peekaboo app launch --name <app>` |
| `list_windows({pid})` | `peekaboo window list --app <name>` |
| No equivalent | `peekaboo agent "natural language task"` |
| No equivalent | `peekaboo mcp serve` |
| `screenshot` | `peekaboo image --mode window --app <name>` |

Key differences:
- Peekaboo uses app names, not pids/window_ids — less precise but much simpler.
- Peekaboo has a natural-language `agent` mode for multi-step tasks.
- Peekaboo works as an MCP server out of the box.
- Peekaboo returns snapshot_id rather than caching server-side.
- cua-driver is better for precise single-window programmatic control; Peekaboo is better for agentic multi-app workflows.

## Installation

```bash
brew install steipete/tap/peekaboo
```

Repo: `~/Public/peekaboo` (cloned from `openclaw/Peekaboo`)
