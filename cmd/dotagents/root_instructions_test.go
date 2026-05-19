package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectDroidRootInstructionsMissing(t *testing.T) {
	home := t.TempDir()
	report := agentReport{}

	if err := inspectDroidRootInstructions(&report, home); err != nil {
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
	if err := inspectDroidRootInstructions(&report, home); err != nil {
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
	if err := inspectDroidRootInstructions(&report, home); err != nil {
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
