# Implementation approaches for non-trivial coding tasks

Comparison of methods available for implementing features, refactors, and multi-file changes when working with AI coding agents. Evaluated April 2026.

## Methods

### 1. omx (ralph) - delegate to Codex/GPT-5

Spawn a detached tmux session running oh-my-codex. Codex works autonomously in its own context with full repo access.

**Invocation:** `/omx ralph --yolo --high` with a detailed prompt, or via the `omx` skill.

**How it works:** Codex gets a self-contained prompt (goal, relevant files, constraints), runs in `omx-*` tmux session, commits incrementally. You monitor via `omx hud`, `git log`, or `tmux attach`.

**Strengths:**
- Truly parallel: runs while you do other things or sleep
- Fresh context: no prior conversation cruft
- GPT-5 is strong at sustained multi-file implementation
- Visible tmux pane - can watch or interrupt
- Cheap relative to Opus for implementation grunt work

**Weaknesses:**
- No interactive clarification once launched (yolo mode)
- Can go off-rails on ambiguous specs without feedback
- Limited to what Codex can infer from repo + prompt
- No structured verification loop - you review after

**Best for:** Well-scoped implementation tasks where SPEC.md or clear instructions exist. Background delegation while you're away.

### 2. ultrapack (/up:make) - staged pipeline

Claude Code plugin providing an opinionated SDLC pipeline: Design -> Plan -> Execute -> Verify -> Review. State lives in `docs/tasks/<slug>.md`.

**Invocation:** `/up:make <description>` or `/up:make handsoff <description>`

**How it works:** Opus orchestrates. Each stage is a skill. Execute dispatches fresh Sonnet implementers per plan phase. Independent reviewer checks the diff without seeing rationale. Task file enables resume from any stage.

**Strengths:**
- Structured rigor: invariants, plan-as-contract, deviation tracking
- Resumable: any fresh agent reads the task file and continues
- Independent review catches real bugs (confidence >= 80 filter)
- Hands-off mode reduces prompts while maintaining safety
- TDD integration when applicable

**Weaknesses:**
- Heavy for small/medium tasks - full pipeline is 5+ stages
- Claude-Code-only (not portable to Codex/OpenCode)
- Burns significant Opus context on orchestration overhead
- Prescriptive: one workflow shape, no escape hatch for tasks that don't fit
- Brand new (created 2026-04-17), single author, unproven at scale

**Best for:** Complex features where design ambiguity is high, invariants matter, and you want auditability. Not worth it for tasks under ~1 hour of work.

### 3. TeamCreate - visible parallel agents

Spawn multiple Claude Code teammates in split tmux panes. Each gets its own context, tools, and task assignment. Team lead coordinates via SendMessage.

**Invocation:** `TeamCreate` tool, then `Agent` with `team_name` parameter to spawn teammates.

**How it works:** Create a team, define tasks (TaskCreate), spawn named agents (architect, implementer, tester, etc.), assign work, receive reports. Teammates are visible in tmux - user can watch. Coordination through SendMessage and shared task list.

**Strengths:**
- True parallelism: multiple agents working simultaneously
- Visible: each agent has its own tmux pane
- Flexible role assignment: researcher, coder, reviewer, whatever you need
- Good for divide-and-conquer when subtasks are independent
- Teammates can message each other for peer coordination

**Weaknesses:**
- Expensive: N agents * context = N * cost
- Coordination overhead: team lead spends tokens managing, not doing
- Merge conflicts if teammates touch overlapping files
- Overkill for sequential work (agents idle waiting on dependencies)
- Team lifecycle management (create, assign, shutdown) adds ceremony

**Best for:** Research + implementation in parallel. Evaluation tasks (this session). Multi-component features where frontend/backend/tests are independent. NOT for sequential multi-step implementation.

### 4. Agent subagents - inline delegation

Spawn background or foreground subagents within a single conversation. No tmux panes, no team coordination overhead.

**Invocation:** `Agent` tool with a prompt. Can run in background or foreground.

**How it works:** Main agent delegates a subtask (research, search, write code) to a subagent. Subagent works with its own context window, returns results to the parent. Parent synthesizes and continues.

**Strengths:**
- Lightweight: no team setup, no task management
- Protects main context from verbose tool output
- Good for parallel research (3-4 searches at once)
- Subagent results land directly in parent conversation
- Can use specialized agent types (Explore, Plan, etc.)

**Weaknesses:**
- Not visible to user (no tmux pane)
- Limited context: subagent doesn't see conversation history
- Parent must wait for results (or use background mode)
- No persistent state between subagent calls
- Can't do sustained multi-file work well - context too small

**Best for:** Research, codebase exploration, short focused tasks (write one file, search for something, analyze a diff). NOT for sustained implementation.

### 5. Direct implementation - just do it

Single agent (you) implements directly in conversation. No delegation, no pipeline.

**Invocation:** Just... work.

**Strengths:**
- Zero overhead
- Full conversation context available
- Immediate feedback loop with user
- Best for small/medium tasks that fit in one session

**Weaknesses:**
- Context window is the ceiling
- Sequential only
- No structured verification beyond what you remember to do
- Loses state if session ends mid-task

**Best for:** Tasks under ~30 minutes. Bug fixes. Config changes. Single-file features.

## Comparison

| Method | Parallelism | Autonomy | Rigor | Cost | Portability | Setup |
|--------|------------|----------|-------|------|-------------|-------|
| omx (ralph) | High | High | Low (no review loop) | Low (GPT-5) | Codex only | Minimal |
| ultrapack | Medium (per-phase) | Medium-High | Very high | High (Opus orchestration) | Claude Code only | Plugin install |
| TeamCreate | High | Medium | Flexible | Very high (N agents) | Claude Code only | Team + tasks |
| Subagents | Medium | Low | None built-in | Medium | Claude Code only | None |
| Direct | None | Low | None built-in | Low | Any agent | None |

## Recommendations by task type

| Task | Recommended | Runner-up |
|------|------------|-----------|
| Well-scoped feature (clear spec exists) | omx ralph | Direct |
| Ambiguous feature (needs design discussion) | Direct + spec skill | ultrapack |
| Multi-component parallel work | TeamCreate | omx (multiple sessions) |
| Research / evaluation | TeamCreate or subagents | Direct |
| Bug fix | Direct | omx (if you're away) |
| Large refactor (many files, one pattern) | omx ralph | Direct |
| Background work while away | omx ralph | - |
| Needs audit trail / resumability | ultrapack | Direct + spec |

## Current setup (dotagents)

Supported today: omx, subagents, TeamCreate, direct. All available via skills or native Claude Code tools.

Not installed: ultrapack. Can coexist if installed (`/plugin marketplace add btseytlin/ultrapack`). The useful concepts (brevity, no-default rule, step-back) have been adopted directly into AGENTS.md and skills/ instead of depending on the plugin.

## Open questions

- Is ultrapack's overhead worth it for medium tasks, or only large ones? Needs real usage to answer.
- Can omx + SPEC.md achieve similar rigor to ultrapack without the pipeline weight?
- TeamCreate cost vs. value: at what task size does the coordination overhead pay for itself?
