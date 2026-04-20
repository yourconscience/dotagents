---
name: spawn
description: Spawn a coordinated team of Claude Code agent teammates in cmux panes. Use when the user asks to "create a team", "launch teammates", "spawn a team", or "/spawn". Handles teammate model selection, task creation, and cmux integration.
---

# spawn

Spawn and manage Claude Code agent teams with proper model routing and cmux integration.

## When to use teammates vs subagents

| | Subagents (Agent tool) | Teammates (TeamCreate) |
|---|---|---|
| Context | Own window, results return to caller | Own window, fully independent |
| Communication | One-way: report back to main only | Bi-directional: teammates message each other |
| Coordination | Main agent manages all work | Shared task list, self-coordination |
| Visibility | Invisible to user (task list only) | Visible cmux/tmux split panes |
| Token cost | Lower (single context) | Higher (each is a full Claude instance) |
| Best for | Focused tasks where only the result matters | Work requiring inter-agent discussion |

**Use subagents when:**
- Task is self-contained and produces a summary (research, search, analysis)
- You want to protect main context from verbose output
- Cost matters more than coordination
- Tasks are independent (no cross-referencing needed)

**Use teammates when:**
- User explicitly asks for a team
- Agents need to challenge each other's findings
- Parallel work on different parts of the same system (architect + implementer)
- Cross-layer changes where teammates own different files/modules
- Long-running work where the user wants visibility

Default to subagents unless the user asks for a team or the work genuinely benefits from inter-agent coordination.

## Delegation modes

Three ways to spawn work. Pick based on the task shape, not habit.

| Need | Mode | Entry point |
|---|---|---|
| Focused task where only the result matters | Subagent (Agent tool) | Built-in, no setup |
| Coordinated team with shared tasks and visibility | Claude Code teammates (TeamCreate + cmux) | This skill |
| Second-model perspective or GPT-5 reasoning on a substantial task | Codex via oh-my-codex | `/omx` skill |

**When omx makes sense over subagents or teammates:**
- You want a different model's take (GPT-5 vs Claude) on architecture, review, or a tricky bug.
- The task is large and self-contained enough to hand off completely: multi-file refactor, deep research report, long test/debug loop.
- You want parallel work without burning Claude context or token budget on it.

