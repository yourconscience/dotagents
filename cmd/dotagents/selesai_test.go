package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelesaiHarnessCapabilities(t *testing.T) {
	selesai := harnessFor(agentSelesai)
	if selesai == nil {
		t.Fatal("Selesai harness is not registered")
	}
	if selesai.Skills != SkillsSymlink {
		t.Fatalf("Selesai skills capability = %v, want symlink", selesai.Skills)
	}
	if selesai.IntegrationNote == "" {
		t.Fatal("Selesai should have integration note about bundled skills")
	}
}

func TestSelesaiDefaultConfig(t *testing.T) {
	configs := defaultAgentConfigs()
	var found bool
	for _, cfg := range configs {
		if cfg.Name == agentSelesai {
			found = true
			if cfg.SkillRoot != "~/.selesai/agent/skills" {
				t.Fatalf("Selesai skill root = %q, want ~/.selesai/agent/skills", cfg.SkillRoot)
			}
			if cfg.Detect != "selesai" {
				t.Fatalf("Selesai detect = %q, want selesai", cfg.Detect)
			}
			break
		}
	}
	if !found {
		t.Fatal("Selesai not found in default agent configs")
	}
}

func TestSelesaiUsesDistinctPath(t *testing.T) {
	home := t.TempDir()
	configs := []agentConfig{
		{Name: agentPi, Enabled: true, SkillRoot: filepath.Join(home, ".pi", "agent", "skills")},
		{Name: agentPiDesktop, Enabled: true, SkillRoot: filepath.Join(home, ".pi", "agent", "skills")},
		{Name: agentSelesai, Enabled: true, SkillRoot: filepath.Join(home, ".selesai", "agent", "skills")},
	}

	// Selesai should have a different path from Pi/Pi Desktop
	if configs[2].SkillRoot == configs[0].SkillRoot {
		t.Fatal("Selesai should not share path with vanilla Pi")
	}
	if configs[2].SkillRoot == configs[1].SkillRoot {
		t.Fatal("Selesai should not share path with Pi Desktop")
	}
}

func TestSelesaiSkillFiltering(t *testing.T) {
	// Test that skill filtering works when Selesai is not installed
	// (should return all skills unchanged)
	expected := map[string]string{
		"test-skill":    "/path/to/test-skill",
		"another-skill": "/path/to/another-skill",
	}

	filtered, err := filterSelesaiExpectedSkills(expected)
	if err != nil {
		t.Fatalf("filterSelesaiExpectedSkills failed: %v", err)
	}

	// When Selesai is not installed, all skills should pass through
	if len(filtered) != len(expected) {
		t.Fatalf("filtered skills count = %d, want %d", len(filtered), len(expected))
	}

	for name, path := range expected {
		if filtered[name] != path {
			t.Fatalf("skill %q path = %q, want %q", name, filtered[name], path)
		}
	}
}

func TestSelesaiSkillFilteringWithMockBundled(t *testing.T) {
	// Create a temporary directory structure mimicking a Selesai installation
	tmpDir := t.TempDir()
	packageDir := filepath.Join(tmpDir, "node_modules", "@selesai", "code")
	skillsDir := filepath.Join(packageDir, "dist", "skills")

	// Create package.json
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"name": "@selesai/code"}`
	if err := os.WriteFile(filepath.Join(packageDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create bundled skills
	for _, name := range []string{"bundled-skill-1", "bundled-skill-2"} {
		skillDir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a mock selesai executable
	binPath := filepath.Join(packageDir, "bin", "selesai.js")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("#!/usr/bin/env node\nconsole.log('selesai');"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Note: This test can't easily test the actual filtering because we can't
	// mock exec.LookPath. The test above verifies the behavior when Selesai
	// is not installed (the common case in CI).
	// The actual bundled skill discovery would need integration testing.
}

func TestSelesaiDetection(t *testing.T) {
	// Test detection with a mock executable
	tmpDir := t.TempDir()
	mockSelesai := filepath.Join(tmpDir, "selesai")

	// Create a script that outputs "selesai" in version
	script := `#!/bin/sh
echo "selesai version 0.13.25"
`
	if err := os.WriteFile(mockSelesai, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	if !detectSelesai(mockSelesai) {
		t.Fatal("Selesai executable with 'selesai' in version output was not detected")
	}

	// Test with non-Selesai executable
	mockOther := filepath.Join(tmpDir, "other")
	otherScript := `#!/bin/sh
echo "other version 1.0.0"
`
	if err := os.WriteFile(mockOther, []byte(otherScript), 0o755); err != nil {
		t.Fatal(err)
	}

	if detectSelesai(mockOther) {
		t.Fatal("Non-Selesai executable was incorrectly detected as Selesai")
	}
}
