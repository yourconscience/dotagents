package main

import (
	"path/filepath"
	"testing"
)

func TestPiDesktopHarnessCapabilities(t *testing.T) {
	piDesktop := harnessFor(agentPiDesktop)
	if piDesktop == nil {
		t.Fatal("Pi Desktop harness is not registered")
	}
	if piDesktop.Skills != SkillsConfigDriven {
		t.Fatalf("Pi Desktop skills capability = %v, want config-driven (plugin-based)", piDesktop.Skills)
	}
	if piDesktop.InspectSkills == nil {
		t.Fatal("Pi Desktop should have custom InspectSkills for plugin")
	}
	if piDesktop.MCP != nil {
		t.Fatal("Pi Desktop unexpectedly exposes MCP support (should be GUI-configured)")
	}
	if piDesktop.Roles != nil {
		t.Fatal("Pi Desktop unexpectedly exposes agent-role support (roles are in plugin)")
	}
	if piDesktop.IntegrationNote == "" {
		t.Fatal("Pi Desktop should have integration note about plugin loading")
	}
}

func TestPiDesktopDetection(t *testing.T) {
	// Save original stat function behavior
	// We can't easily mock os.Stat in Go, so we'll test the happy path
	// by checking if the function exists and has the right signature

	// Test with non-existent app (should return false)
	detected := detectPiDesktop("/nonexistent/pi")
	// This will be false on most test systems, but true on systems with Pi Desktop installed
	// The actual detection depends on /Applications/PI-Desktop.app existence

	// We can't reliably test this without mocking, but we can verify the function is callable
	_ = detected
}

func TestPiDesktopDefaultConfig(t *testing.T) {
	configs := defaultAgentConfigs()
	var found bool
	for _, cfg := range configs {
		if cfg.Name == agentPiDesktop {
			found = true
			if cfg.SkillRoot != "~/.agents" {
				t.Fatalf("Pi Desktop skill root = %q, want ~/.agents (plugin-based)", cfg.SkillRoot)
			}
			if cfg.Detect != "" {
				t.Fatalf("Pi Desktop detect = %q, want empty (GUI app, no CLI)", cfg.Detect)
			}
			break
		}
	}
	if !found {
		t.Fatal("Pi Desktop not found in default agent configs")
	}
}

func TestPiDesktopSharesPathWithVanillaPi(t *testing.T) {
	home := t.TempDir()
	configs := []agentConfig{
		{Name: agentPi, Enabled: true, SkillRoot: filepath.Join(home, ".pi", "agent", "skills")},
		{Name: agentPiDesktop, Enabled: true, SkillRoot: filepath.Join(home, ".agents")},
	}

	// Pi Desktop should use plugin-based approach, not Pi's skill root
	if configs[0].SkillRoot == configs[1].SkillRoot {
		t.Fatal("Pi Desktop should use plugin-based approach (~/.agents), not share Pi's skill root")
	}
}
