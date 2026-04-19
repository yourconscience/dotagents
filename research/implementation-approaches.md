# Implementation approaches for non-trivial coding tasks

Comparison of methods available for implementing features, refactors, and multi-file changes when working with AI coding agents. Evaluated April 2026, updated with community signal from HN, Reddit, X.com, and Simon Willison's agentic engineering patterns guide.

## Methods

### 1. omx - delegate to Codex/GPT-5

oh-my-codex provides multiple execution modes, all running Codex in detached `omx-*` tmux sessions.

#### omx exec (non-interactive, preferred from Claude Code)

```bash
cd <repo> && omx exec "$(cat prompt.md)"
```

Blocks until Codex finishes, returns output. No tmux session management needed. Best for single-shot implementation delegated from within a Claude session. The simplest way to get a second-model perspective without leaving your conversation.

#### omx hidden (background delegation)

```bash
cd <repo> && PATH="/opt/homebrew/bin:$PATH" omx --yolo --high "$(cat prompt.md)"
```

Codex runs in detached tmux session. Claude's Bash returns immediately. Monitor via `omx hud --json`, `tmux capture-pane`, or `git diff --stat`. The "ralph" pattern: serial task loop where each iteration reads a PRD, finds the next unchecked task, implements with TDD, marks complete, commits, exits. Next iteration picks up from the file.

#### omx interactive (visible cmux surface)

Same launch as hidden, plus a cmux sibling surface running `tmux attach`. User can watch Codex work in real-time or intervene. Requires cmux environment (Claude Code desktop). Useful when the task is fragile or you want to catch drift early.

#### omx resume (multi-turn)

Send follow-up prompts to an idle Codex session via `tmux paste-buffer`. Enables iterative refinement without relaunching. If the session is gone, `omx resume` picks a prior session ID and starts fresh.

#### omx team (multiple Codex workers)

Multiple Codex instances coordinated through omx. Less tested than single-instance modes. The cmux shim breaks `show-options` for team workers; use `PATH="/opt/homebrew/bin:$PATH"` to bypass.

**Strengths:**
- Truly parallel: runs while you do other things or sleep
- Fresh context: no prior conversation cruft
- GPT-5 is strong at sustained multi-file implementation
- Multiple modes from blocking (`exec`) to fully autonomous (hidden `--yolo`)
- Cheap relative to Opus for implementation grunt work
- Can resume sessions for iterative refinement

**Weaknesses:**
- No interactive clarification in yolo mode (by design)
- Can go off-rails on ambiguous specs without feedback
- Limited to what Codex can infer from repo + prompt
- No structured verification loop - you review after
- cmux PATH workaround needed for interactive/team modes

**Best for:** Well-scoped implementation (hidden/exec), background work while away (hidden), quick second opinion (exec), iterative refinement (resume). The `omx exec` mode is the lowest-friction delegation path available.

### 2. ultrapack (/up:make) - staged pipeline

Claude Code plugin (btseytlin/ultrapack) providing an opinionated SDLC pipeline: Design -> Plan -> Execute -> Verify -> Review. State lives in `docs/tasks/<slug>.md`.

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

**How it works:** Create a team, define tasks (TaskCreate), spawn named agents, assign work, receive reports. Teammates are visible in tmux. Coordination through SendMessage and shared task list.

**Strengths:**
- True parallelism: multiple agents working simultaneously
- Visible: each agent has its own tmux pane
- Flexible role assignment: researcher, coder, reviewer, whatever you need
- Good for divide-and-conquer when subtasks are independent
- Teammates can message each other for peer coordination

**Weaknesses:**
- Expensive: N agents * context = N * cost
- Coordination overhead: team lead spends tokens managing, not doing
- Polling problem: agents must actively check TaskList (no push notifications). Agents that poll lazily get zero tasks while aggressive pollers claim everything (observed in ralph-vs-teams Reddit comparison: Gamma agent completed zero tasks in Run 1)
- ~14% duplicate work observed when agents race for the same task
- Merge conflicts if teammates touch overlapping files
- Overkill for sequential work (agents idle waiting on dependencies)

**Best for:** Research + implementation in parallel. Evaluation tasks. Multi-component features where subtasks are truly independent. Start with 3-5 teammates (community consensus). NOT for sequential multi-step implementation.

### 4. Agent subagents - inline delegation

Spawn background or foreground subagents within a single conversation. No tmux panes, no team coordination overhead.

**Invocation:** `Agent` tool with a prompt. Can run in background or foreground.

