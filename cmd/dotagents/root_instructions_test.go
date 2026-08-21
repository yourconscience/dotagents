package main

import (
	"os"
	"path/filepath"
	"testing"
)

// droidRootInstructions returns the RootInstructionsCapability for the droid harness.
// Used by tests to exercise inspectRootInstructions with droid-specific paths.
func droidRootInstructions() *RootInstructionsCapability {
	h := harnessFor(agentDroid)
	if h == nil || h.RootInstructions == nil {
		panic("droid harness missing RootInstructions")
	}
	return h.RootInstructions
}

func TestInspectDroidRootInstructionsMissing(t *testing.T) {
	home := t.TempDir()
	report := agentReport{}

	if err := inspectRootInstructions(&report, droidRootInstructions(), filepath.Join(home, ".agents"), home); err != nil {
		t.Fatal(err)
	}

	if report.RootState != stateMissing {
		t.Fatalf("RootState = %q, want %q", report.RootState, stateMissing)
	}
	if report.RootExpected != filepath.Join(home, ".agents", "AGENTS.md") {
		t.Fatalf("RootExpected = %q", report.RootExpected)
	}
}

func TestInspectDroidRootInstructionsSynced(t *testing.T) {
	home := t.TempDir()
	expected := filepath.Join(home, ".agents", "AGENTS.md")
	linkPath := filepath.Join(home, ".factory", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, []byte("# Shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(expected, linkPath); err != nil {
		t.Fatal(err)
	}

	report := agentReport{}
	if err := inspectRootInstructions(&report, droidRootInstructions(), filepath.Join(home, ".agents"), home); err != nil {
		t.Fatal(err)
	}

	if report.RootState != stateSynced {
		t.Fatalf("RootState = %q, want %q", report.RootState, stateSynced)
	}
}

func TestInspectDroidRootInstructionsConflict(t *testing.T) {
	home := t.TempDir()
	linkPath := filepath.Join(home, ".factory", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(linkPath, []byte("# Local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := agentReport{}
	if err := inspectRootInstructions(&report, droidRootInstructions(), filepath.Join(home, ".agents"), home); err != nil {
		t.Fatal(err)
	}

	if report.RootState != stateConflict {
		t.Fatalf("RootState = %q, want %q", report.RootState, stateConflict)
	}
	if len(report.Conflicts) != 1 {
		t.Fatalf("Conflicts = %#v, want one conflict", report.Conflicts)
	}
}

func TestApplyAgentRootInstructionSyncCreatesMissingLink(t *testing.T) {
	home := t.TempDir()
	report := agentReport{
		Name:         agentDroid,
		Detected:     true,
		RootPath:     filepath.Join(home, ".factory", "AGENTS.md"),
		RootExpected: filepath.Join(home, ".agents", "AGENTS.md"),
		RootState:    stateMissing,
	}
	if err := os.MkdirAll(filepath.Dir(report.RootExpected), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report.RootExpected, []byte("# Shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applyAgentRootInstructionSync([]agentReport{report}); err != nil {
		t.Fatal(err)
	}

	rawTarget, err := os.Readlink(report.RootPath)
	if err != nil {
		t.Fatal(err)
	}
	if rawTarget != report.RootExpected {
		t.Fatalf("link target = %q, want %q", rawTarget, report.RootExpected)
	}
}

func claudeCodeRootInstructions() *RootInstructionsCapability {
	h := harnessFor(agentClaudeCode)
	if h == nil || h.RootInstructions == nil {
		panic("claude-code harness missing RootInstructions")
	}
	return h.RootInstructions
}

func codexRootInstructions() *RootInstructionsCapability {
	h := harnessFor(agentCodex)
	if h == nil || h.RootInstructions == nil {
		panic("codex harness missing RootInstructions")
	}
	return h.RootInstructions
}

func TestRootInstructionsNativePaths(t *testing.T) {
	home := t.TempDir()
	repoRoot := filepath.Join(home, ".agents")
	for _, tc := range []struct {
		agent    string
		cap      *RootInstructionsCapability
		wantLink string
	}{
		{agentClaudeCode, claudeCodeRootInstructions(), filepath.Join(home, ".claude", "CLAUDE.md")},
		{agentCodex, codexRootInstructions(), filepath.Join(home, ".codex", "AGENTS.md")},
	} {
		if got := tc.cap.Path(home); got != tc.wantLink {
			t.Fatalf("%s Path = %q, want %q", tc.agent, got, tc.wantLink)
		}
		if got := tc.cap.Expected(repoRoot); got != filepath.Join(repoRoot, "AGENTS.md") {
			t.Fatalf("%s Expected = %q, want %q", tc.agent, got, filepath.Join(repoRoot, "AGENTS.md"))
		}
	}
}

func TestInspectClaudeAndCodexRootInstructions(t *testing.T) {
	for _, tc := range []struct {
		agent string
		cap   func() *RootInstructionsCapability
		dir   string
		file  string
	}{
		{agentClaudeCode, claudeCodeRootInstructions, ".claude", "CLAUDE.md"},
		{agentCodex, codexRootInstructions, ".codex", "AGENTS.md"},
	} {
		t.Run(tc.agent+"/missing", func(t *testing.T) {
			home := t.TempDir()
			report := agentReport{}
			if err := inspectRootInstructions(&report, tc.cap(), filepath.Join(home, ".agents"), home); err != nil {
				t.Fatal(err)
			}
			if report.RootState != stateMissing {
				t.Fatalf("RootState = %q, want %q", report.RootState, stateMissing)
			}
		})
		t.Run(tc.agent+"/synced", func(t *testing.T) {
			home := t.TempDir()
			expected := filepath.Join(home, ".agents", "AGENTS.md")
			linkPath := filepath.Join(home, tc.dir, tc.file)
			if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(expected, []byte("# Shared\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(expected, linkPath); err != nil {
				t.Fatal(err)
			}
			report := agentReport{}
			if err := inspectRootInstructions(&report, tc.cap(), filepath.Join(home, ".agents"), home); err != nil {
				t.Fatal(err)
			}
			if report.RootState != stateSynced {
				t.Fatalf("RootState = %q, want %q", report.RootState, stateSynced)
			}
		})
		t.Run(tc.agent+"/conflict", func(t *testing.T) {
			home := t.TempDir()
			linkPath := filepath.Join(home, tc.dir, tc.file)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(linkPath, []byte("# Local\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			report := agentReport{}
			if err := inspectRootInstructions(&report, tc.cap(), filepath.Join(home, ".agents"), home); err != nil {
				t.Fatal(err)
			}
			if report.RootState != stateConflict {
				t.Fatalf("RootState = %q, want %q", report.RootState, stateConflict)
			}
			if len(report.Conflicts) != 1 {
				t.Fatalf("Conflicts = %#v, want one conflict", report.Conflicts)
			}
		})
	}
}

func TestApplyAgentRootInstructionSyncClaudeAndCodex(t *testing.T) {
	for _, tc := range []struct {
		agent    string
		linkPath string
	}{
		{agentClaudeCode, ".claude/CLAUDE.md"},
		{agentCodex, ".codex/AGENTS.md"},
	} {
		home := t.TempDir()
		report := agentReport{
			Name:         tc.agent,
			Detected:     true,
			RootPath:     filepath.Join(home, filepath.FromSlash(tc.linkPath)),
			RootExpected: filepath.Join(home, ".agents", "AGENTS.md"),
			RootState:    stateMissing,
		}
		if err := os.MkdirAll(filepath.Dir(report.RootExpected), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(report.RootExpected, []byte("# Shared\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := applyAgentRootInstructionSync([]agentReport{report}); err != nil {
			t.Fatal(err)
		}

		rawTarget, err := os.Readlink(report.RootPath)
		if err != nil {
			t.Fatal(err)
		}
		if rawTarget != report.RootExpected {
			t.Fatalf("%s link target = %q, want %q", tc.agent, rawTarget, report.RootExpected)
		}
	}
}
