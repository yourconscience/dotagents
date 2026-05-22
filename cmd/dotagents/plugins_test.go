package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginCompatibilityNativeFormats(t *testing.T) {
	codexPlugin := pluginConfig{
		Name:     "browser",
		Format:   pluginFormatCodex,
		Surfaces: []string{pluginSurfaceSkills, pluginSurfaceMCP, pluginSurfaceNative},
		Agents:   []string{agentClaudeCode, agentCodex, agentAmp, agentHermes, agentDroid},
	}

	if state, _ := pluginCompatibility(codexPlugin, agentCodex); state != "supported" {
		t.Fatalf("codex plugin on codex = %q, want supported", state)
	}
	if state, _ := pluginCompatibility(codexPlugin, agentAmp); state != "partial" {
		t.Fatalf("codex plugin on amp = %q, want partial", state)
	}

	claudePlugin := pluginConfig{
		Name:     "feature-dev",
		Format:   pluginFormatClaude,
		Surfaces: []string{pluginSurfaceSkills, pluginSurfaceAgents, pluginSurfaceCmds, pluginSurfaceNative},
		Agents:   []string{agentClaudeCode, agentDroid},
	}
	if state, _ := pluginCompatibility(claudePlugin, agentClaudeCode); state != "supported" {
		t.Fatalf("claude plugin on claude-code = %q, want supported", state)
	}
	if state, _ := pluginCompatibility(claudePlugin, agentDroid); state != "partial" {
		t.Fatalf("claude plugin on droid = %q, want partial", state)
	}
}

func TestPluginConfigValidation(t *testing.T) {
	home := t.TempDir()
	cfg := config{
		Agents: []agentConfig{
			{Name: agentClaudeCode, Enabled: true, SkillRoot: filepath.Join(home, "claude", "skills")},
			{Name: agentCodex, Enabled: true, SkillRoot: filepath.Join(home, "codex", "skills")},
		},
		Plugins: []pluginConfig{{
			Name:     "review",
			Format:   "claude-plugin",
			Surfaces: []string{"skill", "agent", "command"},
			Agents:   []string{"Claude-Code"},
		}},
	}
	if err := validateConfig(&cfg, "/home/me", false); err != nil {
		t.Fatal(err)
	}
	plugin := cfg.Plugins[0]
	if plugin.Format != pluginFormatClaude {
		t.Fatalf("format = %q, want %q", plugin.Format, pluginFormatClaude)
	}
	if plugin.Agents[0] != agentClaudeCode {
		t.Fatalf("agent = %q, want %q", plugin.Agents[0], agentClaudeCode)
	}
	if plugin.Surfaces[0] != pluginSurfaceSkills || plugin.Surfaces[1] != pluginSurfaceAgents || plugin.Surfaces[2] != pluginSurfaceCmds {
		t.Fatalf("surfaces not normalized: %#v", plugin.Surfaces)
	}
}

func TestPluginConfigRejectsUnknownSurface(t *testing.T) {
	home := t.TempDir()
	cfg := config{
		Agents: []agentConfig{{Name: agentCodex, Enabled: true, SkillRoot: filepath.Join(home, "codex", "skills")}},
		Plugins: []pluginConfig{{
			Name:     "bad",
			Surfaces: []string{"telepathy"},
			Agents:   []string{agentCodex},
		}},
	}
	if err := validateConfig(&cfg, "/home/me", false); err == nil {
		t.Fatal("validateConfig accepted unsupported plugin surface")
	}
}

func TestPluginConfigRejectsEnabledPluginWithoutSource(t *testing.T) {
	home := t.TempDir()
	cfg := config{
		Agents: []agentConfig{{Name: agentCodex, Enabled: true, SkillRoot: filepath.Join(home, "codex", "skills")}},
		Plugins: []pluginConfig{{
			Name:     "missing-source",
			Enabled:  true,
			Surfaces: []string{pluginSurfaceSkills},
			Agents:   []string{agentCodex},
		}},
	}
	if err := validateConfig(&cfg, "/home/me", false); err == nil {
		t.Fatal("validateConfig accepted enabled plugin without source")
	}
}

func TestDiscoverPluginSkillsUsesVersionedSourceAndAgentTargets(t *testing.T) {
	home := t.TempDir()
	pluginRoot := filepath.Join(home, "plugin-cache")
	t.Setenv("DOTAGENTS_CODEX_PLUGIN_ROOT", pluginRoot)
	source := filepath.Join(pluginRoot, "openai-bundled", "browser")
	skillDir := filepath.Join(source, "0.1.0", "skills", "browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: browser\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins := []pluginConfig{{
		Name:     "browser",
		Enabled:  true,
		Source:   "codex:openai-bundled/browser",
		Format:   pluginFormatCodex,
		Surfaces: []string{pluginSurfaceSkills},
		Agents:   []string{agentClaudeCode, agentCodex, agentHermes, agentDroid},
	}}

	skills, err := discoverPluginSkills(plugins, home, agentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if skills["browser"] != skillDir {
		t.Fatalf("browser skill = %q, want %q", skills["browser"], skillDir)
	}

	skills, err = discoverPluginSkills(plugins, home, agentDroid)
	if err != nil {
		t.Fatal(err)
	}
	if skills["browser"] != skillDir {
		t.Fatalf("droid browser skill = %q, want %q", skills["browser"], skillDir)
	}
}

