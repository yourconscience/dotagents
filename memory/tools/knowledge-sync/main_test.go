package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	// Isolate from any machine-global/system git identity so resolution is
	// deterministic and only repo-local config counts.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
}

func TestResolveIdentityPrefersKnowledgeEnv(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Setenv("KNOWLEDGE_GIT_AUTHOR_NAME", "Pin Name")
	t.Setenv("KNOWLEDGE_GIT_AUTHOR_EMAIL", "pin@example.com")
	t.Setenv("GIT_AUTHOR_NAME", "Env Name")
	t.Setenv("GIT_AUTHOR_EMAIL", "env@example.com")

	name, email, err := resolveIdentity(dir)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if name != "Pin Name" || email != "pin@example.com" {
		t.Errorf("got %q <%q>, want dedicated KNOWLEDGE_GIT_* pin", name, email)
	}
}

func TestResolveIdentityFallsBackToGitEnvThenConfig(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	// No KNOWLEDGE_GIT_* pin; standard git env wins over config.
	t.Setenv("KNOWLEDGE_GIT_AUTHOR_NAME", "")
	t.Setenv("KNOWLEDGE_GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_AUTHOR_NAME", "Env Name")
	t.Setenv("GIT_AUTHOR_EMAIL", "env@example.com")
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Cfg Name").CombinedOutput(); err != nil {
		t.Fatalf("config name: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "cfg@example.com").CombinedOutput(); err != nil {
		t.Fatalf("config email: %v %s", err, out)
	}

	name, email, err := resolveIdentity(dir)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if name != "Env Name" || email != "env@example.com" {
		t.Errorf("got %q <%q>, want GIT_AUTHOR_* env preference", name, email)
	}

	// With no env at all, repo-local git config is used.
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	name, email, err = resolveIdentity(dir)
	if err != nil {
		t.Fatalf("resolveIdentity(config): %v", err)
	}
	if name != "Cfg Name" || email != "cfg@example.com" {
		t.Errorf("got %q <%q>, want repo-local config fallback", name, email)
	}
}

func TestResolveIdentityRefusesHostDetectedFallback(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	// No identity from any source: must error rather than let git synthesize a
	// user@hostname identity.
	t.Setenv("KNOWLEDGE_GIT_AUTHOR_NAME", "")
	t.Setenv("KNOWLEDGE_GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")

	if _, _, err := resolveIdentity(dir); err == nil {
		t.Fatal("expected resolveIdentity to refuse with no identity configured")
	}
}

func TestCommitPinnedOverridesLeakyEnv(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	// A stale/partial git env in the ambient process must not leak into the
	// commit -- commitPinned forces the resolved identity for author + committer.
	t.Setenv("GIT_AUTHOR_NAME", "Leaky")
	t.Setenv("GIT_AUTHOR_EMAIL", "leak@bad")
	t.Setenv("GIT_COMMITTER_NAME", "Leaky")
	t.Setenv("GIT_COMMITTER_EMAIL", "leak@bad")

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("add: %v %s", err, out)
	}
	if out, err := commitPinned(dir, "Pinned Name", "pin@example.com", "msg"); err != nil {
		t.Fatalf("commitPinned: %v %s", err, out)
	}
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%an|%ae|%cn|%ce").CombinedOutput()
	if err != nil {
		t.Fatalf("log: %v %s", err, out)
	}
	got := strings.TrimSpace(string(out))
	want := "Pinned Name|pin@example.com|Pinned Name|pin@example.com"
	if got != want {
		t.Errorf("commit identity leaked: got %q, want %q", got, want)
	}
}
