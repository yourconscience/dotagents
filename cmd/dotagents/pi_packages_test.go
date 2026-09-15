package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPiPackageSyncPreservesOtherSettings(t *testing.T) {
	home := t.TempDir()
	path := piSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
  "defaultProvider": "openai-codex",
  "defaultModel": "gpt-5.6-sol",
  "packages": ["npm:old@1.0.0"]
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := []string{"npm:pi-mcp-adapter@2.33.0", "npm:pi-subagents@0.67.0"}
	report := agentReport{Name: agentPi, Detected: true}
	agent := agentConfig{Name: agentPi, Packages: &want}
	if err := augmentPiPackageReport(&report, agent, home); err != nil {
		t.Fatal(err)
	}
	if len(report.DriftedPackage) != 2 || len(report.UpdatesPackage) != 1 {
		t.Fatalf("unexpected drift report: %#v", report)
	}
	if err := applyAgentPackageSync([]agentReport{report}, []agentConfig{agent}, home); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		DefaultProvider string   `json:"defaultProvider"`
		DefaultModel    string   `json:"defaultModel"`
		Packages        []string `json:"packages"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.DefaultProvider != "openai-codex" || settings.DefaultModel != "gpt-5.6-sol" {
		t.Fatalf("unrelated settings changed: %#v", settings)
	}
	if !reflect.DeepEqual(settings.Packages, want) {
		t.Fatalf("packages = %#v, want %#v", settings.Packages, want)
	}

	var synced agentReport
	if err := augmentPiPackageReport(&synced, agent, home); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(synced.ManagedPackage, want) || len(synced.DriftedPackage) != 0 {
		t.Fatalf("unexpected synced report: %#v", synced)
	}
}

func TestPiPackageSyncReplacesFilteredEntries(t *testing.T) {
	home := t.TempDir()
	path := piSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"packages":[{"source":"npm:old","skills":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	report := agentReport{Name: agentPi, Detected: true}
	packages := []string{"npm:new@1.0.0"}
	agent := agentConfig{Name: agentPi, Packages: &packages}
	if err := augmentPiPackageReport(&report, agent, home); err != nil {
		t.Fatal(err)
	}
	if len(report.DriftedPackage) != 1 || !reflect.DeepEqual(report.RemovesPackage, []string{"filtered package entries"}) {
		t.Fatalf("expected filtered entry replacement to be destructive drift: %#v", report)
	}
}

func TestPiPackageSyncReportsExplicitEmptyListDrift(t *testing.T) {
	home := t.TempDir()
	path := piSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"packages":["npm:old@1.0.0"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	packages := []string{}
	report := agentReport{Name: agentPi, Detected: true}
	if err := augmentPiPackageReport(&report, agentConfig{Name: agentPi, Packages: &packages}, home); err != nil {
		t.Fatal(err)
	}
	if len(report.DriftedPackage) != 1 || !reflect.DeepEqual(report.RemovesPackage, []string{"npm:old@1.0.0"}) || isReportSynced(report) {
		t.Fatalf("explicit empty list must report destructive drift: %#v", report)
	}
}

func TestPiPackageConfigPreservesAbsentVersusEmpty(t *testing.T) {
	type wrapper struct {
		Agent agentConfig `yaml:"agent"`
	}
	cases := []struct {
		name     string
		packages *[]string
		contains string
	}{
		{name: "absent", packages: nil, contains: "skill_root: \"\"\n"},
		{name: "empty", packages: &[]string{}, contains: "packages: []"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := yaml.Marshal(wrapper{Agent: agentConfig{Name: agentPi, Packages: tc.packages}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.contains) {
				t.Fatalf("YAML %q does not contain %q", data, tc.contains)
			}
			var decoded wrapper
			if err := yaml.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if (decoded.Agent.Packages == nil) != (tc.packages == nil) {
				t.Fatalf("round trip changed package presence: %q", data)
			}
		})
	}
}

func TestPiPackageSyncRejectsNullSettings(t *testing.T) {
	home := t.TempDir()
	path := piSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packages := []string{"npm:new@1.0.0"}
	if err := augmentPiPackageReport(&agentReport{}, agentConfig{Name: agentPi, Packages: &packages}, home); err == nil {
		t.Fatal("expected null settings to be rejected")
	}
	if err := syncPiPackages(home, packages); err == nil {
		t.Fatal("expected null settings to be rejected without panic")
	}
}

func TestPackagesArePiOnly(t *testing.T) {
	home := t.TempDir()
	cfg := config{Agents: []agentConfig{{
		Name:      agentCodex,
		Enabled:   true,
		SkillRoot: filepath.Join(home, ".codex", "skills"),
		Packages:  packageList("npm:example@1.0.0"),
	}}}
	if err := validateConfig(&cfg, home, false); err == nil {
		t.Fatal("expected non-Pi packages to be rejected")
	}
}

func packageList(values ...string) *[]string {
	return &values
}
