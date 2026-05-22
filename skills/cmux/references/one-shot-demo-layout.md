# One-Shot cmux Git Project Reference

## Standard 4-Pane Layout

```plaintext
Workspace: cmux-git-vitals-demo
+-----------+-----------+-----------+
|  Editor   | Git + Tests| Browser   |
|           | Terminal   | Preview   |
|           |            |           |
+-----------+-----------+-----------+
| README.md | Diff viewer|           |
+-----------+-----------+-----------+
```

## Makefile Golden Path

```makefile
.PHONY: setup demo test clean

setup:
	cp .env.template .env
	chmod +x scripts/*.sh tests/*.sh
	bash scripts/setup-hook.sh

demo: setup
	@echo "Starting server + cmux workspace..."
	@bash -c 'node src/server.js & echo $$! > /tmp/cmv.pid; sleep 2'
	@bash scripts/cmux-demo.sh

test:
	bash tests/smoke.sh

clean:
	-kill $$(cat /tmp/cmv.pid 2>/dev/null) 2>/dev/null || true
	-kill $$(lsof -t -i:3000) 2>/dev/null || true
	rm -f /tmp/cmv.pid
	git checkout -- .
```

## scripts/cmux-demo.sh (Workspace-Safe)

Parse the workspace handle from `new-workspace` output. Do NOT pass the workspace name string to later commands -- it causes `Invalid workspace handle: name`.

```bash
#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")/.." && pwd)"
WS="${CMUX_WORKSPACE:-cmux-git-vitals-demo}"
URL="${DEMO_BROWSER_URL:-http://127.0.0.1:3000}"

# Create workspace and capture numeric handle
WS_RES=$(cmux new-workspace --name "$WS" --cwd "$DIR" 2>/dev/null || true)
WS_HANDLE=$(echo "$WS_RES" | grep -o 'workspace:[0-9]*' | head -1)
[ -z "$WS_HANDLE" ] && WS_HANDLE=$(cmux current-workspace 2>/dev/null | head -1 || true)

WS_ARG=""
[ -n "$WS_HANDLE" ] && WS_ARG="--workspace $WS_HANDLE"

cmux new-pane --type terminal --direction down $WS_ARG --cmd "node src/server.js"
cmux new-pane --type terminal --direction right $WS_ARG --cmd "watch -n 2 'git log --oneline --graph -10'"
cmux new-pane --type browser --direction right $WS_ARG --url "$URL"
cmux new-pane --type markdown --direction down $WS_ARG --file README.md
cmux new-pane --type terminal --direction right $WS_ARG --cmd "git diff --color-words"
```

## Post-Commit Hook Pattern

```bash
#!/usr/bin/env bash
PORT="${PORT:-3000}"
curl -s -X POST "http://127.0.0.1:${PORT}/api/notify" || true
```

This hooks into the SSE `/api/events` stream to refresh all open dashboard tabs.

## Smoke Test Strategy

Assert that the dashboard at `http://127.0.0.1:3000/api/git` returns a JSON body containing the current HEAD hash:

```bash
HASH=$(git rev-parse --short HEAD)
BODY=$(curl -sf "http://127.0.0.1:3000/api/git")
echo "$BODY" | grep -q "$HASH"
```

## Pitfalls

### Empty Repo
`git log --oneline` throws "fatal: your current branch 'main' does not have any commits yet". Wrap all git `execSync` calls in try/catch and provide fallback values:

```javascript
try { branch = execSync('git branch --show-current').toString().trim(); } catch {}
try { log = execSync('git log --oneline -20').toString().trim(); } catch {}
if (!log) log = '(no commits yet)';
```

### Port Collision (EADDRINUSE)
The Makefile `clean` target must kill the server; `demo` should guard against a stale process:

```makefile
clean:
	-kill $$(cat /tmp/cmv.pid 2>/dev/null) 2>/dev/null || true
	-kill $$(lsof -t -i:3000) 2>/dev/null || true
	rm -f /tmp/cmv.pid
	git checkout -- .
```

Then run `make clean && make demo` rather than just `make demo`.

### Workspace Name vs Handle
`cmux new-workspace --name "my-demo"` returns `OK workspace:9`. Use `workspace:9` for `--workspace`, not the name. The name is display-only.

```
# WRONG
cmux new-pane --workspace "my-demo"
# Error: Invalid workspace handle: my-demo

# CORRECT
WS=$(cmux new-workspace --name "my-demo" --cwd "/repo")
cmux new-pane --workspace workspace:9
```

### Makefile `$$` Escaping
Inside a Makefile recipe, use `$$` to pass a single `$` to the shell. The `echo $!` trick to capture a PID requires `$$!` in the recipe body.

```makefile
demo:
	@bash -c 'node server.js & echo $$! > /tmp/pid; sleep 2'
```

### Shell scripts need `chmod +x`
`make setup` should explicitly chmod helper scripts before attempting to run them.
