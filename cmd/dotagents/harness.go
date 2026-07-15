package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
)

// SkillsKind describes how an agent discovers skills from dotagents.
type SkillsKind int

const (
	// SkillsSymlink means dotagents creates symlinks in the agent's skill root.
	SkillsSymlink SkillsKind = iota
	// SkillsConfigDriven means the agent reads skills from a config-driven
	// shared path (e.g. Amp's amp.skills.path or Hermes' skills.external_dirs).
	// Inspect and setup are handled by custom functions on the harness.
	SkillsConfigDriven
)

// RolesCapability describes how agent roles are rendered for this harness.
type RolesCapability struct {
	Extension string                      // file extension including dot: ".md", ".toml"
	Render    func(role agentRole) string // render role to file content
}

// RootInstructionsCapability describes a root instructions symlink the agent needs.
type RootInstructionsCapability struct {
	Path     func(home string) string // symlink path, e.g. ~/.factory/AGENTS.md
	Expected func(home string) string // target path, e.g. ~/.agents/AGENTS.md
}

// DoctorCheck is a named health check contributed by a harness.
type DoctorCheck struct {
	Name string
	Run  func(repoRoot, home string, cfg config) checkResult
}

// InspectSkillsFunc is the signature for config-driven skill inspection.
// Only used when Skills == SkillsConfigDriven.
type InspectSkillsFunc func(agent agentConfig, expected map[string]string, agentsSkillRoot string, cfg config, home string) (agentReport, error)

// SetupFunc is the signature for agent-specific config patching during setup.
type SetupFunc func(home string, cfg config) (bool, error)

// Harness is the central descriptor for a coding agent integration.
// All feature dispatch reads capability fields instead of switching on agent names.
type Harness struct {
	// Detect validates an executable after the generic PATH lookup.
	// nil means the executable's presence is sufficient.
	Detect func(executable string) bool

	// Skills integration mode.
	Skills SkillsKind
	// InspectSkills is called instead of the generic symlink inspector
	// when Skills == SkillsConfigDriven.
	InspectSkills InspectSkillsFunc
	// Setup patches the agent's config during `dotagents setup`.
	// nil means no patching needed.
	Setup SetupFunc

	// MCP holds the MCP config target. nil = no MCP support.
	MCP *mcpTarget
	// Roles holds the agent role rendering capability. nil = no role support.
	Roles *RolesCapability
	// Hooks holds the hook config target. nil = no hook support.
	Hooks *hookTarget

	// PluginSurfaces declares which plugin surfaces this agent supports
	// beyond the baseline (skills, mcp, assets).
	PluginSurfaces map[string]bool

	// RootInstructions describes a root instructions symlink. nil = none.
	RootInstructions *RootInstructionsCapability

	// IntegrationNote is printed in status/sync reports for this agent.
	// Empty string = no note.
	IntegrationNote string

	// DoctorChecks are agent-specific health checks appended to `dotagents doctor`.
	DoctorChecks []DoctorCheck

	// TrailerExample is the Co-authored-by trailer this agent's bot produces.
	// Used by commit-msg hook tests to validate coverage.
	TrailerExample string
}

// harnesses maps agent name to its harness descriptor.
// Built at init time with current behavior encoded declaratively.
var harnesses map[string]*Harness
var harnessesOnce sync.Once

