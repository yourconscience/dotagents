# Team Architecture Patterns

Decision tree for choosing the right pattern. Start here, then read the pattern section that matches.

```
Is the work sequential (each step depends on the last)?
  Yes -> Pipeline

Is the work parallelizable with a merge step?
  Yes -> Fan-out/Fan-in

Do you need different specialists for different input types?
  Yes -> Expert Pool

Is there a create-then-verify cycle?
  Yes -> Producer-Reviewer

Does a central agent need to manage dynamic state and reassign work?
  Yes -> Supervisor

Is the problem recursive or hierarchically decomposable?
  Yes -> Hierarchical Delegation
```

## Pipeline

Sequential handoff. Each agent's output feeds the next agent's input.

```
[Analyst] -> [Designer] -> [Builder] -> [Reviewer]
```

**When to use:** Strong sequential dependencies where later stages cannot start without earlier output. Software lifecycle, content pipelines, ETL.

**Tradeoffs:** Bottleneck at slowest stage. Limited parallelism. Best when stages are roughly equal duration.

**Team vs subagent:** Subagents usually suffice since communication is one-directional. Use a team only if stages need to negotiate (e.g., designer pushes back on analyst's requirements).

## Fan-out/Fan-in

Parallel independent work followed by synthesis.

```
         +-> [Agent A] --+
[Split]  +-> [Agent B] --+--> [Merge]
         +-> [Agent C] --+
```

**When to use:** Independent subtasks that can run simultaneously. Research across multiple sources, parallel file processing, multi-perspective analysis.

**Tradeoffs:** Merge step complexity scales with fan-out width. Conflicting findings need resolution strategy.

**Team vs subagent:** Subagents for truly independent work (each produces a report, merger combines). Team when agents should share discoveries mid-flight (one researcher's finding changes another's search direction).

## Expert Pool

Route work to the right specialist based on input characteristics.

```
[Router] --> [Expert: Frontend]
         --> [Expert: Backend]
         --> [Expert: Database]
         --> [Expert: Security]
```

**When to use:** Heterogeneous work items requiring different domain expertise. Ticket triage, multi-language codebases, mixed-format processing.

**Tradeoffs:** Router accuracy is critical. Misrouted work wastes an expert's context. Keep the routing logic simple and explicit.

**Team vs subagent:** Subagents when routing is clean and one-shot. Team when experts need to consult each other (frontend asks backend about API shape).

## Producer-Reviewer

One agent creates, another validates. Iterates until quality bar is met.

```
[Producer] <--> [Reviewer]
     |               |
     v               v
  draft v1     feedback v1
  draft v2     "approved"
```

**When to use:** Any task where quality depends on independent verification. Code generation, document drafting, security review, test writing.

**Tradeoffs:** Can loop if quality bar is ambiguous. Set a max iteration count (typically 2-3). Reviewer must have concrete acceptance criteria.

**Team vs subagent:** Team strongly preferred. The back-and-forth messaging is the whole point. Subagent pattern would require the parent to relay feedback manually.

## Supervisor

Central agent maintains state, distributes work dynamically, handles failures.

```
         +-> [Worker A]
[Super]  +-> [Worker B]
         +-> [Worker C]
         (reassigns on failure)
```

**When to use:** Work where task assignment depends on runtime state. Load balancing, retry-on-failure, dynamic priority adjustment. Long-running jobs where some workers may stall.

**Tradeoffs:** Supervisor is a single point of failure and a context bottleneck. Keep supervisor logic thin: dispatch and monitor, don't process.

**Team vs subagent:** Team when workers report partial progress and supervisor adapts. Subagents when supervisor just collects final results.

## Hierarchical Delegation

Top-level agent decomposes the problem, delegates sub-problems recursively.

```
[Lead]
  +-> [Module A Lead]
  |     +-> [A Worker 1]
  |     +-> [A Worker 2]
  +-> [Module B Lead]
        +-> [B Worker 1]
```

**When to use:** Large problems that decompose into semi-independent sub-problems. Monorepo refactors, multi-service changes, large document generation.

**Tradeoffs:** Depth increases latency and token cost. Keep to 2 levels max in practice. Each level must produce a clear contract (interface, spec) for the level below.

**Team vs subagent:** Hybrid is natural here. Top level is a team (leads coordinate). Each lead uses subagents for their workers (no cross-module communication needed).

## Combining patterns

Real workflows often combine patterns:

- **Fan-out + Producer-Reviewer:** Parallel research agents, each reviewed independently, then merged.
- **Pipeline + Fan-out:** Sequential phases where one phase fans out internally.
- **Supervisor + Expert Pool:** Supervisor routes to experts and handles re-routing on failure.

Name the combined pattern in the orchestrator so future readers understand the intent.
