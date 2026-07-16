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

func TestIntervalToScheduleWeekly(t *testing.T) {
	if got := intervalToSchedule(cronIntervalWeekly); got != "0 4 * * 1" {
		t.Fatalf("weekly schedule = %q, want Monday 04:00", got)
	}
}

func TestPatchHermesConfigAddsDotagentsSkillDir(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("skills:\n  external_dirs:\n    - ~/keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patched, err := patchHermesConfig(home, config{})
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
	if !containsInterfaceString(dirs, "~/keep") || !containsInterfaceString(dirs, dotagentsSkillsPathValue) {
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
