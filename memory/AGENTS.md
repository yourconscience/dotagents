# Memory layout

Model durable capabilities, not vendors or individual agents. `memory/` is the
stable abstraction for session digests, vault sync, and search indexing.

Keep provider names such as `memsearch` as implementation details in code or
configuration, not as top-level directories. Prefer generic lifecycle hooks
(`session-start`, `stop`, `session-end`) and dispatch on hook payload shape when
agent integrations differ.

Avoid per-agent directories unless an integration boundary forces separate
input schemas, config formats, or approval flows. If agent-specific parsing is
needed, isolate it in small library modules rather than duplicating hook
implementations.
