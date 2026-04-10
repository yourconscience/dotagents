---
name: omx
description: Delegate substantial coding, review, or research work to a Codex (GPT-5) agent running in a visible cmux pane via oh-my-codex (omx), poll its progress in the background, and inspect diffs without leaving the main Claude Code session. Use when the user says /omx, asks to "run codex in a pane", "delegate to codex", "offload to GPT-5", or wants a long-running codex task visible while Claude keeps working.
---

# omx

Spawn a Codex agent in a dedicated cmux workspace, let it work in parallel with Claude Code, poll status via `omx hud --json`, and inspect changes via `git diff`.

## When to use

- The user wants to delegate a substantial task (refactor, multi-file change, review pass, independent research) to Codex GPT-5 while Claude keeps working on something else.
- The user wants a visible, interactive Codex session they can eyeball or intervene in (not a headless subagent).
- The task is large enough to benefit from GPT-5's longer reasoning budget or a different model perspective.
- Good fits: adversarial review, second opinion on a design, mechanical refactors, long-running test/debug loops, deep-research reports.
- Bad fits: tiny edits, anything under ~2 minutes, tasks requiring tight coupling with Claude's current context.

## Preflight

Run once before launching:

```bash
which omx                    # should print /opt/homebrew/bin/omx
omx version                  # confirm installed (currently v0.12.4)
omx doctor 2>&1 | tail -20   # sanity-check install health
```

If `omx doctor` reports missing pieces, run `omx setup` (it installs skills, prompts, MCP servers, and scope-specific AGENTS.md into `~/.codex/`).

## Isolation: dedicated codex workspace

The user's Claude Code TUI must stay full-size. Do NOT split Claude's pane. Instead, spawn codex into a dedicated cmux workspace so it lives on its own sidebar tab.

Layout model:
- `window > workspace > pane > surface`
- Each workspace is a sidebar tab. Switching workspaces hides the other entirely, so panes in workspace "codex" never shrink the Claude TUI in workspace "hb" (or wherever Claude is).
- Multiple codex sessions become splits or surface tabs inside the single "codex" workspace.

### Reuse or create the codex workspace

```bash
CODEX_WS=$(cmux list-workspaces 2>&1 | awk '/codex/ {print $2; exit}')
if [ -z "$CODEX_WS" ]; then
  cmux new-workspace --name codex --cwd "$(pwd)"
  CODEX_WS=$(cmux list-workspaces 2>&1 | awk '/codex/ {print $2; exit}')
fi
echo "codex workspace: $CODEX_WS"
```

First launch creates the workspace; subsequent launches reuse it.

### Add a second (or third) codex session inside that workspace

Use surfaces (tabs within a pane) for multiple codex sessions in one workspace. Surfaces are free: they do not shrink each other.

```bash
# Find the first pane in the codex workspace:
CODEX_PANE=$(cmux tree 2>&1 | awk -v ws="$CODEX_WS" '
  $0 ~ ws {in_ws=1; next}
  in_ws && /pane pane:/ {match($0, /pane:[0-9]+/); print substr($0, RSTART, RLENGTH); exit}
')
# Spawn a new terminal surface (tab) in that pane:
cmux new-surface --pane "$CODEX_PANE" --type terminal
```

If the user explicitly wants side-by-side visibility (not tabs), use `cmux new-pane --workspace "$CODEX_WS" --direction right` instead.

## Launch a codex task

Write the prompt to a temp file so send strings stay small and quoting stays sane:

```bash
PROMPT_FILE=$(mktemp /tmp/codex-prompt.XXXXXX.md)
cat > "$PROMPT_FILE" <<'EOF'
<paste the full self-contained task brief here>
EOF
```

Then send it into the target codex surface. The identifying surface is captured from `cmux new-surface` output (or the initial surface of the workspace). Replace `surface:NN` below.

```bash
cmux send --surface surface:NN "cd $(pwd) && cmux omx --high \"\$(cat $PROMPT_FILE)\""
```

### Mode cheat sheet

| Need | Command | Behavior |
|---|---|---|
| Safe review/research | `cmux omx --high "..."` | interactive codex, default sandbox (read+approve) |
| Let it write files, no approval prompts | `cmux omx --yolo --high "..."` | yolo: write without asking, sandboxed cwd |
| Heavy reasoning | add `--xhigh` | max reasoning effort |
| Full bypass, trusted task | `cmux omx --madmax --high "..."` | bypass approvals AND sandbox - use only on trusted prompts in throwaway dirs |
| Resume previous session | `cmux omx resume` | picks from prior interactive sessions |
| Non-interactive one-shot | `cmux omx exec --yolo --high "..."` | headless, returns when done |

Use `--high` as a near-default: GPT-5 is strong enough that the extra cost is usually worth it. Escalate to `--xhigh` for ambiguous design/debug work.

### Handling first-run prompts

On a fresh install Codex may prompt for:
- "Star the repo?" - send `n` then Enter
- "Trust this directory?" - send `y` then Enter

```bash
cmux send --surface surface:NN "n"
cmux send-key --surface surface:NN Enter
```

