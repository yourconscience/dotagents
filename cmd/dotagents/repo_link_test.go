package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInspectRepoLinkInPlaceClone covers the documented `git clone ... ~/.agents`
// install path: the repo is a real directory at ~/.agents that already is
// repoRoot. No symlink is needed, so the state must be synced (not conflict).
func TestInspectRepoLinkInPlaceClone(t *testing.T) {
	home := t.TempDir()
	linkPath := filepath.Join(home, ".agents")
	if err := os.Mkdir(linkPath, 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := inspectRepoLink(linkPath, home)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != stateSynced {
		t.Fatalf("in-place clone: got state %q, want %q", report.State, stateSynced)
	}
	if report.ExpectedTarget != linkPath {
		t.Fatalf("in-place clone: got ExpectedTarget %q, want %q", report.ExpectedTarget, linkPath)
	}
}

// TestInspectRepoLinkSymlink covers the symlink layout: ~/.agents -> repoRoot
// living elsewhere (e.g. ~/Workspace/dotagents).
func TestInspectRepoLinkSymlink(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	linkPath := filepath.Join(home, ".agents")
	if err := os.Symlink(repoRoot, linkPath); err != nil {
		t.Fatal(err)
	}

	report, err := inspectRepoLink(repoRoot, home)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != stateSynced {
		t.Fatalf("symlink: got state %q, want %q", report.State, stateSynced)
	}
}

// TestInspectRepoLinkForeignDirConflict guards the safety case: a real directory
// at ~/.agents that is not the repo must still be reported as a conflict so the
// tool never clobbers unrelated user data.
func TestInspectRepoLinkForeignDirConflict(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	linkPath := filepath.Join(home, ".agents")
	if err := os.Mkdir(linkPath, 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := inspectRepoLink(repoRoot, home)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != stateConflict {
		t.Fatalf("foreign dir: got state %q, want %q", report.State, stateConflict)
	}
}

// TestInspectRepoLinkMissing covers a fresh machine with no ~/.agents yet.
func TestInspectRepoLinkMissing(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()

	report, err := inspectRepoLink(repoRoot, home)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != stateMissing {
		t.Fatalf("missing: got state %q, want %q", report.State, stateMissing)
	}
}
