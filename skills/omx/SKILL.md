---
name: omx
description: Delegate substantial coding, review, or research work to a Codex (GPT-5) agent running in a visible cmux pane via oh-my-codex (omx), poll its progress in the background, and inspect diffs without leaving the main Claude Code session. Use when the user says /omx, asks to "run codex in a pane", "delegate to codex", "offload to GPT-5", or wants a long-running codex task visible while Claude keeps working.
---

# omx

Spawn a Codex agent as a surface tab next to Claude in the same cmux pane, let it work in parallel with Claude Code, poll status via `omx hud --json`, and inspect changes via `git diff`.

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

## Isolation: surface tab next to Claude (NOT a new workspace)

The user's Claude Code TUI must stay full-size. Do NOT split Claude's pane, and do NOT create a new workspace. Instead, spawn a new SURFACE TAB inside Claude's current pane. Surfaces are tab-bar entries within a pane; switching tabs swaps the visible terminal without resizing anything, so neither Claude nor codex shrinks.

Layout model:
- `window > workspace > pane > surface`
- A surface is a tab inside a pane. Claude Code is running in a surface right now; adding a sibling surface to the same pane just creates a tab next to it.
- Spawning a new workspace is the WRONG move here: `cmux omx` (the wrapper) forces a tmux-like env that makes OMX auto-attach `omx hud --watch` as a split, and under the wrapper the split can leak into the caller's workspace and shrink Claude. Use plain `omx`, not `cmux omx`, and keep the layout flat.

### Identify Claude's current pane and add a codex tab

Claude knows its own surface from `CMUX_SURFACE_ID` env var. Derive the pane from `cmux tree`:

```bash
CLAUDE_SURFACE="${CMUX_SURFACE_ID:-surface:20}"
CLAUDE_PANE=$(cmux tree 2>&1 | awk -v s="$CLAUDE_SURFACE" '
  /pane pane:/ {
    match($0, /pane:[0-9]+/); p = substr($0, RSTART, RLENGTH)
  }
  $0 ~ s {print p; exit}
')
CODEX_SURFACE=$(cmux new-surface --pane "$CLAUDE_PANE" --type terminal 2>&1 | awk '{print $2}')
CODEX_WS=$(cmux tree 2>&1 | awk -v s="$CODEX_SURFACE" '
  /workspace workspace:/ {match($0, /workspace:[0-9]+/); w = substr($0, RSTART, RLENGTH)}
  $0 ~ s {print w; exit}
')
echo "codex tab: $CODEX_SURFACE in $CODEX_WS (next to claude in $CLAUDE_PANE)"
```

Every `cmux send` / `cmux capture-pane` against the codex surface MUST pass both `--workspace "$CODEX_WS"` and `--surface "$CODEX_SURFACE"` — cmux requires the workspace context for non-focused surfaces.

### Register the session in the local registry

Write one JSON line per spawned surface to `/tmp/omx-sessions.jsonl` so Claude can list, report, close, or resume sessions later without re-deriving anything. Do this immediately after `cmux new-surface`:

```bash
OMX_TASK_LABEL="${OMX_TASK_LABEL:-codex task}"
OMX_START_TS=$(date +%s)
OMX_CWD=$(pwd)
OMX_SHA_BEFORE=$(git -C "$OMX_CWD" rev-parse HEAD 2>/dev/null || echo "")
jq -cn \
  --arg ts "$OMX_START_TS" \
  --arg ws "$CODEX_WS" \
  --arg sf "$CODEX_SURFACE" \
  --arg pn "$CLAUDE_PANE" \
  --arg label "$OMX_TASK_LABEL" \
  --arg cwd "$OMX_CWD" \
  --arg prompt "${PROMPT_FILE:-}" \
  --arg sha "$OMX_SHA_BEFORE" \
  '{start: $ts|tonumber, workspace: $ws, surface: $sf, pane: $pn, label: $label, cwd: $cwd, prompt_file: $prompt, sha_before: $sha}' \
  >> /tmp/omx-sessions.jsonl
```

The registry is the single source of truth for the "Session lifecycle" section below. Treat it as append-only during a session, prune stale entries on explicit close.

### Git identity for commits made by codex

Commits codex produces inherit `GIT_AUTHOR_*` / `GIT_COMMITTER_*` from the shell via `~/.zshrc`, where those vars are `export`ed globally (same identity used by `cc`, `cx`, `oc`). No per-launch env wiring is needed: as long as the codex surface is a zsh shell spawned from a normal login shell, `git commit` inside codex signs with the user's identity automatically. If you need to verify: `cmux send --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" 'echo "$GIT_AUTHOR_EMAIL"'`.

### Multiple concurrent codex sessions

Each additional codex session is another `cmux new-surface --pane "$CLAUDE_PANE" --type terminal` call — just another tab in the same tab bar. Tabs are free, they do not resize each other.

