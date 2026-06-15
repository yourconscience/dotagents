package main

import (
	"testing"
)

func TestResolveSourceStatus_defaults(t *testing.T) {
	cfg := config{Version: 1}
	statuses := resolveSourceStatus(cfg, "/tmp/fakehome")

	if len(statuses) != len(sourceRegistry) {
		t.Fatalf("expected %d sources, got %d", len(sourceRegistry), len(statuses))
	}

	byName := make(map[string]sourceStatus)
	for _, ss := range statuses {
		byName[ss.Name] = ss
	}

	// discord defaults to disabled
	if byName["discord"].Enabled {
		t.Error("discord should be disabled by default")
	}

	// github defaults to enabled
	if !byName["github"].Enabled {
		t.Error("github should be enabled by default")
	}

	// ra defaults to disabled
	if byName["ra"].Enabled {
		t.Error("ra should be disabled by default")
	}

	// hacker-news algolia is always available (public API, no check)
	hn := byName["hacker-news"]
	if hn.Best != "algolia" {
		t.Errorf("hacker-news best should be algolia, got %q", hn.Best)
	}
}

func TestResolveSourceStatus_enableOverride(t *testing.T) {
	enabled := true
	cfg := config{
		Version: 1,
		Sources: []sourceConfig{
			{Name: "discord", Enabled: &enabled},
		},
	}
	statuses := resolveSourceStatus(cfg, "/tmp/fakehome")

	for _, ss := range statuses {
		if ss.Name == "discord" {
			if !ss.Enabled {
				t.Error("discord should be enabled via override")
			}
			return
		}
	}
	t.Error("discord not found in statuses")
}

func TestResolveSourceStatus_disableOverride(t *testing.T) {
	disabled := false
	cfg := config{
		Version: 1,
		Sources: []sourceConfig{
			{Name: "github", Enabled: &disabled},
		},
	}
	statuses := resolveSourceStatus(cfg, "/tmp/fakehome")

	for _, ss := range statuses {
		if ss.Name == "github" {
			if ss.Enabled {
				t.Error("github should be disabled via override")
			}
			if ss.Best != "" {
				t.Errorf("disabled source should have empty best, got %q", ss.Best)
			}
			return
		}
	}
	t.Error("github not found in statuses")
}

func TestResolveSourceStatus_mcpCheck(t *testing.T) {
	cfg := config{
		Version: 1,
		MCPServers: []mcpServerConfig{
			{Name: "tavily", Enabled: true, Command: "npx"},
		},
	}
	statuses := resolveSourceStatus(cfg, "/tmp/fakehome")

	for _, ss := range statuses {
		if ss.Name == "web-search" {
			if ss.Best != "tavily-mcp" {
				t.Errorf("web-search best should be tavily-mcp, got %q", ss.Best)
			}
			return
		}
	}
	t.Error("web-search not found")
}

func TestResolveSourceStatus_mcpMissing(t *testing.T) {
	cfg := config{Version: 1}
	statuses := resolveSourceStatus(cfg, "/tmp/fakehome")

	for _, ss := range statuses {
		if ss.Name == "linkedin" {
			// linkedin-mcp should be unavailable without MCP config
			for _, ms := range ss.Methods {
				if ms.Name == "linkedin-mcp" && ms.Available {
					t.Error("linkedin-mcp should be unavailable without MCP server config")
				}
			}
			// best should fall back to websearch
			if ss.Best != "websearch" {
				t.Errorf("linkedin best should be websearch without MCP, got %q", ss.Best)
			}
			return
		}
	}
	t.Error("linkedin not found")
}

func TestResolveSourceStatus_preferredOverride(t *testing.T) {
	cfg := config{
		Version: 1,
		Sources: []sourceConfig{
			{Name: "reddit", Preferred: "pullpush"},
		},
	}
	statuses := resolveSourceStatus(cfg, "/tmp/fakehome")

	for _, ss := range statuses {
		if ss.Name == "reddit" {
			if ss.Best != "pullpush" {
				t.Errorf("reddit best should be pullpush via preferred override, got %q", ss.Best)
			}
			return
		}
	}
	t.Error("reddit not found")
}

func TestFindSourceDef(t *testing.T) {
	def := findSourceDef("x.com")
	if def == nil {
		t.Fatal("x.com not found in registry")
	}
	if def.Desc != "X.com / Twitter" {
		t.Errorf("unexpected desc: %s", def.Desc)
	}

	if findSourceDef("nonexistent") != nil {
		t.Error("nonexistent source should return nil")
	}
}

func TestSourceConfigMerge(t *testing.T) {
	enabled := true
	base := config{
		Version: 1,
		Agents:  []agentConfig{{Name: "test", Enabled: true, SkillRoot: "/tmp"}},
		Sources: []sourceConfig{
			{Name: "discord"},
			{Name: "github"},
		},
	}
	overlay := config{
		Sources: []sourceConfig{
			{Name: "discord", Enabled: &enabled, Preferred: "discord-cli"},
		},
	}
	mergeConfig(&base, overlay)

	if len(base.Sources) != 2 {
		t.Fatalf("expected 2 sources after merge, got %d", len(base.Sources))
	}
	for _, s := range base.Sources {
		if s.Name == "discord" {
			if s.Enabled == nil || !*s.Enabled {
				t.Error("discord enabled should be true after merge")
			}
			if s.Preferred != "discord-cli" {
				t.Errorf("discord preferred should be discord-cli, got %q", s.Preferred)
			}
			return
		}
	}
	t.Error("discord not found after merge")
}
