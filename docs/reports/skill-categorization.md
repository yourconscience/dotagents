# Skill Categorization Report

Date: 2026-06-12  
Branch: ghostex-skill-categorization  
Scope: dotagents skills (16) + Ghostex bundled skills (5)

---

## Summary Table

| Skill | Category | Implementation Surface | Ghostex Overlap | Recommendation |
|---|---|---|---|---|
| **cmux** | Orchestration / multiplexer | cmux CLI | High: ghostex-agent-orchestration (pane mgmt), ghostex-browser-use (browser panes) | deprecate-behind-trial |
| **tmux** | Orchestration / multiplexer | tmux CLI | Partial: ghostex-agent-orchestration wraps zmx (tmux-compatible); overlap via agent workflow pattern | deprecate-behind-trial |
| **spawn** | Delegation / meta-orchestration | agent-agnostic (Claude Code teams section references cmux) | Partial: ghostex-agent-orchestration covers team creation; spawn is broader (Hermes, Codex, Droid) | parameterize for Ghostex (add gx section) |
| **remote-access** | Remote access / mobile bridge | REST bridge + takopi (Telegram) | Partial: Ghostex has no takopi or mobile bridge; computer-use for macOS is different | keep as-is (complementary) |
| **dotagents** | Infra / config management | Go CLI (plain CLI) | None | keep as-is |
| **grill-me** | Writing / process | none (prompt-only) | None | keep as-is |
| **gws** | Integration / CLI wrapper | gws CLI | None | keep as-is |
| **humanizer** | Writing / voice | none (prompt-only) | None | keep as-is |
| **jobs** | Workflow / process | gws CLI + LinkedIn MCP + WebSearch | None | keep as-is |
| **pr-triage** | Workflow / process | gh CLI + Go tool + GitHub MCP | ghostex-manage-beads covers bead-based review tracking (no overlap on PRs proper) | keep as-is |
| **repo-eval** | Research | gh CLI + WebSearch + x-cli | None | keep as-is |
| **spec** | Writing / process | none (prompt-only) | None | keep as-is |
| **tech-search** | Research | WebSearch + x-cli + rdt + discord-cli | None | keep as-is |
| **tg** | Integration / CLI wrapper | tg CLI (Telethon daemon) | None | keep as-is |
| **x-cli** | Integration / CLI wrapper | x-cli binary | None | keep as-is |
| **x-sim** | Research / writing | x-cli + Go tool | None | keep as-is |

---

## Ghostex Bundled Skills

| Skill | Category | Surface | Relation to dotagents skills |
|---|---|---|---|
| ghostex-agent-orchestration | Orchestration / multiplexer | ghostex / zmx CLI | Replaces cmux and tmux for Ghostex-managed panes |
| ghostex-browser-use | Integration / browser | CEF DevTools MCP | Replaces cmux browser surfaces |
| ghostex-computer-use | Integration / native macOS | cua-driver CLI | No direct dotagents equivalent |
| ghostex-generate-title | Utility | ghostex CLI | No dotagents equivalent |
| ghostex-manage-beads | Workflow / process | gx bd CLI | Parallel to pr-triage (different object: beads vs PRs) |

---

## Orchestration Cluster: Detailed Analysis

### Skills in the cluster

- **tmux**: Generic pane management reference. Detects CMUX env vars and routes to cmux if present. Pure tmux when no cmux.
- **cmux**: Full cmux pane + browser surface management. cmux-specific launchers. Hooks routed through dotagents.
- **spawn**: Agent delegation meta-skill. Surface-agnostic in framework; Claude Code teams section explicitly depends on cmux (`cmux claude-teams`).
- **remote-access**: Two modes: takopi (Telegram -> Pi) and Mac bridge (REST). Not a multiplexer itself; uses no tmux or cmux directly.

### Ghostex overlap

`ghostex-agent-orchestration` covers the same conceptual surface as cmux + tmux for Ghostex-managed terminals:

| cmux capability | tmux capability | ghostex-agent-orchestration equivalent |
|---|---|---|
| `cmux new-workspace`, `cmux new-pane` | `tmux new-session`, `tmux new-window` | `ghostex create-session`, `ghostex create-agent` |
| `cmux send`, `cmux send-key` | `tmux send-keys` | `ghostex send-message`, `ghostex send-text`, `ghostex send-key` |
| `cmux read-screen` | `tmux capture-pane` | `ghostex read-text` |
| `cmux browser ...` | (none) | `$ghostex-browser-use` |
| `cmux list-panes`, `cmux tree` | `tmux list-panes` | `ghostex sessions --json`, `ghostex state` |
| `cmux focus-pane` | `tmux select-pane` | `ghostex focus` |
| `cmux close-surface` | `tmux kill-pane` | `ghostex kill` |

