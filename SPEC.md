# SPEC: Harness Architecture Refactor

## Goal

Replace scattered agent-specific switch/if dispatch in the dotagents CLI with a unified, table-driven harness abstraction. Each coding agent (claude-code, codex, amp, hermes, droid, omp, future agents) becomes a harness descriptor with declared capabilities. All feature dispatch (skills, MCP, roles, hooks, setup, doctor checks) reads from harness descriptors instead of hardcoded agent names.

Adding a new agent should require: one YAML entry in `dotagents.yaml`, one Go harness descriptor, zero edits to generic feature code.

## Non-goals

- Do not change `dotagents.yaml` schema for users (existing configs must parse identically).
- Do not split the single-binary CLI into multiple packages.
- Do not add runtime plugin loading or dynamic registration.
- Do not change external behavior of `status`, `sync`, `setup`, `doctor` commands.
- Do not change MCP/hook config format handlers (JSON/YAML/TOML parsers stay as-is).

## Current state

20+ dispatch points across 10 files hard-code agent names. Audit summary:

| Capability | Pattern today | Dispatch points | Agents involved |
|---|---|---|---|
| Skills sync | if-chain in inspect.go, sync.go, setup.go | 3 | amp, hermes (config-driven) vs rest (symlink) |
| MCP | Table-driven `mcpTargets` map | 1 table, 6 entries | all 6 |
| Agent roles | switch in agents.go | 1 switch + 3 render fns | claude-code, codex, droid |
| Hooks | Table-driven `hookTargets` map | 1 table, 2 entries | claude-code, hermes |
| Plugins | `agentPluginSurfaceSupport()` switch | 1 switch | all 6 |
| Setup patches | switch in setup.go | 1 switch + 2 patch fns | amp, hermes |
| Root instructions | droid-only if in inspect.go | 1 if | droid |
| Report integration | if-chain in report.go | 2 ifs | amp, hermes |
| Doctor checks | hermes-only functions | 2 functions | hermes |
| Commit-msg trailers | test map | 1 map | all 6 + openclaw |

## Harness descriptor

```go
type Harness struct {
    // Identity (from dotagents.yaml, unchanged)
    Name      string
    Detect    string
    SkillRoot string
    AgentRoot string

    // Capability: skills
    Skills SkillsCapability // symlink | config-driven | none

    // Capability: MCP
    MCP *MCPCapability // nil = no MCP support

    // Capability: agent roles
    Roles *RolesCapability // nil = no role rendering

    // Capability: hooks
    Hooks *HooksCapability // nil = no hook support

    // Capability: setup
    Setup SetupFunc // nil = no config patching needed

    // Capability: root instructions
    RootInstructions *RootInstructionsCapability // nil = none

    // Capability: plugins
    PluginSurfaces map[string]bool

    // Display
    IntegrationNote string // shown in status report, empty = none

    // Doctor
    DoctorChecks []DoctorCheck

    // Commit-msg trailer pattern
    TrailerExample string
}
```

### SkillsCapability

```go
type SkillsKind int
const (
    SkillsSymlink     SkillsKind = iota // symlink into SkillRoot
    SkillsConfigDriven                   // config-driven (amp, hermes)
)

type SkillsCapability struct {
    Kind    SkillsKind
    Inspect func(agent, expected, ...) (agentReport, error) // only for config-driven
    // Symlink agents use the generic inspectAgent/applyAgentSync path
}
```

### MCPCapability

Reuse existing `mcpTarget` struct — it's already the right shape. Just move it into the harness.

### RolesCapability

```go
type RolesCapability struct {
    Extension string // ".md", ".toml"
    Render    func(role agentRole) string
}
```

### HooksCapability

Reuse existing `hookTarget` struct.

### RootInstructionsCapability

```go
type RootInstructionsCapability struct {
    Path     func(home string) string           // e.g. ~/.factory/AGENTS.md
    Expected func(home string) string           // e.g. ~/.agents/AGENTS.md
    Inspect  func(report *agentReport, home string) error
}
```

