package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMCPJSON(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupPluginDir(t *testing.T) (string, string) {
	t.Helper()
	pluginRoot := t.TempDir()
	pluginData := t.TempDir()
	writePluginJSON(t, pluginRoot, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin"
	}`)
	return pluginRoot, pluginData
}

func TestDiscoverPluginMCPServersStdio(t *testing.T) {
	root, data := setupPluginDir(t)

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "server"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"local-server": {
				"type": "stdio",
				"command": "./bin/server",
				"args": ["--data", "${PLUGIN_DATA}/db", "--config", "${PLUGIN_ROOT}/config.json"],
				"env": {
					"DATA_DIR": "${PLUGIN_DATA}/state"
				}
			}
		}
	}`)

	servers, problems, err := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	s := servers[0]
	if s.Name != "local-server" {
		t.Errorf("name = %q", s.Name)
	}
	if !strings.HasSuffix(s.Command, filepath.Join("bin", "server")) {
		t.Errorf("command = %q, expected absolute path ending in bin/server", s.Command)
	}
	if len(s.Args) != 4 {
		t.Fatalf("args len = %d", len(s.Args))
	}
	if !strings.HasPrefix(s.Args[1], data) {
		t.Errorf("args[1] = %q, expected PLUGIN_DATA prefix", s.Args[1])
	}
	if !strings.HasPrefix(s.Args[3], root) {
		t.Errorf("args[3] = %q, expected PLUGIN_ROOT prefix", s.Args[3])
	}
	if s.Env["PLUGIN_ROOT"] != root {
		t.Errorf("env PLUGIN_ROOT = %q", s.Env["PLUGIN_ROOT"])
	}
	if s.Env["PLUGIN_DATA"] != data {
		t.Errorf("env PLUGIN_DATA = %q", s.Env["PLUGIN_DATA"])
	}
	if !strings.HasPrefix(s.Env["DATA_DIR"], data) {
		t.Errorf("env DATA_DIR = %q", s.Env["DATA_DIR"])
	}
}

func TestDiscoverPluginMCPServersCwdSkipped(t *testing.T) {
	root, data := setupPluginDir(t)
	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"with-cwd": {
				"type": "stdio",
				"command": "node",
				"cwd": "${PLUGIN_ROOT}"
			}
		}
	}`)

	servers, problems, _ := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
	if len(problems) == 0 {
		t.Fatal("expected a problem for cwd")
	}
	if !strings.Contains(problems[0], "cwd unsupported") {
		t.Errorf("problem = %q", problems[0])
	}
}

func TestDiscoverPluginMCPServersReservedEnv(t *testing.T) {
	root, data := setupPluginDir(t)
	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"bad-env": {
				"type": "stdio",
				"command": "node",
				"env": {"PLUGIN_ROOT": "/custom/path"}
			}
		}
	}`)

	servers, problems, _ := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
	if len(problems) == 0 {
		t.Fatal("expected a problem for reserved env key")
	}
}

func TestDiscoverPluginMCPServersUnsupportedTransport(t *testing.T) {
	root, data := setupPluginDir(t)
	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"remote": {
				"type": "streamable-http",
				"url": "https://api.example.com/mcp"
			},
			"local": {
				"type": "stdio",
				"command": "node"
			}
		}
	}`)

	servers, problems, _ := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server (stdio only), got %d", len(servers))
	}
	if servers[0].Name != "local" {
		t.Errorf("expected 'local' server, got %q", servers[0].Name)
	}
	foundTransport := false
	for _, p := range problems {
		if strings.Contains(p, "unsupported transport") {
			foundTransport = true
		}
	}
	if !foundTransport {
		t.Errorf("expected unsupported transport problem, got %v", problems)
	}
}

func TestDiscoverPluginMCPServersSchemaMismatch(t *testing.T) {
	root, data := setupPluginDir(t)
	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/2.0.0/mcp.schema.json",
		"mcpServers": {}
	}`)

	servers, problems, _ := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if len(servers) != 0 {
		t.Errorf("expected 0 servers for schema mismatch, got %d", len(servers))
	}
	if len(problems) == 0 {
		t.Fatal("expected problem for schema mismatch")
	}
}

