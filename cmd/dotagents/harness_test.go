package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPiAndOMPHarnessCapabilities(t *testing.T) {
	pi := harnessFor(agentPi)
	if pi == nil {
		t.Fatal("Pi harness is not registered")
	}
	if pi.Skills != SkillsSymlink {
		t.Fatalf("Pi skills capability = %v, want symlink", pi.Skills)
	}
	if pi.MCP != nil {
		t.Fatal("Pi unexpectedly exposes MCP support")
	}
	if pi.Roles != nil {
		t.Fatal("Pi unexpectedly exposes agent-role support")
	}

	omp := harnessFor(agentOMP)
	if omp == nil {
		t.Fatal("OMP harness is not registered")
	}
	if omp.Skills != SkillsSymlink {
		t.Fatalf("OMP skills capability = %v, want symlink", omp.Skills)
	}
	if omp.MCP == nil {
		t.Fatal("OMP does not expose MCP support")
	}
	if omp.Roles == nil || omp.Roles.Extension != ".md" {
		t.Fatalf("OMP roles capability = %#v, want Markdown roles", omp.Roles)
	}
}

func TestCanonicalPiAndOMPPaths(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	cfg, err := loadConfig(repoRoot, home, filepath.Join(repoRoot, "dotagents.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	agents := make(map[string]agentConfig, len(cfg.Agents))
	for _, agent := range cfg.Agents {
		agents[agent.Name] = agent
	}
	if got, want := agents[agentPi].SkillRoot, filepath.Join(home, ".pi", "agent", "skills"); got != want {
		t.Fatalf("Pi skill root = %q, want %q", got, want)
	}
	if got, want := agents[agentOMP].SkillRoot, filepath.Join(home, ".omp", "agent", "skills"); got != want {
		t.Fatalf("OMP skill root = %q, want %q", got, want)
	}
	if got, want := agents[agentOMP].AgentRoot, filepath.Join(home, ".omp", "agent", "agents"); got != want {
		t.Fatalf("OMP agent root = %q, want %q", got, want)
	}

	target, err := mcpTargetForHarness(agentOMP)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := target.configPath(home), filepath.Join(home, ".omp", "agent", "mcp.json"); got != want {
		t.Fatalf("OMP MCP path = %q, want %q", got, want)
	}
}

func TestConfigRejectsPiMCPButAcceptsOMP(t *testing.T) {
	home := t.TempDir()
	baseAgents := []agentConfig{
		{Name: agentPi, Enabled: true, SkillRoot: filepath.Join(home, ".pi", "agent", "skills")},
		{Name: agentOMP, Enabled: true, SkillRoot: filepath.Join(home, ".omp", "agent", "skills"), AgentRoot: filepath.Join(home, ".omp", "agent", "agents")},
	}

	piConfig := config{
		Version: 1,
		Agents:  append([]agentConfig(nil), baseAgents...),
		MCPServers: []mcpServerConfig{{
			Name: "search", Enabled: true, Command: "search-server", Agents: []string{agentPi},
		}},
	}
	if err := validateConfig(&piConfig, home, true); err == nil || err.Error() != `config MCP server search targets unsupported agent "pi"` {
		t.Fatalf("Pi MCP validation error = %v", err)
	}

	ompConfig := config{
		Version: 1,
		Agents:  append([]agentConfig(nil), baseAgents...),
		MCPServers: []mcpServerConfig{{
			Name: "search", Enabled: true, Command: "search-server", Agents: []string{agentOMP},
		}},
	}
	if err := validateConfig(&ompConfig, home, true); err != nil {
		t.Fatalf("OMP MCP config rejected: %v", err)
	}
}

func TestOMPMCPPatchUsesOMPConfig(t *testing.T) {
	home := t.TempDir()
	server := testMCPServer()
	server.Agents = []string{agentOMP}

	if err := patchMCPServer(agentOMP, server, home); err != nil {
		t.Fatal(err)
	}
	state, err := inspectMCPServer(agentOMP, server, home)
	if err != nil {
		t.Fatal(err)
	}
	if state != stateSynced {
		t.Fatalf("OMP MCP state after patch = %q, want %q", state, stateSynced)
	}
	if _, err := os.Stat(filepath.Join(home, ".omp", "agent", "mcp.json")); err != nil {
		t.Fatalf("OMP MCP config was not written to native path: %v", err)
	}
	if err := patchMCPServer(agentPi, server, home); err == nil || !strings.Contains(err.Error(), "no MCP support") {
		t.Fatalf("Pi MCP patch error = %v, want unsupported-agent error", err)
	}
}

func TestOMPRendererProducesNativeRoleAndPiSkipsRoles(t *testing.T) {
	role := agentRole{
		Name:         "researcher",
		Description:  "Find reliable evidence",
		Model:        "gpt-5.5",
		Tools:        []string{"Read", "WebSearch", "Write", "NotebookEdit", "read"},
		Instructions: "Compare the sources.",
	}
	home := t.TempDir()

	if path, content, ok := renderAgentRole(role, agentConfig{Name: agentPi, AgentRoot: filepath.Join(home, ".pi", "agent", "agents")}); ok || path != "" || content != "" {
		t.Fatalf("Pi rendered unsupported role: path=%q ok=%v content=%q", path, ok, content)
	}

	path, content, ok := renderAgentRole(role, agentConfig{Name: agentOMP, AgentRoot: filepath.Join(home, ".omp", "agent", "agents")})
	if !ok {
		t.Fatal("OMP role was not rendered")
	}
	if want := filepath.Join(home, ".omp", "agent", "agents", "researcher.md"); path != want {
		t.Fatalf("OMP role path = %q, want %q", path, want)
	}

	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) != 3 {
		t.Fatalf("OMP role lacks YAML frontmatter:\n%s", content)
	}
	var frontmatter struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Model       []string `yaml:"model"`
		Tools       []string `yaml:"tools"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err != nil {
		t.Fatalf("parse OMP role frontmatter: %v", err)
	}
	if frontmatter.Name != role.Name || frontmatter.Description != role.Description {
		t.Fatalf("OMP identity frontmatter = %#v", frontmatter)
	}
	if !reflect.DeepEqual(frontmatter.Model, []string{"gpt-5.5"}) {
		t.Fatalf("OMP model = %#v", frontmatter.Model)
	}
	if !reflect.DeepEqual(frontmatter.Tools, []string{"read", "web_search", "write"}) {
		t.Fatalf("OMP tools = %#v", frontmatter.Tools)
	}
	if !strings.Contains(parts[2], role.Instructions) {
		t.Fatalf("OMP role dropped instructions:\n%s", content)
	}
}

func TestPiDetectionRejectsOMPAlias(t *testing.T) {
	writeVersionCommand := func(name, version string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' '"+version+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}

	if !detectVanillaPi(writeVersionCommand("pi", "pi 0.60.0")) {
		t.Fatal("vanilla Pi executable was rejected")
	}
	if detectVanillaPi(writeVersionCommand("pi", "omp/0.4.5")) {
		t.Fatal("OMP's pi compatibility alias was accepted as vanilla Pi")
	}
}
