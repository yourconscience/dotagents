# First-party dotagents integrations

Evaluated: 2026-05-17 15:01 +04

## Decision

Do not build dotagents integration as an Amp workspace plugin or any other committed agent-specific project directory. Dotagents should own the reusable primitives directly and render or patch each agent through adapters.

Canonical repo surfaces:

- `skills/` for shared workflows and invocation guidance.
- `agents/*.yaml` for repo-owned subagent roles.
- `memory/hooks/` for lifecycle hooks.
- `memory/` for knowledge sync and memory tooling.
- `skills/dotagents/dotagents.yaml` for managed MCP and sync targets.

Agent-specific plugin systems are reference material, not the source of truth.

## Models to study

- Cline: strongest separation of plugins, skills, hooks, rules, and subagents. Start here for vocabulary and boundaries.
  - https://github.com/cline/cline/blob/main/docs/customization/plugins.mdx
  - https://github.com/cline/cline/blob/main/docs/customization/skills.mdx
  - https://github.com/cline/cline/blob/main/docs/customization/hooks.mdx
  - https://github.com/cline/cline/blob/main/docs/features/subagents.mdx
- OpenCode: best repo-owned layout reference. Study how it keeps workflow artifacts in repo files while narrowing runtime plugin APIs.
  - https://github.com/anomalyco/opencode/blob/dev/packages/core/src/plugin.ts
  - https://github.com/anomalyco/opencode/blob/dev/packages/core/src/plugin/boot.ts
  - https://github.com/anomalyco/opencode/blob/dev/.opencode/agent/triage.md
  - https://github.com/anomalyco/opencode/blob/dev/.opencode/skills/effect/SKILL.md
- Claude Code: best packaging reference for bundling commands, agents, skills, hooks, and MCP.
  - https://github.com/anthropics/claude-code/blob/main/plugins/README.md
  - https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/README.md
  - https://github.com/anthropics/claude-code/blob/main/plugins/hookify/hooks/hooks.json
- Continue: useful for source-controlled team automation and local/global layering.
  - https://github.com/continuedev/continue/blob/main/core/config/loadLocalAssistants.ts
  - https://github.com/continuedev/continue/blob/main/docs/agents/overview.mdx
  - https://github.com/continuedev/continue/blob/main/docs/checks/generating-checks.mdx

## Avoid as foundation

- Roo Code: archived/shut down, useful only as historical reference.
  - https://github.com/RooCodeInc/Roo-Code/blob/main/README.md
- Aider: strong CLI, but extensibility is mostly commands, modes, and flags rather than a first-class plugin system.
  - https://github.com/Aider-AI/aider/blob/main/aider/commands.py
  - https://github.com/Aider-AI/aider/blob/main/aider/website/docs/usage/commands.md

## Design implications

- Keep packaging separate from runtime. A future dotagents bundle manifest can declare skills, hooks, memory sync, subagents, and MCP targets without becoming the runtime itself.
- Keep adapters explicit. Each agent should get targeted config patches or generated native files; no whole-config symlinks and no checked-in vendor runtime directories. Existing ignored local settings may still need patches when the agent gives them precedence.
- Keep hooks typed and narrow. Lifecycle hooks should be explicit scripts under `memory/hooks/` with clear input/output contracts.
- Keep memory inspectable. Prefer markdown and repo-owned sync rules over opaque global retrieval.
- Keep invocation explicit. Do not rely only on model auto-discovery for important skills or subagent routing.
