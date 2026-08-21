package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRunSyncRepairsHermesExternalDirsDrift(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTAGENTS_HOME", repoRoot)

	writeSyncTestFile(t, filepath.Join(repoRoot, "dotagents.yaml"), []byte(`version: 1
agents:
  - name: hermes
    enabled: true
    skill_root: ~/.hermes/skills
`))
	writeSyncTestFile(t, filepath.Join(repoRoot, "skills", "sample", "SKILL.md"), []byte("---\nname: sample\n---\n"))
	writeSyncTestFile(t, filepath.Join(home, ".hermes", "config.yaml"), []byte("{}\n"))

	if err := runSync(runOptions{Agents: agentHermes}); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(filepath.Join(home, ".hermes", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	skills, ok := raw["skills"].(map[string]interface{})
	if !ok {
		t.Fatalf("skills = %#v, want map", raw["skills"])
	}
	dirs, ok := skills["external_dirs"].([]interface{})
	if !ok {
		t.Fatalf("external_dirs = %#v, want list", skills["external_dirs"])
	}
	if !containsInterfaceString(dirs, filepath.Join(repoRoot, "skills")) {
		t.Fatalf("external_dirs = %#v, want %q", dirs, filepath.Join(repoRoot, "skills"))
	}

	if _, err := os.Lstat(filepath.Join(home, ".hermes", "skills", "sample")); !os.IsNotExist(err) {
		t.Fatalf("runSync mirrored Hermes skill instead of repairing config, stat err = %v", err)
	}
}

func writeSyncTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInspectAgentAcceptsMatchingNativeCopyAndRejectsDifferentContent(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	writeSyncTestFile(t, filepath.Join(repoRoot, "AGENTS.md"), []byte("# Shared\n"))
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, "AGENTS.md"), filepath.Join(home, ".claude", "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(repoRoot, "skills", "existing")
	native := filepath.Join(home, ".claude", "skills", "existing")
	content := []byte("---\nname: existing\n---\n")
	writeSyncTestFile(t, filepath.Join(canonical, "SKILL.md"), content)
	writeSyncTestFile(t, filepath.Join(canonical, "references", "guide.txt"), []byte("same\n"))
	writeSyncTestFile(t, filepath.Join(native, "SKILL.md"), content)
	writeSyncTestFile(t, filepath.Join(native, "references", "guide.txt"), []byte("same\n"))
	agent := agentConfig{Name: agentClaudeCode, Enabled: true, SkillRoot: filepath.Dir(native)}

	report, err := inspectAgent(agent, map[string]string{"existing": canonical}, repoRoot, filepath.Join(repoRoot, "skills"), config{Version: 1}, home)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Synced || !stringSlicesEqual(report.Managed, []string{"existing"}) || len(report.Conflicts) != 0 {
		t.Fatalf("matching native copy report = %#v", report)
	}
	if info, err := os.Lstat(native); err != nil || !info.IsDir() {
		t.Fatalf("matching native source was replaced: info=%v err=%v", info, err)
	}

	writeSyncTestFile(t, filepath.Join(native, "references", "guide.txt"), []byte("different\n"))
	report, err = inspectAgent(agent, map[string]string{"existing": canonical}, repoRoot, filepath.Join(repoRoot, "skills"), config{Version: 1}, home)
	if err != nil {
		t.Fatal(err)
	}
	if report.Synced || len(report.Conflicts) != 1 {
		t.Fatalf("different native copy report = %#v", report)
	}
}