If the prompt text is unfamiliar, capture the pane first and read before sending:

```bash
cmux capture-pane --surface surface:NN --lines 40
```

## Poll progress (background loop)

`omx hud --json` is the background polling primitive. It reports the session the user is actively running, so run it from the same user account on the same machine. Key fields:

- `session.pid`, `session.cwd`, `session.started_at`
- `metrics.last_activity` (unix seconds)
- `hudNotify.last_agent_output` (last streamed chunk, truncated)
- `hudNotify.turn_count`

Minimal poll:

```bash
omx hud --json | jq '{
  pid: .session.pid,
  cwd: .session.cwd,
  turns: .hudNotify.turn_count,
  last_activity: .metrics.last_activity,
  last_output: (.hudNotify.last_agent_output // "" | .[0:200])
}'
```

### Background poll pattern

Claude can spawn a polling loop as a `run_in_background` Bash task. Keep the interval at 60-120s to stay inside the prompt cache window.

```bash
# background poll, appends snapshots to a log
while true; do
  date +%s
  omx hud --json | jq -c '{
    turns: .hudNotify.turn_count,
    last_activity: .metrics.last_activity,
    last_output: (.hudNotify.last_agent_output // "" | .[0:200])
  }'
  sleep 90
done >> /tmp/omx-poll.log 2>&1
```

Then `tail -20 /tmp/omx-poll.log` periodically. Do NOT poll every few seconds; the HUD JSON is roughly 10-40 KB per read and will overflow context fast.

## Inspect changes

Three complementary views:

1. **Visual pane state** (what the user sees):
   ```bash
   cmux capture-pane --surface surface:NN --lines 80
   ```
2. **Filesystem diff** (ground truth):
   ```bash
   git -C <repo> status -sb
   git -C <repo> diff --stat
   git -C <repo> diff <path>
   ```
3. **Agent self-report** (turn count, last output):
   ```bash
   omx hud --json | jq '.hudNotify'
   ```

Cross-check all three before claiming "done". The HUD can show `last_activity` old even when the agent is mid-turn (buffered output).

## Wrap up a session

```bash
# interactively: user kills it, or
cmux send-key --surface surface:NN C-c
cmux send-key --surface surface:NN C-c        # second ^C exits codex cleanly
# close the surface if the tab should go away:
cmux close-surface --surface surface:NN
```

Leave the `codex` workspace alive so the next launch reuses it. Only `cmux close-workspace` if the user explicitly asks.

## Known caveats

- **`omx team` workers die under cmux**: testing with `cmux omx team 2:writer "..."` in this cmux environment reproducibly lands both workers in `dead_worker` state (`alive:false`, `turn_count:null`). Root cause is suspected tmux-shim incompatibility (`cmux omx` startup logs `Unsupported tmux compatibility command: show-options`). Until resolved, prefer a single `cmux omx` session per task. If true parallelism is needed, spawn multiple independent surfaces manually instead of using team mode.
- **tmux compat shim warnings are cosmetic**: the `show-options` warning does not break normal `cmux omx` runs, only `omx team` bootstrap.
- **First run touches `~/.codex/`**: `omx setup` overwrites `~/.codex/config.toml`. If the user has a hand-tuned codex config, back it up before running setup.
- **Madmax is actually dangerous**: `--madmax` bypasses both approvals and the sandbox. Never use it on prompts whose content Claude has not fully vetted; never in the user's real home directory without explicit opt-in.

## Resume workflow

```bash
cmux send --surface surface:NN "cmux omx resume"
cmux capture-pane --surface surface:NN --lines 40
```

Codex shows a picker of prior interactive sessions. Send the session number + Enter.

## Alternative: omc for Claude Code orchestration

`cmux omc` (oh-my-claude-sisyphus) is the sibling tool for orchestrating Claude Code itself rather than Codex. Use it when the user wants to delegate to another *Claude* session in a pane (not GPT-5). Surface summary:

- `cmux omc team 3:claude "implement feature"` - spawn a 3-worker Claude team
- `cmux omc --watch` - live HUD
- installed separately via `oh-my-claude-sisyphus`

Do not mix omc and omx in the same workspace; use distinct cmux workspaces per delegatee kind.

## Quick recipe (TL;DR)

```bash
# 1. ensure codex workspace exists
cmux list-workspaces | grep -q codex || cmux new-workspace --name codex --cwd "$(pwd)"

# 2. write prompt
PROMPT_FILE=$(mktemp /tmp/codex-prompt.XXXXXX.md)
cat > "$PROMPT_FILE" <<'EOF'
<full task brief - self-contained, no hidden context>
EOF

# 3. find the codex workspace's initial surface and send the command
#    (capture surface ref from `cmux tree` under workspace codex)
cmux send --surface surface:NN "cd $(pwd) && cmux omx --yolo --high \"\$(cat $PROMPT_FILE)\""

# 4. poll in background
omx hud --json | jq '.hudNotify.turn_count, .metrics.last_activity'

# 5. inspect
git status -sb && git diff --stat
cmux capture-pane --surface surface:NN --lines 60
```
