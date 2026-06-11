#!/usr/bin/env bash
set -euo pipefail

skill_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
tool_dir="$skill_dir/tools/pr-triage"

if [ -d "$tool_dir" ] && [ -f "$tool_dir/go.mod" ]; then
  cd "$tool_dir" && exec go run . hook stop
fi

printf '{"continue":true,"suppressOutput":true,"message":"pr-triage hook skipped: tool directory missing"}\n'