spawn's Claude Code teams section (`TeamCreate + cmux claude-teams`) does not have a direct Ghostex equivalent yet: `ghostex create-agent` launches configured agent panes but the Claude Code multi-agent team protocol (`TeamCreate`, `TaskCreate`, `SendMessage`) is harness-specific and lives at a layer above the multiplexer.

remote-access is complementary: it covers Telegram-based mobile access to Pi and a REST bridge for Hermes-to-Mac communication. Ghostex has no takopi or bridge equivalent.

### Options evaluated

**Option A: Keep separate per-surface skills**

Pros: clean separation, no conditional logic inside a skill, easy to deprecate one surface.  
Cons: agents currently must detect the surface and pick between tmux and cmux manually (tmux.md delegates this with env-var detection), and adding a third surface (ghostex) means a third parallel skill that agents must choose between.

**Option B: Merge into one "mux" skill with per-surface sections**

Pros: single lookup for agents, env-var routing logic centralized, detection logic is the same in all three skills.  
Cons: the merged skill grows large; detection-routing becomes critical to get right; harder to deprecate one surface cleanly; Ghostex bundled skills already cover ghostex-agent-orchestration independently, so merging would duplicate or conflict with the bundled skill.

**Option C: spawn delegates to surface skills**

Pros: spawn already decides delegation mode; surface detection fits its pattern.  
Cons: spawn is about agent delegation (what work to hand off and to whom), not multiplexer command syntax. Mixing those concerns into spawn bloats it. Surface-selection is a pre-condition for spawn, not spawn's job.

---

## Recommendation

### For the trial period (cmux and tmux)

Keep cmux and tmux as separate skills, both marked as **deprecate-behind-trial**. The tmux skill already has the right routing logic (checks for CMUX env vars first). When the Ghostex trial concludes:

- If Ghostex replaces cmux: remove cmux, update tmux detection to add a ghostex env-var check above the CMUX check, and note that ghostex-agent-orchestration is the authoritative reference.
- If cmux stays: no change needed.

Do not merge cmux and tmux now. The trial outcome should drive which surface becomes primary.

### For spawn

Add a **Ghostex execution section** to spawn alongside the existing Claude Code, Hermes, Codex, and Droid sections. The teams subsection currently says "Requires CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 (set by cmux claude-teams)". When running under Ghostex, `ghostex create-agent` is the equivalent pane launcher. The TeamCreate protocol itself is Claude Code-level and does not change.

This is a targeted addition, not a merge. spawn stays the decision layer; the new Ghostex section gives the execution syntax for agents running inside Ghostex.

### For remote-access

Keep as-is. It covers a different dimension (mobile/remote reach) that Ghostex does not address. takopi and the Mac bridge are complementary to any multiplexer surface.

### Summary

- **cmux**: deprecate-behind-trial (overlap with ghostex-agent-orchestration + ghostex-browser-use is high)
- **tmux**: deprecate-behind-trial (overlap with ghostex-agent-orchestration via zmx is partial but covers the core use case)
- **spawn**: parameterize for Ghostex -- add a `## Ghostex execution` section mirroring the cmux teams launch pattern
- **remote-access**: keep as-is -- no Ghostex overlap on mobile/bridge surface

---

## Evidence Notes

All findings are based on direct reading of SKILL.md files. No speculation about undocumented Ghostex CLI behavior.

- dotagents skills: `~/.agents/skills/*/SKILL.md`
- Ghostex bundled skills: `/Applications/Ghostex.app/Contents/Resources/CLI/skills/*/SKILL.md`
- ghostex-agent-orchestration explicitly states it wraps zmx ("Ghostex uses zmx under the hood for this") confirming the tmux-layer overlap
- ghostex-browser-use is a CEF DevTools MCP bridge, a direct replacement for cmux browser surfaces
- ghostex-computer-use delegates to cua-driver; no dotagents equivalent exists
- ghostex-generate-title and ghostex-manage-beads have no dotagents equivalents
