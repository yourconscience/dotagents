---
name: spawn
description: "Decide how to delegate work to subagents or teams. Agent-agnostic: covers Claude Code, Hermes, and Codex."
---

# spawn

Decide how to split work across agents. This skill is about the decision, not the tool syntax: pick the right delegation mode, then follow the agent-specific execution section.

## Decision framework

### Task sizing

| Task size | Time | Examples | Delegation mode |
|---|---|---|---|
| Small | < 5 min | lookup, lint check, simple search | Do it yourself |
| Medium | 5-30 min | focused research, single-file refactor, test writing | Subagent |
| Large | 30+ min | multi-file refactor, deep research, architecture design | Subagent or team |
| Cross-model | Any | second opinion, GPT vs Claude comparison | omx (see `/omx` skill) |

### When to use subagents

- Task is self-contained and produces a summary
- You want to protect main context from verbose output
- Tasks are independent (no cross-referencing needed)
- Default choice for most delegation

### When to use a team (Claude Code only)

- User explicitly asks for a team
- Agents need to challenge each other's findings
- Parallel work on interrelated parts of the same system
- Long-running work where the user wants visible panes

### When to use omx

- You want a different model's perspective (GPT-5 vs Claude)
- Task is large enough to justify session overhead (> 5 min)
- You want parallel work without burning your own context/quota
- See `/omx` skill for full runbook

## Model selection

Cost-aware defaults regardless of which agent is primary.

| Role complexity | Model | Use for |
|---|---|---|
| Complex design/architecture | opus / gpt-5 | architect, lead reviewer, complex refactors |
| Standard implementation | sonnet / gpt-5 | builder, researcher, most tasks |
| Simple/mechanical | haiku / gpt-4.1-mini | linting, formatting, simple lookups |

## Team architecture patterns

| Pattern | Shape | Best for |
|---|---|---|
| Pipeline | A -> B -> C | Sequential dependencies |
| Fan-out/Fan-in | Split -> parallel -> merge | Independent subtasks |
| Producer-Reviewer | Create <-> validate | Quality-gated output |
| Expert Pool | Router -> specialist | Heterogeneous work items |

Most common: **Architect + Builder + Reviewer** (producer-reviewer).

## Artifact management

When a team produces intermediate files, use `_workspace/{team_name}/` in the project root (add `_workspace/` to `.gitignore`).

Naming: `{phase}_{agent}_{artifact}.{ext}` (e.g., `01_researcher_findings.md`).

## Anti-patterns

- Spawning subagents for < 5 minute tasks
- More than 5 concurrent agents (coordination overhead dominates)
- Agents without clear file/task boundaries (they step on each other)
- Not setting model explicitly (inherits expensive parent model)

---

## Claude Code execution

### Subagents (Agent tool)

```
Agent({
  name: "researcher",
  model: "sonnet",
  subagent_type: "researcher",
  prompt: "self-contained task description"
})
```

- Results return to caller's context
- Spawn independent subagents in a single message for parallel execution
- Use `subagent_type` to reference reusable definitions from `~/.claude/agents/` or `.claude/agents/`

### Teams (TeamCreate + cmux)

Requires `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` (set by `cmux claude-teams`).

1. `TeamCreate({ team_name: "feature-x" })`
2. `TaskCreate` for each work item
3. `Agent` with `team_name` and `model` per teammate (parallel launch in one message)
4. `TaskUpdate` to assign owners
5. Wait for messages; don't poll
6. `SendMessage({ to: "name", message: { type: "shutdown_request" } })` when done

### Reusable agent definitions

Store in `~/.claude/agents/*.md` with YAML frontmatter (name, model, tools, description). Sync via `dotagents sync`.

---

## Hermes execution

Hermes is an orchestrator first. Default to delegating substantial coding work rather than doing it yourself.

### Delegation routing

| Task type | Delegate to | How |
|---|---|---|
| Coding (features, refactors, fixes) | Claude Code | `claude -p "..."` via terminal |
| Cross-model perspective, large codegen | Codex (GPT-5) | `omx exec "..."` via terminal |
| Parallel coding on same repo | Hermes worktree | `hermes -w -p "..."` via terminal |
| In-process research, analysis, small tasks | Hermes subagent | `delegate_task(...)` |
| Conversation, planning, Q&A | Do it yourself | Direct response |

### When to delegate vs do it yourself

Delegate when:
- Task involves >5 min of coding tool calls
- Task benefits from a stronger model (Claude Opus/Sonnet, GPT-5)
- Multiple independent coding tasks can run in parallel

Do it yourself when:
- Quick lookups, single-file reads, simple questions
- Research using your own skills (web, tech-search, memory)
- Planning and conversation
- The overhead of delegation exceeds the task itself

Always state why you are delegating or doing it yourself.

### Cross-agent delegation (from Hermes terminal)

```bash
# Claude Code - non-interactive, blocks until done
claude -p "self-contained task prompt" --allowedTools "Edit,Read,Write,Bash"

# Codex via omx - non-interactive, blocks until done
cd /path/to/repo && omx exec "self-contained task prompt"

# Hermes worktree - parallel isolated instance
hermes -w -p "self-contained task prompt"
```

Write self-contained prompts. Delegates have no access to your conversation context. After delegation completes, verify the result before reporting success.

### In-process subagents (delegate_task)

For lighter tasks that don't need a different model or repo isolation:

```python
# single
delegate_task(goal="Review this PR for security issues", context="...", toolsets=["file"])

# parallel (up to 3 concurrent)
delegate_task(tasks=[
  {goal: "Research approach A", context: "...", toolsets: ["web", "file"]},
  {goal: "Research approach B", context: "...", toolsets: ["web", "file"]}
])
```

Constraints:
- Max depth: 2 (children cannot delegate further)
- Max concurrency: 3 parallel subagents
- Blocked toolsets for subagents: `delegation`, `clarify`, `memory`, `send_message`
- Subagents start fresh, no parent context

### Subagent model override

In `~/.hermes/config.yaml`:

```yaml
delegation:
  model: "provider/model"
  provider: "openrouter"
  max_iterations: 50
  default_toolsets: ["terminal", "file", "web"]
```

### Skill integration

Add `~/.agents/skills` to `skills.external_dirs` in `~/.hermes/config.yaml`.

---

## Codex execution

### Native subagents

Enable `multi_agent = true` in `~/.codex/config.toml`. Codex spawns child agents within the same session context.

### omx team

For parallel independent work, use `omx team` to spawn separate Codex sessions in tmux panes. Or use the `team-executor` agent from oh-my-codex.

### Limitations

- No shared task list across sessions
- No bi-directional messaging between independent sessions
- Coordination is single-session only