func TestDiscoverPluginSkillsSelectsDeterministicVersion(t *testing.T) {
	home := t.TempDir()
	pluginRoot := filepath.Join(home, "plugin-cache")
	t.Setenv("DOTAGENTS_CODEX_PLUGIN_ROOT", pluginRoot)
	source := filepath.Join(pluginRoot, "openai-bundled", "browser")
	oldSkillDir := filepath.Join(source, "0.9.0", "skills", "browser")
	newSkillDir := filepath.Join(source, "0.10.0", "skills", "browser")
	for _, dir := range []string{oldSkillDir, newSkillDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: browser\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	plugins := []pluginConfig{{
		Name:     "browser",
		Enabled:  true,
		Source:   "codex:openai-bundled/browser",
		Surfaces: []string{pluginSurfaceSkills},
		Agents:   []string{agentCodex},
	}}

	skills, err := discoverPluginSkills(plugins, home, agentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if skills["browser"] != newSkillDir {
		t.Fatalf("browser skill = %q, want %q", skills["browser"], newSkillDir)
	}
}

func TestInspectAgentRemovesDisabledPluginSkillLinks(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	source := filepath.Join(home, "plugins", "browser")
	skillDir := filepath.Join(source, "skills", "browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: browser\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	agentSkillRoot := filepath.Join(home, ".codex", "skills")
	if err := os.MkdirAll(agentSkillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skillDir, filepath.Join(agentSkillRoot, "browser")); err != nil {
		t.Fatal(err)
	}

	cfg := config{Plugins: []pluginConfig{{
		Name:     "browser",
		Enabled:  false,
		Source:   source,
		Surfaces: []string{pluginSurfaceSkills},
		Agents:   []string{agentCodex},
	}}}
	report, err := inspectAgent(agentConfig{Name: agentCodex, SkillRoot: agentSkillRoot}, map[string]string{}, repoRoot, cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.StaleManaged) != 1 || report.StaleManaged[0] != "browser" {
		t.Fatalf("stale managed = %#v, want browser", report.StaleManaged)
	}
	if len(report.Removes) != 1 || report.Removes[0] != "browser" {
		t.Fatalf("removes = %#v, want browser", report.Removes)
	}
}

func TestCheckFirstPartyPluginsValidatesAllAgentTargets(t *testing.T) {
	result := checkFirstPartyPlugins(config{Plugins: []pluginConfig{{
		Name:     "native-codex",
		Enabled:  true,
		Format:   pluginFormatCodex,
		Surfaces: []string{pluginSurfaceNative},
	}}})
	if result.status != checkStatusWarn {
		t.Fatalf("status = %q, want %q", result.status, checkStatusWarn)
	}
	if !strings.Contains(result.detail, "native-codex:claude-code") || !strings.Contains(result.detail, "native-codex:amp") {
		t.Fatalf("detail = %q, want unsupported all-agent targets", result.detail)
	}
}

func TestExpectedSkillsForAgentAddsTargetedPluginSkills(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, "plugins", "frontend-design")
	skillDir := filepath.Join(source, "skills", "frontend-design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: frontend-design\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config{Plugins: []pluginConfig{{
		Name:     "frontend-design",
		Enabled:  true,
		Source:   source,
		Surfaces: []string{pluginSurfaceSkills},
		Agents:   []string{agentClaudeCode},
	}}}
	base := map[string]string{"dotagents": filepath.Join(home, ".agents", "skills", "dotagents")}

	expected, err := expectedSkillsForAgent(base, home, cfg, agentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if expected["frontend-design"] != skillDir {
		t.Fatalf("frontend-design target = %q, want %q", expected["frontend-design"], skillDir)
	}

	expected, err = expectedSkillsForAgent(base, home, cfg, agentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := expected["frontend-design"]; ok {
		t.Fatalf("codex unexpectedly received frontend-design: %#v", expected)
	}
}

func TestInspectAgentsSkipsPluginDiscoveryForUndetectedAgents(t *testing.T) {
	home := t.TempDir()
	cfg := config{Plugins: []pluginConfig{{
		Name:     "browser",
		Enabled:  true,
		Source:   filepath.Join(home, "missing-plugin-cache"),
		Surfaces: []string{pluginSurfaceSkills},
		Agents:   []string{agentCodex},
	}}}
	agents := []agentConfig{{
		Name:      agentCodex,
		Enabled:   true,
		Detect:    "definitely-not-installed-dotagents-test",
		SkillRoot: filepath.Join(home, ".codex", "skills"),
	}}

	reports, err := inspectAgents(agents, map[string]string{}, t.TempDir(), home, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Detected {
		t.Fatalf("reports = %#v, want one undetected report", reports)
	}
}