### How omx detects tmux and why it matters here

Claude Code, when launched via the `cc` alias, runs inside an outer `tmux` session named `cc`. Codex launched via `cx` runs in its own outer tmux session too. That means when Claude invokes `omx` via `cmux send` into a sibling surface, omx sees `$TMUX` set (inherited from the parent cc process environment) and assumes it is inside tmux. Its response is to **spawn a detached sub-tmux session** named like `omx-dotagents-main-<ts>-<id>` where the actual codex process runs — and **leave the cmux surface at the idle zsh prompt**. The codex UI is NOT visible in the cmux surface; it is running headless in the detached tmux session.

Practical implications:

- `cmux capture-pane` on the spawned surface shows only the zsh prompt, never codex output.
- `cmux send-key C-c C-c` to the spawned surface does NOT quit codex — it ^Cs the empty zsh.
- `tmux list-sessions` shows both `cc` (your claude) and `omx-<project>-main-<ts>-<id>` (the detached codex).
- Codex still makes real progress and reports via `omx hud --json`, which is why background polling still works.

If you want the codex UI visible in the cmux surface, attach the detached session there after launching:

```bash
OMX_TMUX_SESSION=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
cmux send --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" "tmux attach -t $OMX_TMUX_SESSION"
cmux send-key --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" Enter
```

If you only need background delegation (the most common case), skip the attach — HUD polling + `git diff` + a final `tmux capture-pane -t $OMX_TMUX_SESSION -p` on finish is enough.

### Do NOT use `cmux omx` — use plain `omx`

