package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDiscoveredSkill(t *testing.T, root string, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireCollisionOrigins(t *testing.T, err error, origins ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected skill collision, got nil")
	}
	for _, origin := range origins {
		if !strings.Contains(err.Error(), origin) {
			t.Fatalf("collision error %q does not identify %q", err, origin)
		}
	}
}

func TestLocalAndExternalSkillCollisionIdentifiesBothOrigins(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	writeDiscoveredSkill(t, filepath.Join(repoRoot, "skills"), "shared")

	const externalURL = "https://github.com/example/catalog"
	externalRoot := filepath.Join(externalCacheDir(home), "catalog")
	makeGitDir(t, externalRoot)
	writeDiscoveredSkill(t, filepath.Join(externalRoot, "skills"), "shared")

	_, err := expectedSkills(repoRoot, home, config{ExternalSkills: []externalSkillSource{{
		URL: externalURL, SkillDir: "skills", Branch: "main",
	}}})
	requireCollisionOrigins(t, err, "local skills", `external Git "`+externalURL+`"`)
}

func TestLocalAndPluginSkillCollisionIdentifiesBothOrigins(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	writeDiscoveredSkill(t, filepath.Join(repoRoot, "skills"), "shared")
	base, err := expectedSkills(repoRoot, home, config{})
	if err != nil {
		t.Fatal(err)
	}

	pluginRoot := filepath.Join(home, "plugins", "catalog")
	writeDiscoveredSkill(t, filepath.Join(pluginRoot, "skills"), "shared")
	cfg := config{Plugins: []pluginConfig{{
		Name: "catalog", Enabled: true, Source: pluginRoot,
		Surfaces: []string{pluginSurfaceSkills}, Agents: []string{agentClaudeCode},
	}}}
	_, err = expectedSkillsForAgent(base, home, cfg, agentClaudeCode)
	requireCollisionOrigins(t, err, "local skills", `installed plugin "catalog"`)
}

func TestExternalPluginCollisionRespectsPerAgentFiltering(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	const externalURL = "https://github.com/example/catalog"
	externalRoot := filepath.Join(externalCacheDir(home), "catalog")
	makeGitDir(t, externalRoot)
	externalSkill := writeDiscoveredSkill(t, filepath.Join(externalRoot, "skills"), "shared")

	pluginRoot := filepath.Join(home, "plugins", "review")
	writeDiscoveredSkill(t, filepath.Join(pluginRoot, "skills"), "shared")
	cfg := config{
		ExternalSkills: []externalSkillSource{{URL: externalURL, SkillDir: "skills", Branch: "main"}},
		Plugins: []pluginConfig{{
			Name: "review", Enabled: true, Source: pluginRoot,
			Surfaces: []string{pluginSurfaceSkills}, Agents: []string{agentClaudeCode},
		}},
	}
	base, err := expectedSkills(repoRoot, home, cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = expectedSkillsForAgent(base, home, cfg, agentClaudeCode)
	requireCollisionOrigins(t, err, `external Git "`+externalURL+`"`, `installed plugin "review"`)

	codexSkills, err := expectedSkillsForAgent(base, home, cfg, agentCodex)
	if err != nil {
		t.Fatalf("plugin filtered away from Codex still caused collision: %v", err)
	}
	if got := codexSkills["shared"]; got != externalSkill {
		t.Fatalf("Codex shared skill = %q, want external skill %q", got, externalSkill)
	}
}
