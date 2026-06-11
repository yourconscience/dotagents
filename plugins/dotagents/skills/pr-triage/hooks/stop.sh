#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
tool_dir="$skill_dir/tools/pr-triage"

# Resolve the session's working directory so the tool inspects the repo the
# session is in, not this checkout. Hook stdin carries {"cwd": ...}; fall back
# to the hook's own cwd.
session_cwd="$PWD"
if [ ! -t 0 ]; then
  payload="$(cat || true)"
  if [ -n "$payload" ] && command -v jq >/dev/null 2>&1; then
    payload_cwd="$(printf '%s' "$payload" | jq -r '.cwd // empty' 2>/dev/null || true)"
    if [ -n "$payload_cwd" ] && [ -d "$payload_cwd" ]; then
      session_cwd="$payload_cwd"
    fi
  fi
fi

# Only inspect PRs when the session is inside a git work tree; otherwise stay quiet.
if ! git -C "$session_cwd" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  printf '{"continue":true,"suppressOutput":true,"message":"pr-triage hook skipped: not a git repository"}\n'
  exit 0
fi

if [ -d "$tool_dir" ] && [ -f "$tool_dir/go.mod" ]; then
  export PR_TRIAGE_PWD="$session_cwd"
  cd "$tool_dir" && exec go run . hook stop
fi

printf '{"continue":true,"suppressOutput":true,"message":"pr-triage hook skipped: tool directory missing"}\n'