**How it works:** Main agent delegates a subtask to a subagent. Subagent works with its own context window, returns results to the parent. Parent synthesizes and continues.

Simon Willison identifies three subagent patterns:
- **Sequential exploration**: parent pauses, subagent investigates, returns findings (e.g., Explore agent tracing a Django view)
- **Parallel execution**: multiple subagents run simultaneously on independent tasks (e.g., "find and update all affected templates")
- **Specialist roles**: custom system prompts for code review, test running, debugging

**Strengths:**
- Lightweight: no team setup, no task management
- Protects main context from verbose tool output (Willison: "the main value is preserving that valuable root context")
- Good for parallel research (3-4 searches at once)
- Can use specialized agent types and cheaper models (Haiku for exploration)

**Weaknesses:**
- Not visible to user (no tmux pane)
- Limited context: subagent doesn't see conversation history
- No persistent state between subagent calls
- Can't do sustained multi-file work well - context too small
- Don't fragment unnecessarily: parent handles most work fine if tokens allow

**Best for:** Research, codebase exploration, short focused tasks. NOT for sustained implementation.

### 5. PLAN.md orchestration - file-driven autonomous runs

Emerging community pattern: write orchestration instructions into a plan file, then launch a fresh agent that reads and executes it. No framework, no plugin.

**How it works:** A PLAN.md file contains both the milestone spec and orchestration instructions (how to research, implement, validate, and consult other agents for second opinions). Launch with: "Please read @PLAN-M3.md. I'd like you to implement this plan, per the instructions in that file." Agent runs 2-4 hours autonomously.

Key technique from Reddit (u/ljw1004, 61 upvotes): orchestration instructions tell the agent to make four separate requests to different agents (Claude/Codex) for second opinions on KISS, codebase style, correctness, and milestone goals. If the second opinion has objections, the agent must address them before proceeding.