`cmux omx` wraps OMX in an extra tmux compat shim that causes HUD auto-attach to leak into the wrong workspace (can shrink Claude's pane) and breaks `omx team` workers (shim does not implement `show-options`). Plain `omx` leverages the real outer tmux and stays well-behaved.

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
cmux send --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" "cd $(pwd) && omx --yolo --high \"\$(cat $PROMPT_FILE)\""
cmux send-key --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" Enter
```

### Mode cheat sheet

| Need | Command | Behavior |
|---|---|---|
| Safe review/research | `omx --high "..."` | interactive codex, default sandbox (read+approve) |
| Let it write files, no approval prompts | `omx --yolo --high "..."` | yolo: write without asking, sandboxed cwd |
| Heavy reasoning | add `--xhigh` | max reasoning effort |
| Full bypass, trusted task | `omx --madmax --high "..."` | bypass approvals AND sandbox - use only on trusted prompts in throwaway dirs |
| Resume previous session | `omx resume` | picks from prior interactive sessions |
| Non-interactive one-shot | `omx exec --yolo --high "..."` | headless, returns when done |

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

### Tell "stuck" apart from "idle post-task"

Do NOT assume codex is stuck just because it stopped producing output. These three states look similar from the outside but mean different things:

| State | HUD signal | Pane signal | Action |
|---|---|---|---|
| Actively working | `last_activity` within last 60s, `turn_count` increasing between polls | streamed text, spinner, `• Working (Nm Ns ...)` | wait |
| Idle post-task (done) | `last_activity` many minutes old, `turn_count` stable, or hud `null` if session was closed | prompt shows `› Summarize recent commits` placeholder, `gpt-5.4 high · N% left` footer, NO spinner, Stop hook already ran in the transcript above | task is COMPLETE, codex is waiting for next user input. Read the transcript, report results, close the surface or leave it for resume |
| Genuinely stuck | `last_activity` minutes old, `turn_count` stable, pane has a spinner or a half-printed command, no Stop hook in transcript | spinner that never advances, or unfinished `• Ran ...` line | capture 80+ lines of scrollback, look for the last tool call. If it is waiting on a prompt (y/n), send the answer. Otherwise ^C twice and retry. |

The `› Summarize recent commits` line is codex's placeholder suggestion at the idle prompt, not a running task. If you see it, codex is done.

## Session lifecycle

All lifecycle operations read `/tmp/omx-sessions.jsonl`. The registry is the single source of truth — never guess surfaces from `cmux tree` alone.

### List active sessions

```bash
jq -r '"\(.surface)\t\(.workspace)\t\(.label)\t\(.cwd)\t\(.start|todate)"' /tmp/omx-sessions.jsonl 2>/dev/null \
  | column -t -s $'\t'
```

### Report what a session produced

For the codex session identified by `$CODEX_SURFACE`, show commits since launch, working-tree diff, last HUD state, and a pane tail. This is the "share status about what was made" view Claude should present to the user before closing.

```bash
ENTRY=$(jq -c --arg sf "$CODEX_SURFACE" 'select(.surface == $sf)' /tmp/omx-sessions.jsonl | tail -1)
CWD=$(echo "$ENTRY" | jq -r '.cwd')
SHA_BEFORE=$(echo "$ENTRY" | jq -r '.sha_before')
LABEL=$(echo "$ENTRY" | jq -r '.label')
WS=$(echo "$ENTRY" | jq -r '.workspace')

echo "== $LABEL ($CODEX_SURFACE in $WS, cwd $CWD) =="
if [ -n "$SHA_BEFORE" ]; then
  echo "-- commits since launch --"
  git -C "$CWD" log --oneline "$SHA_BEFORE..HEAD" 2>/dev/null || echo "(no new commits)"
fi
echo "-- working tree --"
git -C "$CWD" status -sb
git -C "$CWD" diff --stat
echo "-- HUD --"
omx hud --json 2>/dev/null | jq '{turns: .hudNotify.turn_count, last_activity: .metrics.last_activity, last_output: (.hudNotify.last_agent_output // "" | .[0:200])}'
echo "-- codex transcript tail (from detached omx tmux session) --"
OMX_TMUX_SESSION=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
if [ -n "$OMX_TMUX_SESSION" ]; then
  tmux capture-pane -t "$OMX_TMUX_SESSION" -p 2>&1 | tail -40
else
  echo "(no omx tmux session found — codex either exited or never started)"
fi
```

The report reads from the detached `omx-*` tmux session, not the cmux surface. The cmux surface just hosts the zsh that launched omx; it does not contain codex's transcript.

### Close a session explicitly

Closing is three steps: quit codex cleanly **in the detached tmux session**, close the cmux surface, prune the registry. The cmux surface's C-c does NOT reach codex — you must send it to the tmux session.

```bash
ENTRY=$(jq -c --arg sf "$CODEX_SURFACE" 'select(.surface == $sf)' /tmp/omx-sessions.jsonl | tail -1)
WS=$(echo "$ENTRY" | jq -r '.workspace')

# 1) quit codex cleanly in its detached tmux session
OMX_TMUX_SESSION=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
if [ -n "$OMX_TMUX_SESSION" ]; then
  tmux send-keys -t "$OMX_TMUX_SESSION" C-c
  tmux send-keys -t "$OMX_TMUX_SESSION" C-c
fi

# 2) close the cmux surface (the empty zsh that launched omx)
cmux close-surface --workspace "$WS" --surface "$CODEX_SURFACE"

# 3) drop the entry from the registry
jq -c --arg sf "$CODEX_SURFACE" 'select(.surface != $sf)' /tmp/omx-sessions.jsonl > /tmp/omx-sessions.jsonl.tmp \
  && mv /tmp/omx-sessions.jsonl.tmp /tmp/omx-sessions.jsonl
```

If the user wants to reuse the tab for a follow-up, skip the close step and use "Resume" below instead.

### Resume a session (send more instructions)

Two flavors depending on whether the detached codex tmux session is still alive:

**Codex still running (detached tmux session still listed, idle post-task at the `›` prompt)** — send the new turn into the tmux session directly:

```bash
OMX_TMUX_SESSION=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
FOLLOWUP_FILE=$(mktemp /tmp/codex-followup.XXXXXX.md)
cat > "$FOLLOWUP_FILE" <<'EOF'
<new instructions, self-contained>
EOF
# send the text as a single paste, then Enter
tmux load-buffer -b omxfu "$FOLLOWUP_FILE"
tmux paste-buffer -b omxfu -t "$OMX_TMUX_SESSION"
tmux send-keys -t "$OMX_TMUX_SESSION" Enter
tmux delete-buffer -b omxfu
```

Then poll with `tmux capture-pane -t "$OMX_TMUX_SESSION" -p` and `omx hud --json` as usual.

**Codex already exited (detached tmux session gone)** — launch a new codex in the same cmux surface via `omx resume`, which picks a prior interactive session by id:

```bash
ENTRY=$(jq -c --arg sf "$CODEX_SURFACE" 'select(.surface == $sf)' /tmp/omx-sessions.jsonl | tail -1)
WS=$(echo "$ENTRY" | jq -r '.workspace')
cmux send --workspace "$WS" --surface "$CODEX_SURFACE" "omx resume"
cmux send-key --workspace "$WS" --surface "$CODEX_SURFACE" Enter
# then read the picker from the new omx tmux session that gets spawned:
sleep 2
OMX_TMUX_SESSION=$(tmux list-sessions 2>&1 | awk -F: '/^omx-/ {print $1; exit}')
tmux capture-pane -t "$OMX_TMUX_SESSION" -p 2>&1 | tail -40
# send the session number + Enter via tmux send-keys -t "$OMX_TMUX_SESSION"
```

## Known caveats

- **Plain `omx` backgrounds codex into a detached tmux session**. When launched from inside the outer `cc` tmux session (which Claude Code runs in by default), `omx` detects `$TMUX` and spawns a detached sub-session named `omx-<project>-main-<ts>-<id>`. The codex UI is NOT rendered in the cmux surface — that surface just hosts the idle zsh that called omx. All `tmux send-keys` / `tmux capture-pane` for interacting with or reading codex must target the detached session, not the cmux surface. The "Session lifecycle" section above reflects this.
- **Never use `cmux omx`; always use plain `omx`**. `cmux omx` wraps OMX in an additional tmux compat shim. HUD auto-attach then leaks into the wrong workspace (can shrink Claude's pane) and `omx team` workers die at boot (`dead_worker`, shim missing `show-options`). Plain `omx` uses the real outer tmux and stays well-behaved.
- **`omx team` is unreliable in this environment**. Workers need a clean outer tmux and still frequently land in `dead_worker` state. For parallelism, spawn multiple independent codex surfaces (one `cmux new-surface` + one `omx` launch per task) instead of using team mode.
- **First run touches `~/.codex/`**: `omx setup` overwrites `~/.codex/config.toml`. If the user has a hand-tuned codex config, back it up before running setup.
- **Madmax is actually dangerous**: `--madmax` bypasses both approvals and the sandbox. Never use it on prompts whose content Claude has not fully vetted; never in the user's real home directory without explicit opt-in.
- **Idle post-task is not stuck**: see the "Tell stuck apart from idle post-task" table above. A codex surface sitting at `› Summarize recent commits` with `gpt-5.4 high · N% left` has already finished and is waiting for the next prompt.

## Alternative: omc for Claude Code orchestration

`cmux omc` (oh-my-claude-sisyphus) is the sibling tool for orchestrating Claude Code itself rather than Codex. Use it when the user wants to delegate to another *Claude* session in a pane (not GPT-5). Surface summary:

- `cmux omc team 3:claude "implement feature"` - spawn a 3-worker Claude team
- `cmux omc --watch` - live HUD
- installed separately via `oh-my-claude-sisyphus`

Do not mix omc and omx in the same workspace; use distinct cmux workspaces per delegatee kind.

## Quick recipe (TL;DR)

```bash
# 1. derive claude's pane from env and add a sibling codex tab
CLAUDE_SURFACE="${CMUX_SURFACE_ID:-surface:20}"
CLAUDE_PANE=$(cmux tree 2>&1 | awk -v s="$CLAUDE_SURFACE" '/pane pane:/ {match($0,/pane:[0-9]+/); p=substr($0,RSTART,RLENGTH)} $0 ~ s {print p; exit}')
CODEX_SURFACE=$(cmux new-surface --pane "$CLAUDE_PANE" --type terminal 2>&1 | awk '{print $2}')
CODEX_WS=$(cmux tree 2>&1 | awk -v s="$CODEX_SURFACE" '/workspace workspace:/ {match($0,/workspace:[0-9]+/); w=substr($0,RSTART,RLENGTH)} $0 ~ s {print w; exit}')

# 2. write prompt (self-contained, no hidden context)
PROMPT_FILE=$(mktemp /tmp/codex-prompt.XXXXXX.md)
cat > "$PROMPT_FILE" <<'EOF'
<full task brief here>
EOF

# 3. register the session so we can list/report/close/resume later
OMX_TASK_LABEL="ship two dotagents PRs"
jq -cn --arg ts "$(date +%s)" --arg ws "$CODEX_WS" --arg sf "$CODEX_SURFACE" \
       --arg pn "$CLAUDE_PANE" --arg label "$OMX_TASK_LABEL" --arg cwd "$(pwd)" \
       --arg prompt "$PROMPT_FILE" --arg sha "$(git rev-parse HEAD 2>/dev/null || echo '')" \
  '{start:$ts|tonumber, workspace:$ws, surface:$sf, pane:$pn, label:$label, cwd:$cwd, prompt_file:$prompt, sha_before:$sha}' \
  >> /tmp/omx-sessions.jsonl

# 4. launch with plain omx (NOT cmux omx). git identity is inherited from ~/.zshrc exports.
cmux send --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" "cd $(pwd) && omx --yolo --high \"\$(cat $PROMPT_FILE)\""
cmux send-key --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" Enter

# 5. poll progress
omx hud --json | jq '{turns: .hudNotify.turn_count, last: .metrics.last_activity, out: (.hudNotify.last_agent_output // "" | .[0:160])}'

# 6. report results (see "Session lifecycle → Report" for the full block)
git status -sb && git diff --stat
cmux capture-pane --workspace "$CODEX_WS" --surface "$CODEX_SURFACE" --lines 60

# 7. close cleanly when done (see "Session lifecycle → Close" for the full block)
#    cmux send-key ... C-c C-c; cmux close-surface ...; prune /tmp/omx-sessions.jsonl
```
