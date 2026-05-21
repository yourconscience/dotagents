package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCronCommandForWeeklyDeps(t *testing.T) {
	cmd, interval := cronCommandForOptions("/repo", "/usr/bin/go", cronOptions{Deps: true, Interval: cronIntervalDefault})
	if interval != cronIntervalWeekly {
		t.Fatalf("interval = %q, want %s", interval, cronIntervalWeekly)
	}
	if !strings.Contains(cmd, "deps update") {
		t.Fatalf("cmd = %q, want deps update", cmd)
	}
	if !strings.Contains(cmd, "/repo/cmd/dotagents") {
		t.Fatalf("cmd = %q, want root CLI path", cmd)
	}
	if strings.Contains(cmd, " pull") {
		t.Fatalf("cmd = %q, should not use pull mode", cmd)
	}
}

func TestPathContainsDir(t *testing.T) {
	path := strings.Join([]string{"/usr/bin", "/Users/me/go/bin"}, string(':'))
	if !pathContainsDir(path, "/Users/me/go/bin") {
		t.Fatal("expected PATH to contain Go bin dir")
	}
	if pathContainsDir(path, "/Users/me/.local/bin") {
		t.Fatal("unexpected PATH match")
	}
}

func TestGoBinDirUsesEffectiveGOBIN(t *testing.T) {
	fakeGo := writeFakeGo(t)
	t.Setenv("FAKE_GOBIN", "/custom/go/bin")
	t.Setenv("FAKE_GOPATH", "/wrong/go")

	got, err := goBinDir(fakeGo)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/go/bin" {
		t.Fatalf("goBinDir = %q, want effective GOBIN", got)
	}
}

func TestGoBinDirFallsBackToGOPATHBin(t *testing.T) {
	fakeGo := writeFakeGo(t)
	t.Setenv("FAKE_GOBIN", "")
	t.Setenv("FAKE_GOPATH", strings.Join([]string{"/first/go", "/second/go"}, string(os.PathListSeparator)))

	got, err := goBinDir(fakeGo)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("/first/go", "bin") {
		t.Fatalf("goBinDir = %q, want first GOPATH bin", got)
	}
}

func writeFakeGo(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go")
	data := `#!/bin/sh
if [ "$1" = "env" ] && [ "$2" = "GOBIN" ]; then
  printf '%s\n' "$FAKE_GOBIN"
  exit 0
fi
if [ "$1" = "env" ] && [ "$2" = "GOPATH" ]; then
  printf '%s\n' "$FAKE_GOPATH"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIntervalToScheduleWeekly(t *testing.T) {
	if got := intervalToSchedule(cronIntervalWeekly); got != "0 4 * * 1" {
		t.Fatalf("weekly schedule = %q, want Monday 04:00", got)
	}
}

func TestPatchHermesConfigAddsPluginSkillDirs(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("skills:\n  external_dirs:\n    - ~/keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pluginSource := filepath.Join(home, "plugins", "browser")
	pluginSkillDir := filepath.Join(pluginSource, "skills", "browser")
	if err := os.MkdirAll(pluginSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginSkillDir, "SKILL.md"), []byte("---\nname: browser\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patched, err := patchHermesConfig(home, config{Plugins: []pluginConfig{{
		Name:     "browser",
		Enabled:  true,
		Source:   pluginSource,
		Surfaces: []string{pluginSurfaceSkills},
		Agents:   []string{agentHermes},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("patchHermesConfig reported no changes")
	}

	out, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	skills := raw["skills"].(map[string]interface{})
	dirs := skills["external_dirs"].([]interface{})
	if !containsInterfaceString(dirs, "~/keep") || !containsInterfaceString(dirs, ampSkillsPath) || !containsInterfaceString(dirs, filepath.Join(pluginSource, "skills")) {
		t.Fatalf("external_dirs = %#v", dirs)
	}
}

func containsInterfaceString(items []interface{}, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