func TestDiscoverPluginMCPServersCommandEscape(t *testing.T) {
	root, data := setupPluginDir(t)
	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"escape": {
				"type": "stdio",
				"command": "../../etc/passwd"
			}
		}
	}`)

	servers, problems, _ := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
	if len(problems) == 0 {
		t.Fatal("expected problem for command escape")
	}
}

func TestDiscoverPluginMCPServersBadServerName(t *testing.T) {
	root, data := setupPluginDir(t)
	writeMCPJSON(t, root, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"My.Server": {
				"type": "stdio",
				"command": "node"
			}
		}
	}`)

	servers, problems, _ := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
	if len(problems) == 0 {
		t.Fatal("expected problem for bad server name")
	}
}

func TestDiscoverPluginMCPServersMissing(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()

	servers, problems, err := discoverPluginMCPServers(root, pluginManifestSchemaID, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 || len(problems) != 0 {
		t.Errorf("expected empty result for missing mcp.json")
	}
}

func TestExpandPluginPlaceholders(t *testing.T) {
	root := "/home/user/.agents/plugins/test"
	data := "/home/user/.agents/plugins/data/test"

	tests := []struct {
		input string
		want  string
	}{
		{"${PLUGIN_ROOT}/config.json", root + "/config.json"},
		{"${PLUGIN_DATA}/db", data + "/db"},
		{"${PLUGIN_ROOT}/a/${PLUGIN_DATA}/b", root + "/a/" + data + "/b"},
		{"no-placeholders", "no-placeholders"},
		{"${UNKNOWN}", "${UNKNOWN}"},
		{"", ""},
	}
	for _, tt := range tests {
		got := expandPluginPlaceholders(tt.input, root, data)
		if got != tt.want {
			t.Errorf("expandPluginPlaceholders(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExpandPluginPlaceholdersSinglePass(t *testing.T) {
	root := "/path"
	data := "${PLUGIN_ROOT}/nested"

	got := expandPluginPlaceholders("${PLUGIN_DATA}/file", root, data)
	want := "${PLUGIN_ROOT}/nested/file"
	if got != want {
		t.Errorf("single-pass test: got %q, want %q (proves no recursive expansion)", got, want)
	}
}

func TestInjectPluginMCPServersOptIn(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")
	pluginDir := filepath.Join(cacheRoot, "test-plugin")
	makeGitDir(t, pluginDir)
	writePluginJSON(t, pluginDir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin"
	}`)
	writeMCPJSON(t, pluginDir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"test-server": {
				"type": "stdio",
				"command": "node"
			}
		}
	}`)

	cfg := config{
		ExternalSkills: []externalSkillSource{
			{URL: "https://github.com/example/test-plugin", Branch: "main", SkillDir: "skills", MCP: false},
		},
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	injectPluginMCPServers(&cfg, home)
	if len(cfg.MCPServers) != 0 {
		t.Fatalf("expected 0 MCP servers when mcp: false, got %d", len(cfg.MCPServers))
	}
}

func TestInjectPluginMCPServersWithOptIn(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")
	pluginDir := filepath.Join(cacheRoot, "test-plugin")
	makeGitDir(t, pluginDir)
	writePluginJSON(t, pluginDir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin"
	}`)
	writeMCPJSON(t, pluginDir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"test-server": {
				"type": "stdio",
				"command": "node"
			}
		}
	}`)

	cfg := config{
		ExternalSkills: []externalSkillSource{
			{URL: "https://github.com/example/test-plugin", Branch: "main", SkillDir: "skills", MCP: true},
		},
	}

	injectPluginMCPServers(&cfg, home)
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("expected 1 MCP server when mcp: true, got %d", len(cfg.MCPServers))
	}
	if cfg.MCPServers[0].Name != "test-server" {
		t.Errorf("server name = %q", cfg.MCPServers[0].Name)
	}
}

func TestInjectPluginMCPServersUserWins(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")
	pluginDir := filepath.Join(cacheRoot, "test-plugin")
	makeGitDir(t, pluginDir)
	writePluginJSON(t, pluginDir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin"
	}`)
	writeMCPJSON(t, pluginDir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": {
			"my-server": {
				"type": "stdio",
				"command": "node"
			}
		}
	}`)

	cfg := config{
		MCPServers: []mcpServerConfig{
			{Name: "my-server", Enabled: true, Command: "custom-cmd"},
		},
		ExternalSkills: []externalSkillSource{
			{URL: "https://github.com/example/test-plugin", Branch: "main", SkillDir: "skills", MCP: true},
		},
	}

	injectPluginMCPServers(&cfg, home)
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("expected 1 MCP server (user's), got %d", len(cfg.MCPServers))
	}
	if cfg.MCPServers[0].Command != "custom-cmd" {
		t.Errorf("expected user's command, got %q", cfg.MCPServers[0].Command)
	}
}
