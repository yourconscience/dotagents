package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefuseWorktreeRoot(t *testing.T) {
	repo := t.TempDir()
	if err := refuseWorktreeRoot(repo); err != nil {
		t.Fatalf("canonical root must be accepted: %v", err)
	}

	worktree := filepath.Join(repo, ".worktrees", "sync-main")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := refuseWorktreeRoot(worktree); err == nil {
		t.Fatal("worktree root must be refused")
	} else if !strings.Contains(err.Error(), "canonical checkout") {
		t.Fatalf("unexpected error: %v", err)
	}

	// A symlink aliasing a worktree root resolves to the same refusal.
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(worktree, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := refuseWorktreeRoot(alias); err == nil {
		t.Fatal("symlinked worktree root must be refused")
	}
}