### DoctorCheck

```go
type DoctorCheck struct {
    Name string
    Run  func(repoRoot, home string, cfg config) checkResult
}
```

## Harness registry

```go
var harnesses = map[string]*Harness{
    "claude-code": { ... },
    "codex":       { ... },
    "amp":         { ... },
    "hermes":      { ... },
    "droid":       { ... },
    "omp":         { ... },
}
```

Built at init time. `loadConfig` resolves each `agentConfig` to its harness. Unknown agent names with no harness entry get a minimal default (symlink skills, no MCP/roles/hooks).

## Migration plan

### Phase 1: Introduce Harness type + registry

- Define `Harness` and capability types in new file `harness.go`.
- Build the `harnesses` map with current behavior encoded declaratively.
- Do NOT change any callers yet.

### Phase 2: Skills dispatch

- Replace `inspectAgent()` if-chain with `harness.Skills.Kind` dispatch.
- Replace `applyAgentSync()` skip for amp/hermes with `Skills.Kind == SkillsConfigDriven` check.
- Replace `patchAgentConfig()` switch with `harness.Setup` call.
- Remove `inspectAmpAgent`, `inspectHermesAgent` from inspect.go; move to harness closures.

### Phase 3: Agent roles dispatch

- Replace `renderAgentRole()` switch with `harness.Roles` lookup.
- Move `renderClaudeAgentRole`, `renderCodexAgentRole`, `renderDroidAgentRole` into harness closures or keep as named functions referenced by harness.

### Phase 4: Hooks, plugins, report, doctor

- Replace `hookTargets` map access with `harness.Hooks`.
- Replace `agentPluginSurfaceSupport()` switch with `harness.PluginSurfaces`.
- Replace `printReport()` agent-name ifs with `harness.IntegrationNote`.
- Replace hermes-specific doctor checks with `harness.DoctorChecks`.
- Replace root instructions droid-only code with `harness.RootInstructions`.

### Phase 5: Delete constants, clean up

- Remove `agentAmp`, `agentClaudeCode`, etc. constants (or keep as string literals in harness registry only).
- Remove all switch/if-chains that dispatched on agent name.
- Update commit-msg trailer test to derive from harness registry.

## Acceptance tests

- All existing tests in `go test ./cmd/dotagents/` pass unchanged.
- `dotagents status` output is byte-identical before and after (for detected agents).
- `dotagents sync --agents=omp` creates skill symlinks in `~/.omp/agent/skills/`.
- `dotagents doctor` produces identical check results.
- Adding a hypothetical new agent requires only `harness.go` entry + `dotagents.yaml` entry.

## Constraints

- Go, single binary, no new dependencies.
- Backward-compatible `dotagents.yaml` schema (version 1).
- Each phase must leave tests green; no big-bang rewrite.

## Codebase notes

- `mcp.go` `mcpTargets` and `hooks.go` `hookTargets` are already the right pattern. They become fields in the harness.
- `agents.go` role rendering is the trickiest — 3 distinct formats with agent-specific model mapping and tool mapping. The harness should reference the render function, not try to generalize the formats.
- `plugins.go` `agentPluginSurfaceSupport` is a perfect candidate for a data field.
- `setup.go` amp/hermes patching functions are ~100 lines each with complex JSON/YAML logic. Keep them as named functions, just referenced from harness.

## Risks / open questions

- **Config-driven skill inspection** (amp, hermes) has deeply different logic from symlink inspection. The harness must allow full override of the inspect path, not just a flag.
- **Doctor checks** are currently global, not per-agent. Moving hermes checks to the harness means the doctor runner must iterate harnesses, not maintain its own list.
- **Commit-msg trailers**: The test map `commitMsgHookTrailerCases` is test-only and references agent names. It could be derived from harness entries, but the trailer pattern isn't strictly a harness concern — it's a git hook concern. Keep the test map for now, validate it against the harness registry.

## Outcome / Deviations

(To be filled after implementation)