func initHarnesses() {
	harnesses = map[string]*Harness{
		"amp": {
			Skills: SkillsConfigDriven,
			InspectSkills: func(agent agentConfig, expected map[string]string, _ string, cfg config, home string) (agentReport, error) {
				return inspectAmpAgent(agent, expected, cfg, home)
			},
			Setup: func(home string, _ config) (bool, error) {
				return patchAmpConfig(home)
			},
			MCP: mcpTargetPtr(mcpTarget{
				agentName:  "amp",
				configPath: ampSettingsPath,
				inspect:    inspectJSONMCPServer,
				patch:      patchJSONMCPServer,
				read:       readJSONMCPServer,
				rootKey:    "amp.mcpServers",
			}),
			PluginSurfaces:  map[string]bool{},
			IntegrationNote: "config-driven via Amp settings -> amp.skills.path",
			TrailerExample:  "Co-authored-by: amp[bot] <amp[bot]@users.noreply.github.com>",
		},

		"claude-code": {
			Skills: SkillsSymlink,
			MCP: mcpTargetPtr(mcpTarget{
				agentName:  "claude-code",
				configPath: func(home string) string { return filepath.Join(home, ".claude.json") },
				inspect:    inspectJSONMCPServer,
				patch:      patchJSONMCPServer,
				read:       readClaudeMCPServer,
				rootKey:    "mcpServers",
				defaults:   map[string]interface{}{"type": "stdio"},
			}),
			Roles: &RolesCapability{Extension: ".md", Render: renderClaudeAgentRole},
			Hooks: &hookTarget{
				agentName: "claude-code",
				inspect:   inspectClaudeHook,
				patch:     patchClaudeHook,
			},
			PluginSurfaces: map[string]bool{
				pluginSurfaceAgents: true,
				pluginSurfaceHooks:  true,
				pluginSurfaceCmds:   true,
				pluginFormatClaude:  true,
			},
			TrailerExample: "Co-authored-by: claude[bot] <claude[bot]@users.noreply.github.com>",
		},

		"codex": {
			Skills: SkillsSymlink,
			MCP: mcpTargetPtr(mcpTarget{
				agentName:  "codex",
				configPath: func(home string) string { return filepath.Join(home, ".codex", "config.toml") },
				inspect:    inspectCodexMCPServer,
				patch:      patchCodexMCPServer,
				read:       readCodexMCPServer,
			}),
			Roles: &RolesCapability{Extension: ".toml", Render: renderCodexAgentRole},
			Hooks: &hookTarget{
				agentName: "codex",
				inspect:   inspectCodexHook,
				patch:     patchCodexHook,
			},
			PluginSurfaces: map[string]bool{
				pluginSurfaceAgents: true,
				pluginSurfaceHooks:  true,
				pluginFormatCodex:   true,
			},
			TrailerExample: "Co-Authored-By: codex[bot] <codex[bot]@users.noreply.github.com>",
		},

		"droid": {
			Skills: SkillsSymlink,
			MCP: mcpTargetPtr(mcpTarget{
				agentName:  "droid",
				configPath: func(home string) string { return filepath.Join(home, ".factory", "mcp.json") },
				inspect:    inspectJSONMCPServer,
				patch:      patchJSONMCPServer,
				read:       readJSONMCPServer,
				rootKey:    "mcpServers",
				defaults:   map[string]interface{}{"type": "stdio", "disabled": false},
			}),
			Roles: &RolesCapability{Extension: ".md", Render: renderDroidAgentRole},
			Hooks: &hookTarget{
				agentName: "droid",
				inspect:   inspectDroidHook,
				patch:     patchDroidHook,
			},
			PluginSurfaces: map[string]bool{
				pluginSurfaceAgents: true,
				pluginSurfaceHooks:  true,
			},
			RootInstructions: &RootInstructionsCapability{
				Path:     func(home string) string { return filepath.Join(home, ".factory", "AGENTS.md") },
				Expected: func(home string) string { return filepath.Join(home, ".agents", "AGENTS.md") },
			},
			TrailerExample: "Co-authored-by: factory-droid[bot] <factory-droid[bot]@users.noreply.github.com>",
		},

		"hermes": {
			Skills: SkillsConfigDriven,
			InspectSkills: func(agent agentConfig, expected map[string]string, agentsSkillRoot string, cfg config, home string) (agentReport, error) {
				return inspectHermesAgent(agent, expected, agentsSkillRoot, cfg, home)
			},
			Setup: func(home string, cfg config) (bool, error) {
				return patchHermesConfig(home, cfg)
			},
			MCP: mcpTargetPtr(mcpTarget{
				agentName:  "hermes",
				configPath: func(home string) string { return filepath.Join(home, ".hermes", "config.yaml") },
				inspect:    inspectYAMLMCPServer,
				patch:      patchYAMLMCPServer,
				read:       readYAMLMCPServer,
				rootKey:    "mcp_servers",
			}),
			Hooks: &hookTarget{
				agentName: "hermes",
				inspect:   inspectHermesHook,
				patch:     patchHermesHook,
			},
			PluginSurfaces: map[string]bool{
				pluginSurfaceHooks: true,
			},
			IntegrationNote: "config-driven via ~/.hermes/config.yaml -> skills.external_dirs",
			DoctorChecks: []DoctorCheck{
				{Name: "hermes direct mirrors", Run: checkHermesDirectMirrors},
				{Name: "hermes hooks", Run: func(_, home string, cfg config) checkResult {
					return checkHermesHooks(home, cfg)
				}},
			},
			TrailerExample: "Co-Authored-By: hermes[bot] <hermes[bot]@users.noreply.github.com>",
		},

		agentPi: {
			Detect:         detectVanillaPi,
			Skills:         SkillsSymlink,
			PluginSurfaces: map[string]bool{pluginSurfaceMCP: false},
			TrailerExample: "Co-authored-by: pi[bot] <pi[bot]@users.noreply.github.com>",
		},

		agentOMP: {
			Skills: SkillsSymlink,
			MCP: mcpTargetPtr(mcpTarget{
				agentName:  agentOMP,
				configPath: func(home string) string { return filepath.Join(home, ".omp", "agent", "mcp.json") },
				inspect:    inspectJSONMCPServer,
				patch:      patchJSONMCPServer,
				read:       readJSONMCPServer,
				rootKey:    "mcpServers",
			}),
			Roles: &RolesCapability{Extension: ".md", Render: renderOMPAgentRole},
			PluginSurfaces: map[string]bool{
				pluginSurfaceAgents: true,
			},
		},
	}
}

