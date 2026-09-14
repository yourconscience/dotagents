package main

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPiDesktopHarnessCapabilities(t *testing.T) {
	piDesktop := harnessFor(agentPiDesktop)
	if piDesktop == nil {
		t.Fatal("Pi Desktop harness is not registered")
	}
	if piDesktop.Skills != SkillsSymlink {
		t.Fatalf("Pi Desktop skills capability = %v, want symlink", piDesktop.Skills)
	}
	if piDesktop.Roles == nil || piDesktop.Roles.Extension != ".md" {
		t.Fatalf("Pi Desktop roles capability = %#v, want Markdown roles", piDesktop.Roles)
	}
	if piDesktop.MCP != nil || piDesktop.Hooks != nil || piDesktop.RootInstructions != nil {
		t.Fatal("Pi Desktop must expose only its verified skills and subagents surfaces")
	}
	if piDesktop.IntegrationNote == "" {
		t.Fatal("Pi Desktop should document its native global roots")
	}
}

func TestPiDesktopDefaultConfig(t *testing.T) {
	for _, cfg := range defaultAgentConfigs() {
		if cfg.Name != agentPiDesktop {
			continue
		}
		if cfg.SkillRoot != "~/.agents/skills" || cfg.AgentRoot != "~/.agents/subagents" || cfg.Detect != "" {
			t.Fatalf("Pi Desktop default config = %#v", cfg)
		}
		return
	}
	t.Fatal("Pi Desktop not found in default agent configs")
}

func TestPiDesktopRendersSupportedSubagentRole(t *testing.T) {
	role := agentRole{
		Name:         "researcher",
		Description:  "Find reliable evidence",
		Model:        "gpt-5.6-luna",
		Tools:        []string{"read", "grep"},
		Instructions: "Compare the sources.",
	}
	root := filepath.Join(t.TempDir(), ".agents", "subagents")
	path, content, ok := renderAgentRole(role, agentConfig{Name: agentPiDesktop, AgentRoot: root})
	if !ok {
		t.Fatal("Pi Desktop role was not rendered")
	}
	if want := filepath.Join(root, "researcher.md"); path != want {
		t.Fatalf("Pi Desktop role path = %q, want %q", path, want)
	}
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) != 3 {
		t.Fatalf("Pi Desktop role lacks YAML frontmatter:\n%s", content)
	}
	var frontmatter struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Model       string `yaml:"model"`
		Tools       string `yaml:"tools"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err != nil {
		t.Fatalf("parse Pi Desktop role frontmatter: %v", err)
	}
	if frontmatter.Name != role.Name || frontmatter.Description != role.Description || frontmatter.Model != role.Model || frontmatter.Tools != "read, grep" {
		t.Fatalf("Pi Desktop role frontmatter = %#v", frontmatter)
	}
	if !strings.Contains(parts[2], role.Instructions) {
		t.Fatalf("Pi Desktop role dropped instructions:\n%s", content)
	}
}