**Strengths:**
- Zero tooling: just markdown files and a fresh agent
- Portable: works with any agent that reads files (Claude Code, Codex, OpenCode)
- Transparent: the plan IS the orchestration, readable by humans and agents
- Proven 3-4 hour autonomous runs in both greenfield and brownfield codebases
- Cross-model validation built in (ask Codex to review Claude's output and vice versa)

**Weaknesses:**
- Requires upfront effort writing good orchestration instructions
- No structured resume (agent crashes = start over, unlike ultrapack's task-file stages)
- Validation quality depends entirely on what you put in the file
- Scales to one milestone at a time

**Best for:** Medium-to-large features where you want autonomy without framework overhead. The pragmatic middle ground between "just do it" and ultrapack's full pipeline.

### 6. Direct implementation - just do it

Single agent implements directly in conversation. No delegation, no pipeline.

**Strengths:**
- Zero overhead
- Full conversation context available
- Immediate feedback loop with user

**Weaknesses:**
- Context window is the ceiling
- Sequential only
- Loses state if session ends mid-task

**Best for:** Tasks under ~30 minutes. Bug fixes. Config changes. Single-file features.

## Comparison

| Method | Parallelism | Autonomy | Rigor | Cost | Portability | Setup |
|--------|------------|----------|-------|------|-------------|-------|
| omx exec | None (blocks) | Medium | Low | Low (GPT-5) | Codex | None |
| omx hidden/ralph | High | High | Low | Low (GPT-5) | Codex | Minimal |
| omx interactive | High | High | Low | Low (GPT-5) | Codex + cmux | Minimal |
| omx team | Very high | High | Low | Medium (N * GPT-5) | Codex | Moderate |
| ultrapack | Medium (per-phase) | Medium-High | Very high | High (Opus) | Claude Code only | Plugin install |
| TeamCreate | High | Medium | Flexible | Very high (N * Opus) | Claude Code only | Team + tasks |
| Subagents | Medium | Low | None built-in | Medium | Claude Code only | None |
| PLAN.md pattern | None | Very high | Medium (self-directed) | Low-Medium | Any agent | File authoring |
| Direct | None | Low | None built-in | Low | Any agent | None |

## Recommendations by task type

| Task | Recommended | Runner-up |
|------|------------|-----------|
| Well-scoped feature (clear spec) | omx hidden/ralph | omx exec |
| Ambiguous feature (needs design) | Direct + spec skill | PLAN.md pattern |
| Quick second opinion on approach | omx exec | Subagent |
| Multi-component parallel work | TeamCreate | omx team |
| Research / evaluation | TeamCreate or subagents | Direct |
| Bug fix | Direct | omx exec |
| Large refactor (many files) | omx hidden/ralph | PLAN.md pattern |
| Background work while away | omx hidden/ralph | - |
| Needs audit trail / resumability | ultrapack | PLAN.md pattern |
| 3-4 hour autonomous feature build | PLAN.md pattern | omx hidden |
| Cross-model validation | PLAN.md pattern (built-in) | omx exec (as reviewer) |

## Community signal

### HN

Several orchestration tools emerging (Optio, Corral, Stoneforge, OpenTiger) but none with significant traction (all under 100 points). Common architecture: planner -> workers -> reviewer, parallel git worktrees, test-suite-as-guardrail. Key skepticism: "a human in the loop is crucial for task planning" and agents disabling tests or removing validation when stuck.

Optio (88 points, 59 comments) drew the most discussion. Proponents argue the value is "directing at a higher level" with clear acceptance criteria. Critics note agents make "increasingly creative excuses for why the test is wrong" after repeated failures.

### Reddit r/ClaudeAI

**Ralph bash loop vs Agent Teams** (113 upvotes): Same 14-task PRD, same model. Teams was 3.8x faster (10 min vs 38 min) but had coordination bugs: one agent got zero tasks due to lazy polling, ~14% duplicate work from race conditions. Code quality was identical (98% coverage, all tests passing). Conclusion: teams win on speed for independent tasks, ralph wins on reliability and cost.

**3-4 hour orchestration runs** (61 upvotes): PLAN.md pattern with cross-model second opinions. Key insight: "I don't read the plans that the AI writes. Their audience is other AIs who review the plan and other AIs who implement the plan."

**8 parallel Claude Code agents** (6 upvotes, 17 comments): Mixed results. Works when tasks are truly independent; breaks down when dependencies exist.

### X.com

@rohanvarma (Codex team): "Most people just aren't asking agents to do ambitious enough work that actually requires true delegation. The people who are delegating use Codex." Framing pair programming as a local optimization; delegation as the longer-term pattern.

@nummanali: Codex <-> Claude Code communication via cmux, using AGENTS.md to codify the message protocol. Cross-agent orchestration through shared terminal infrastructure.

### Simon Willison

Key frameworks from his agentic engineering patterns guide:
- Parallel agents work best for research, proof-of-concepts, and well-specified work
- "The natural bottleneck is how fast I can review the results" - parallelization only helps if review overhead stays manageable
- Subagents' main value is preserving root context, not speed
- Fresh checkouts to /tmp preferred over git worktrees for isolation
- Cheaper/faster models (Haiku) for exploration subagents; expensive models for implementation

## Current setup (dotagents)

Supported today: all omx modes (exec, hidden, interactive, resume, team), subagents, TeamCreate, direct, PLAN.md pattern (just files). All available via skills or native tools.

Not installed: ultrapack. Can coexist if installed (`/plugin marketplace add btseytlin/ultrapack`). The useful concepts (brevity, no-default rule, step-back) have been adopted directly into AGENTS.md and skills/.

## Open questions

- Is ultrapack's overhead worth it for medium tasks, or only large ones? Needs real usage.
- Can omx exec as a reviewer + PLAN.md achieve similar rigor to ultrapack without the pipeline weight?
- TeamCreate polling problem: at what team size does coordination overhead exceed the parallelism benefit?
- omx team: how stable is multi-worker Codex for real tasks? Limited testing so far.
- PLAN.md pattern: what's the right granularity for milestone files? Too big = context overflow, too small = overhead.

## Sources

- [Optio - Orchestrate AI coding agents](https://news.ycombinator.com/item?id=47520220) (HN, 88 points)
- [Ralph bash loop vs Agent Teams comparison](https://www.reddit.com/r/ClaudeAI/comments/1r24f5f/) (Reddit, 113 upvotes)
- [Orchestration: exact prompts for 3-4 hour runs](https://www.reddit.com/r/ClaudeAI/comments/1s0nktx/) (Reddit, 61 upvotes)
- [Embracing the parallel coding agent lifestyle](https://simonwillison.net/2025/Oct/5/parallel-coding-agents/) (Simon Willison)
- [Subagents - Agentic Engineering Patterns](https://simonwillison.net/guides/agentic-engineering-patterns/subagents/) (Simon Willison)
- [Claude Code Agent Teams docs](https://code.claude.com/docs/en/agent-teams) (Anthropic)
- [Cursor 3 agent-first interface](https://creati.ai/ai-news/2026-04-06/cursor-3-agent-first-interface-claude-code-codex/) (Creati.ai)
- [btseytlin/ultrapack](https://github.com/btseytlin/ultrapack) (GitHub)
