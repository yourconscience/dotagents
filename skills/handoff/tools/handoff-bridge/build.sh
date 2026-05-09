#!/usr/bin/env bash
set -eu

tool_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
out="${1:-$HOME/.local/bin/handoff-bridge}"

mkdir -p "$(dirname "$out")"
go -C "$tool_dir" build -trimpath -ldflags='-s -w' -o "$out" .
chmod +x "$out"
