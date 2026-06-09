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
	t.Setenv("DOTAGENTS_ROOT", repoRoot)

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
	if !containsInterfaceString(dirs, ampSkillsPath) {
		t.Fatalf("external_dirs = %#v, want %q", dirs, ampSkillsPath)
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
