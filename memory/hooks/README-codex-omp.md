# Codex and OMP/Pi memory capture

Claude Code capture is wired automatically by `dotagents setup`. Codex and
OMP/Pi have no session-end hook that dotagents installs for them, so their
automatic capture is opt-in and wired manually. Both route through the same
`session-end.sh` dispatcher, which classifies a Codex/OMP payload and writes the
local `basic_memory` digest — the same digest Claude produces.

Reminder for both harnesses: automatic digests are a safety net, not a
substitute for deliberate capture. Record durable cross-harness facts explicitly
with `rem add -src codex "<fact>"` / `rem add -src omp "<fact>"`.

## Codex

Codex emits Claude-compatible Stop/SessionEnd payloads (with `transcript_path`
and `model`). Add a hook to `~/.codex/hooks.json` on `SessionEnd` (preferred) or
`Stop`, setting `DOTAGENTS_MEMORY_SOURCE=codex` so the digest is labelled and so
the `Stop`-wired path captures rather than passing through:

```json
{
  "type": "command",
  "command": "DOTAGENTS_MEMORY_SOURCE=codex ~/.agents/memory/hooks/session-end.sh",
  "timeout": 30
}
```

Reminder: also `rem add -src codex "<fact>"` for facts worth promoting.

## OMP / Pi

OMP/Pi has no session-end hook but supports extensions that fire on
`agent_end`. Install the shipped extension:

```sh
cp ~/.agents/memory/hooks/omp-memory.ts ~/.omp/agent/extensions/
```

It pipes an `agent: "omp"` payload into `session-end.sh` on every `agent_end`.
Set `DOTAGENTS_MEMORY_HOOKS` if your dotagents checkout is not at `~/.agents`.

Reminder: also `rem add -src omp "<fact>"` for facts worth promoting.
