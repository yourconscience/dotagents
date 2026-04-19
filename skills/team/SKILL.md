---
name: team
description: Spawn a coordinated team of Claude Code agent teammates in cmux panes. Use when the user asks to "create a team", "launch teammates", or "spawn a team". Handles teammate model selection, task creation, and cmux integration.
---

# team

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

## Common team patterns

### Architect + Builder + Reviewer (3-agent)

The most validated pattern (Anthropic's own feature-dev plugin uses this structure).

- **architect** (sonnet/opus): designs the plan, reviews the builder's output
- **builder** (sonnet): implements exactly what the architect specifies
- **reviewer** (sonnet): checks output against spec, catches bugs

### Research team

- **researcher-a** (sonnet): investigates approach A
- **researcher-b** (sonnet): investigates approach B
- **synthesizer** (sonnet): combines findings into recommendation

### Parallel implementation

- **frontend** (sonnet): UI changes
- **backend** (sonnet): API changes
- **tester** (sonnet): writes and runs tests

## Anti-patterns

- Spawning teammates for tasks that take < 5 minutes (subagent is cheaper)
- More than 5 teammates (coordination overhead exceeds benefit)
- Teammates without clear task boundaries (they'll step on each other's files)
- Using opus for every teammate (burns quota fast; sonnet handles most work)
- Not setting model explicitly (teammates inherit parent model, which may be opus)