func getHarnesses() map[string]*Harness {
	harnessesOnce.Do(initHarnesses)
	return harnesses
}

func mcpTargetPtr(t mcpTarget) *mcpTarget {
	return &t
}

// harnessFor returns the harness for the given agent name.
// Returns nil if no harness is registered (unknown agent).
func harnessFor(name string) *Harness {
	return getHarnesses()[normalizeAgentName(name)]
}

// mcpTargetForHarness returns the MCP target from the harness registry.
// This replaces the old mcpTargets map lookup.
func mcpTargetForHarness(agentName string) (mcpTarget, error) {
	h := harnessFor(agentName)
	if h == nil || h.MCP == nil {
		return mcpTarget{}, fmt.Errorf("agent %q has no MCP support", agentName)
	}
	return *h.MCP, nil
}

// hookTargetForHarness returns the hook target from the harness registry.
func hookTargetForHarness(agentName string) (hookTarget, bool) {
	h := harnessFor(agentName)
	if h == nil || h.Hooks == nil {
		return hookTarget{}, false
	}
	return *h.Hooks, true
}

// hasMCPSupport returns true if the agent has MCP support registered.
func hasMCPSupport(agentName string) bool {
	h := harnessFor(agentName)
	return h != nil && h.MCP != nil
}

// allHarnessNames returns all registered agent names.
func allHarnessNames() []string {
	names := make([]string, 0, len(getHarnesses()))
	for name := range getHarnesses() {
		names = append(names, name)
	}
	return names
}

// sortedHarnessNames returns all registered agent names in alphabetical order.
func sortedHarnessNames() []string {
	names := allHarnessNames()
	sort.Strings(names)
	return names
}

func detectVanillaPi(executable string) bool {
	// executable is the absolute path returned by exec.LookPath in isDetected.
	version, _ := exec.Command(executable, "--version").CombinedOutput() // nosemgrep: go.lang.security.audit.dangerous-exec-command
	return !bytes.HasPrefix(bytes.TrimSpace(version), []byte("omp/"))
}
