package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeAmpE2EConfig(t *testing.T, path string) {
	t.Helper()
	data := []byte(fmt.Sprintf(`version: 1
agents:
  - name: amp
    enabled: true
    skill_root: ~/.agents/skills
mcp_servers:
  - name: local
    enabled: true
    command: %s
    args:
      - local-mcp@latest
    env:
      TOKEN: ${TOKEN}
    agents:
      - amp
`, testMCPServer().Command))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAmpSetupE2EConfiguresSkillsAndMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(t.TempDir(), "dotagents.yaml")
	writeAmpE2EConfig(t, configPath)

	if err := run([]string{"setup", "--agents=amp", "--config", configPath}); err != nil {
		t.Fatal(err)
	}

	repoLink, err := os.Readlink(filepath.Join(home, ".agents"))
	if err != nil {
		t.Fatal(err)
	}
	repoRoot, _, err := findRoots()
	if err != nil {
		t.Fatal(err)
	}
	if repoLink != repoRoot {
		t.Fatalf("~/.agents target = %q, want %q", repoLink, repoRoot)
	}

	settingsPath := filepath.Join(home, ".config", "amp", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["amp.skills.path"] != ampSkillsPath {
		t.Fatalf("amp.skills.path = %#v", settings["amp.skills.path"])
	}
	servers, ok := asMap(settings["amp.mcpServers"])
	if !ok {
		t.Fatalf("amp.mcpServers missing from %#v", settings)
	}
	local, ok := asMap(servers["local"])
	if !ok {
		t.Fatalf("local MCP missing from %#v", servers)
	}
	if local["command"] != testMCPServer().Command {
		t.Fatalf("local MCP = %#v", local)
	}

	if err := run([]string{"status", "--agents=amp", "--config", configPath}); err != nil {
		t.Fatal(err)
	}
}

func TestAmpSetupE2EPreservesExistingSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(t.TempDir(), "dotagents.yaml")
	writeAmpE2EConfig(t, configPath)
	settingsPath := filepath.Join(home, ".config", "amp", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"amp.url":"http://localhost:8317","amp.mcpServers":{"keep":{"command":"node","args":[]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"setup", "--agents=amp", "--config", configPath}); err != nil {
		t.Fatal(err)
	}

	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["amp.url"] != "http://localhost:8317" {
		t.Fatalf("amp.url was not preserved: %#v", settings)
	}
	servers, _ := asMap(settings["amp.mcpServers"])
	if _, ok := servers["keep"]; !ok {
		t.Fatalf("existing MCP was not preserved: %#v", servers)
	}
	if _, ok := servers["local"]; !ok {
		t.Fatalf("managed MCP was not added: %#v", servers)
	}
}
