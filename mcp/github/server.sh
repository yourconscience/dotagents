#!/usr/bin/env bash
set -euo pipefail

if ! command -v github-mcp-server >/dev/null 2>&1; then
  printf 'github MCP server unavailable: install github-mcp-server and set GITHUB_PERSONAL_ACCESS_TOKEN\n' >&2
  exit 127
fi

exec github-mcp-server stdio --toolsets repos,pull_requests,actions
