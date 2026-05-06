package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testMCPServer() mcpServerConfig {
	return mcpServerConfig{
		Name:    "linkedin",
		Command: "uvx",
		Args:    []string{"linkedin-scraper-mcp@latest"},
		Env:     map[string]string{"UV_HTTP_TIMEOUT": "300"},
	}
}

func TestDroidMCPInspectMissing(t *testing.T) {
	home := t.TempDir()
	state, err := inspectMCPServer(agentDroid, testMCPServer(), home)
	if err != nil {
		t.Fatal(err)
	}
	if state != stateMissing {
		t.Fatalf("inspect missing = %q, want %q", state, stateMissing)
	}
}

func TestDroidMCPInspectSynced(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".factory", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "mcpServers": {
    "linkedin": {
      "type": "stdio",
      "command": "uvx",
      "args": ["linkedin-scraper-mcp@latest"],
      "env": {"UV_HTTP_TIMEOUT": "300"},
      "disabled": false,
      "local": "kept"
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := inspectMCPServer(agentDroid, testMCPServer(), home)
	if err != nil {
		t.Fatal(err)
	}
	if state != stateSynced {
		t.Fatalf("inspect synced = %q, want %q", state, stateSynced)
	}
}

func TestDroidMCPReadAllowsMissingDisabled(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".factory", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "mcpServers": {
    "linkedin": {
      "type": "stdio",
      "command": "uvx",
      "args": ["linkedin-scraper-mcp@latest"],
      "env": {"UV_HTTP_TIMEOUT": "300"}
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	server, err := readNativeMCPServer(agentDroid, "linkedin", home)
	if err != nil {
		t.Fatal(err)
	}
	if server.Name != "linkedin" || server.Command != "uvx" {
		t.Fatalf("unexpected server: %#v", server)
	}
}

func TestDroidMCPReadRejectsDisabledTrue(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".factory", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "mcpServers": {
    "linkedin": {
      "type": "stdio",
      "command": "uvx",
      "disabled": true
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := readNativeMCPServer(agentDroid, "linkedin", home); err == nil {
		t.Fatal("readNativeMCPServer succeeded with disabled true, want error")
	}
}

func TestDroidMCPInspectDrifted(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".factory", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "mcpServers": {
    "linkedin": {
      "type": "stdio",
      "command": "uvx",
      "args": ["wrong"],
      "env": {"UV_HTTP_TIMEOUT": "300"},
      "disabled": false
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := inspectMCPServer(agentDroid, testMCPServer(), home)
	if err != nil {
		t.Fatal(err)
	}
	if state != stateDrifted {
		t.Fatalf("inspect drifted = %q, want %q", state, stateDrifted)
	}
}

func TestDroidMCPPatchPreservesUnrelatedServers(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".factory", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "otherTopLevel": true,
  "mcpServers": {
    "other": {
      "type": "stdio",
      "command": "node",
      "args": ["server.js"],
      "disabled": false
    },
    "linkedin": {
      "type": "stdio",
      "command": "old",
      "args": ["old"],
      "env": {"LOCAL_ONLY": "keep"},
      "disabled": true,
      "local": "keep"
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchMCPServer(agentDroid, testMCPServer(), home); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["otherTopLevel"] != true {
		t.Fatalf("top-level field was not preserved: %#v", raw)
	}
	servers, ok := asMap(raw["mcpServers"])
	if !ok {
		t.Fatalf("mcpServers missing: %#v", raw)
	}
	other, ok := asMap(servers["other"])
	if !ok || other["command"] != "node" {
		t.Fatalf("unrelated server was not preserved: %#v", servers["other"])
	}
	linkedin, ok := asMap(servers["linkedin"])
	if !ok {
		t.Fatalf("managed server missing: %#v", servers)
	}
	if !matchManagedMCPMap(linkedin, testMCPServer(), mcpTargets[agentDroid].defaults) {
		t.Fatalf("managed server not patched: %#v", linkedin)
	}
	if linkedin["local"] != "keep" {
		t.Fatalf("unrelated managed-entry field was not preserved: %#v", linkedin)
	}
	env, ok := asMap(linkedin["env"])
	if !ok || env["LOCAL_ONLY"] != "keep" || env["UV_HTTP_TIMEOUT"] != "300" {
		t.Fatalf("env fields not merged: %#v", linkedin["env"])
	}
}

func TestClaudeMCPPatchCreatesMissingConfig(t *testing.T) {
	home := t.TempDir()
	if err := patchMCPServer(agentClaudeCode, testMCPServer(), home); err != nil {
		t.Fatal(err)
	}
	state, err := inspectMCPServer(agentClaudeCode, testMCPServer(), home)
	if err != nil {
		t.Fatal(err)
	}
	if state != stateSynced {
		t.Fatalf("inspect after create = %q, want %q", state, stateSynced)
	}
	out, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	servers, _ := asMap(raw["mcpServers"])
	linkedin, _ := asMap(servers["linkedin"])
	if _, ok := linkedin["type"]; ok {
		t.Fatalf("claude config should not get droid type default: %#v", linkedin)
	}
	if _, ok := linkedin["disabled"]; ok {
		t.Fatalf("claude config should not get droid disabled default: %#v", linkedin)
	}
}

func TestUnsupportedMCPAgentErrors(t *testing.T) {
	home := t.TempDir()
	if _, err := inspectMCPServer("openclaw", testMCPServer(), home); err == nil {
		t.Fatal("inspectMCPServer(openclaw) succeeded, want error")
	}
	if err := patchMCPServer("openclaw", testMCPServer(), home); err == nil {
		t.Fatal("patchMCPServer(openclaw) succeeded, want error")
	}
}

func TestDesiredMCPServersEmptyAgentsOnlyTargetsSupportedAgents(t *testing.T) {
	cfg := config{
		MCPServers: []mcpServerConfig{
			{
				Name:    "default",
				Enabled: true,
				Command: "uvx",
			},
		},
	}

	if got := desiredMCPServersForAgent(cfg, agentClaudeCode); len(got) != 1 || got[0].Name != "default" {
		t.Fatalf("desiredMCPServersForAgent(%q) = %#v, want default server", agentClaudeCode, got)
	}
	if got := desiredMCPServersForAgent(cfg, "openclaw"); len(got) != 0 {
		t.Fatalf("desiredMCPServersForAgent(openclaw) = %#v, want no servers", got)
	}
}

func TestDroidMCPPatchCreatesMissingConfig(t *testing.T) {
	home := t.TempDir()
	if err := patchMCPServer(agentDroid, testMCPServer(), home); err != nil {
		t.Fatal(err)
	}
	state, err := inspectMCPServer(agentDroid, testMCPServer(), home)
	if err != nil {
		t.Fatal(err)
	}
	if state != stateSynced {
		t.Fatalf("inspect after create = %q, want %q", state, stateSynced)
	}
}
