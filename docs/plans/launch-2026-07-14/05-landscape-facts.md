# Landscape fact sheet — verified 2026-07-15

Sources: upstream READMEs/releases fetched 2026-07-15. Backs the README
Landscape table and HN comparison claims.

## rulesync (dyoshikawa/rulesync)
- 35 AI-tool rows in the supported-tools matrix (no "40+" claim upstream)
- Skills yes · MCP yes · Hooks yes · Subagents yes
- No lockfile/commit-pinning/audit for external skills (`rulesync fetch`
  imports without pinning)
- Install: npm, brew tap, single binary
- Scope: project + global modes
- Active: v10.1.0 released 2026-07-15; 1,234 stars

## ruler (intellectronica/ruler)
- 31 agents in the agent table
- Skills yes (experimental, default on) · MCP yes · Hooks no ·
  Subagents yes (experimental, default off)
- No pinning/audit
- Install: npm/npx only (Node >=20.19)
- Scope: project-level `.ruler/` with global fallback
- Active: v0.3.44 (2026-06-30), pushed 2026-07-11; 2,805 stars

## openskills (numman-ali/openskills)
- No numeric harness count; ~5 named tools (Claude Code, Cursor, Windsurf,
  Aider, Codex, "anything reading AGENTS.md")
- Skills yes (sole purpose) · MCP no (explicitly rejects) · Hooks no ·
  Subagents no
- Tracks source repo for re-fetch, but no pinned SHA/lockfile/integrity check
- Install: npm/npx
- Scope: project + user level
- **Stale: last commit 2026-01-18** (v1.5.0, 2026-01-17); 10,615 stars

## Emerging small competitors (all <100 stars, omission defensible)
- amtiYo/agents (78★) — `.agents` source of truth → Codex/Claude/Gemini/
  Cursor/Copilot/Antigravity; closest conceptual match
- dallay/agentsync (49★) — symlink-based config sync, claims ~31 agents
- yiftahb/agsync (9★) — skills+MCP+secrets sync for Claude Code/Cursor

No >1k-star newcomer found that the table omits.

## Changes applied from this research
- README landscape: rulesync "40+ broad" → "35 broad"; ruler "35+ broad" →
  "31 broad" (2026-07-15)
- HN draft: "cover 35–40 tools" → "cover ~30–35 tools"
- openskills staleness: usable rebuttal in HN comments, not in the table
