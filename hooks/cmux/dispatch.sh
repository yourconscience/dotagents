#!/usr/bin/env sh
# Dispatches dotagents-managed cmux agent hooks.
# Keep hook installation in dotagents; runtime handling stays in the cmux CLI.

agent="${1:-}"
if [ -z "$agent" ]; then
  printf '{}\n'
  exit 0
fi

case "$agent" in
  feed)
    source_agent=""
    prev=""
    for arg in "$@"; do
      if [ "$prev" = "--source" ]; then
        source_agent="$arg"
        break
      fi
      case "$arg" in
        --source=*)
          source_agent=${arg#--source=}
          break
          ;;
      esac
      prev="$arg"
    done
    agent="$source_agent"
    ;;
esac

case "$agent" in
  codex) disable_var="CMUX_CODEX_HOOKS_DISABLED" ;;
  factory|droid) disable_var="CMUX_FACTORY_HOOKS_DISABLED" ;;
  hermes-agent|hermes) disable_var="CMUX_HERMES_AGENT_HOOKS_DISABLED" ;;
  claude) disable_var="CMUX_CLAUDE_HOOKS_DISABLED" ;;
  *) disable_var="" ;;
esac

if [ -z "${CMUX_SURFACE_ID:-}" ]; then
  printf '{}\n'
  exit 0
fi

if [ -n "$disable_var" ]; then
  eval "disabled=\${$disable_var:-}"
  if [ "$disabled" = "1" ]; then
    printf '{}\n'
    exit 0
  fi
fi

cmux_cli="${CMUX_BUNDLED_CLI_PATH:-}"
if [ -z "$cmux_cli" ] || [ ! -x "$cmux_cli" ]; then
  cmux_cli=$(command -v cmux 2>/dev/null || true)
fi

if [ -z "$cmux_cli" ]; then
  printf '{}\n'
  exit 0
fi

if [ -n "${CMUX_SOCKET_PATH:-}" ]; then
  "$cmux_cli" --socket "$CMUX_SOCKET_PATH" hooks "$@" || printf '{}\n'
else
  "$cmux_cli" hooks "$@" || printf '{}\n'
fi