**When omx is a bad fit:**
- Task needs Claude's current conversation context (omx gets a fresh prompt, no shared state).
- Sub-2-minute work (omx session overhead isn't worth it).
- Tight coordination loop where agents need to message each other (use teammates instead).

omx requires the `omx` CLI installed and working. If unavailable, fall back to subagents or teammates. See `/omx` skill for the full operational runbook.

## Prerequisites

For visible panes, the session must be started via `cmux claude-teams` (sets `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` and provides a tmux shim). Check:

```bash
echo "$CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"
# Should be "1"
```

If not in cmux, teammates still work but spawn as invisible background agents (like subagents with messaging).

## Model selection

Cost-aware defaults. Never let teammates inherit an expensive parent model unintentionally.

| Role complexity | Model | Effort | Use for |
|---|---|---|---|
| Complex design/architecture | opus | high | architect, lead reviewer, complex refactors |
| Standard implementation | sonnet | high | builder, researcher, most tasks |
| Simple/mechanical | haiku | - | linting, formatting, simple lookups |

Set model per teammate using the `model` parameter on the Agent tool:

```
Agent({
  name: "architect",
  team_name: "my-team",
  model: "sonnet",
  prompt: "..."
})
```

Or reference a reusable agent definition (from `~/.claude/agents/` or `.claude/agents/`):

```
Agent({
  name: "architect",
  team_name: "my-team",
  subagent_type: "architect",  # references agents/architect.md
  prompt: "project-specific context here"
})
```

Agent definitions set their own model via frontmatter, so the spawn call doesn't need to repeat it.

## Workflow

1. **Create team**: Use TeamCreate with a descriptive name.

```
TeamCreate({ team_name: "feature-x", description: "..." })
```

2. **Create tasks**: Use TaskCreate for each work item. Set dependencies with TaskUpdate if needed.

3. **Spawn teammates**: Use Agent tool with `team_name` and `model` parameters. Spawn all independent teammates in a single message (parallel launch).

4. **Assign tasks**: Use TaskUpdate to set `owner` on each task matching a teammate name.

5. **Monitor**: Teammates send messages automatically when done. Don't poll.

6. **Shutdown**: When all tasks complete, send shutdown to each teammate:

```
SendMessage({ to: "architect", message: { type: "shutdown_request" } })
```

## Reusable agent definitions

Store commonly-used agent roles in `~/.claude/agents/` (user-level) or `.claude/agents/` (project-level). These are markdown files with YAML frontmatter:

```markdown
---
name: architect
description: Designs system architecture and reviews technical plans
model: sonnet
effort: high
tools: Read, Glob, Grep, Bash, Write, Edit
color: blue
---

You are a senior software architect...
```

See the `agents/` directory in this repository for maintained role definitions. Sync them to `~/.claude/agents/` via the dotagents tool.

## Team architecture patterns

Six patterns for structuring agent teams. Quick reference below; full decision tree and tradeoff analysis in `references/patterns.md`.

| Pattern | Shape | Best for |
|---|---|---|
| Pipeline | A -> B -> C | Sequential dependencies (build lifecycle, ETL) |
| Fan-out/Fan-in | Split -> parallel -> merge | Independent subtasks (multi-source research) |
| Expert Pool | Router -> specialist | Heterogeneous work items (multi-language, mixed formats) |
| Producer-Reviewer | Create <-> validate loop | Quality-gated output (code gen, docs, security review) |
| Supervisor | Central dispatch + workers | Dynamic state, retry-on-failure, load balancing |
| Hierarchical | Lead -> sub-leads -> workers | Large decomposable problems (monorepo refactors) |

### Common compositions

**Architect + Builder + Reviewer** (producer-reviewer): The most validated pattern. architect designs, builder implements, reviewer verifies against spec.

**Research team** (fan-out/fan-in): Multiple researchers investigate in parallel, synthesizer merges findings.

**Parallel implementation** (fan-out/fan-in + producer-reviewer): frontend/backend/tester work in parallel, each reviewed independently.

## Artifact management

When a team produces intermediate files, use a `_workspace/{team_name}/` directory in the project root (and add `_workspace/` to `.gitignore`).

**Naming convention:** `{phase}_{agent}_{artifact}.{ext}`

```
_workspace/{team_name}/
  01_researcher_findings.md
  02_architect_design.md
  03_builder_implementation_notes.md
  04_reviewer_report.md
```

- Intermediate artifacts stay in `_workspace/` for audit trails.
- Final deliverables go to user-specified paths.
- When re-running a team, clear `_workspace_prev/{team_name}/` if it exists, then move `_workspace/{team_name}/` there before starting.

## Anti-patterns

- Spawning teammates for tasks that take < 5 minutes (subagent is cheaper)
- More than 5 teammates (coordination overhead exceeds benefit)
- Teammates without clear task boundaries (they'll step on each other's files)
- Using opus for every teammate (burns quota fast; sonnet handles most work)
- Not setting model explicitly (teammates inherit parent model, which may be opus)

## Multi-agent in Codex

Codex does not have a "team" primitive equivalent to Claude Code's TeamCreate + cmux panes.

**What Codex does have:**
- Native subagents: when `multi_agent = true` in `~/.codex/config.toml`, Codex can spawn child agents within a session. These run in the same session context, not separate panes.
- oh-my-codex adds a `team-executor` agent (`~/.codex/agents/team-executor.toml`) that coordinates parallel work by delegating to named subagents and collecting results.
- Agent definitions live in `~/.codex/agents/*.toml`. See `agents/` in this repo for maintained role definitions and their compat blocks.

**Spawning a subagent in Codex (within a session):**

```
// Codex native subagent spawn (multi_agent feature must be enabled)
// The orchestrating agent calls this pattern in its reasoning:
// "spawn researcher subagent to investigate X, report findings here"
// Codex routes this to the agent definition matching the name
```

**Limitations vs Claude Code teams:**
- No shared task list across Codex sessions
- No bi-directional messaging between independent sessions
- No cmux pane visibility
- Coordination is single-session only; parallel independent sessions require external orchestration (e.g., omx for tmux-based multi-session work)

**Recommended pattern for Codex multi-agent work:**
Use oh-my-codex's `$team` skill or the `team-executor` agent, which handles orchestration within a single Codex session. For truly parallel independent work, use omx to spawn separate Codex sessions in tmux panes.

## Multi-agent in Hermes Agent

Hermes uses `delegate_task` for subagent spawning. No named agent roles - subagents get a goal and context.

**Delegation:**

```python
# single subagent
delegate_task(goal="Review this PR for security issues", context="...", toolsets=["file"])

# parallel (up to 3 concurrent)
delegate_task(tasks=[
  {goal: "Research approach A", context: "...", toolsets: ["web", "file"]},
  {goal: "Research approach B", context: "...", toolsets: ["web", "file"]}
])
```

**Constraints:**
- Max depth: 2 (children cannot delegate further)
- Max concurrency: 3 parallel subagents
- Blocked toolsets for subagents: `delegation`, `clarify`, `memory`, `send_message`
- Subagents start fresh - no parent context bleeds through

**Subagent model override** (in `~/.hermes/config.yaml`):

```yaml
delegation:
  model: "provider/model"
  provider: "openrouter"
  max_iterations: 50
  default_toolsets: ["terminal", "file", "web"]
```

**Skill integration:** Add `~/.agents/skills` to `skills.external_dirs` in config.yaml to make dotagents skills available.

**Limitations vs Claude Code teams:**
- No shared task list
- No bi-directional messaging between subagents
- No named role definitions (pass role context via goal/context params)
- No cmux/tmux pane visibility
- Subagents report back to parent only; no peer-to-peer coordination

## Multi-agent in OpenClaw

OpenClaw uses `sessions_spawn` to delegate to named agents. Agents are workspace instances, not role files.

**Named agents** are configured in `~/.openclaw/openclaw.json`:

```json
{
  "agents": {
    "list": [
      { "id": "main", "default": true, "workspace": "~/.openclaw/workspace" },
      { "id": "researcher", "workspace": "~/.openclaw/workspace-research" }
    ]
  }
}
```

**Spawning a subagent:**

```
sessions_spawn(task="...", agentId="researcher", model="...", mode="run")
```

**Access control:** `agents.list[].subagents.allowAgents` is an allowlist of spawnable agentIds. Default allows only self.

**Skill integration:** `~/.agents/skills/` is already in OpenClaw's skill load path (precedence 3). No config needed.

**Limitations vs Claude Code teams:**
- No shared task list
- No cmux/tmux pane visibility
- No bi-directional messaging between agents
- Subagent persona injection is broken (issue #50263) - subagents run with spawner's context
- Named agents require separate workspace directories with their own SOUL.md/AGENTS.md

