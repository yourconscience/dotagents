package main

import (
	"os/exec"
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
		t.Fatalf("config: %v %s", err, out)
	}
	exec.Command("git", "-C", dir, "config", "user.email", "cfg@example.com").Run()

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
