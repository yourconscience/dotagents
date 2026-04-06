# Plan

`dotagents` is intended to become the canonical shared authored layer for my local AI-agent setup.

## V1 Goals

- Keep one private repo for shared authored rules and skills.
- Avoid per-tool duplication for content that can remain universal.
- Keep the current migration low-risk by staging and reviewing before cutover.

## Near-Term Next Steps

- Replace `~/.agents` with a symlink to `~/Workspace/dotagents` after final review.
- Verify Claude Code, Codex, and OpenCode still resolve shared skills and `AGENTS.md` through the same canonical path.
- Keep private `~/.agents/contexts/` outside this repo for now to avoid mixing durable private memory with a repo that may evolve toward broader sharing later.

## Structural Principles

- `dotagents` should stay the shared authored source of truth.
- System and vendor assets should live in tool-native locations only.
- Shared skills should be written once and reused across Claude Code, Codex, and OpenCode whenever possible.
- Tool-specific wrappers or command launchers should stay outside the repo unless they become clearly worth centralizing.

## Possible V2 Directions

- Add a lightweight validation or sync helper if manual symlink maintenance becomes annoying.
- Add a generated-view or adapter layer only if shared skills stop fitting multiple tools cleanly.
- Explore a split between broadly shareable skills and more private personal operating guidance, while still keeping one local canonical entrypoint.
- Add higher-level shared assets later if they prove durable, such as reusable commands, hooks, or templates.

## Non-Goals For Now

- No per-agent authored skill trees.
- No attempt to mirror every tool's native config surface inside this repo.
- No automation layer just for its own sake.

The bar for adding more structure is simple: it must reduce real maintenance pain, not create a second system to maintain.
