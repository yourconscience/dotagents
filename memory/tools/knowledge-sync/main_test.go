package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func TestReindexStateDirMatchesCaptureDefault(t *testing.T) {
	t.Setenv("MEMSEARCH_STATE_DIR", "")
	got := reindexStateDir()
	if filepath.Base(got) != "state" || filepath.Base(filepath.Dir(got)) != ".memsearch" {
		t.Errorf("default state dir = %q, want ~/.memsearch/state", got)
	}
	t.Setenv("MEMSEARCH_STATE_DIR", "/custom/state")
	if got := reindexStateDir(); got != "/custom/state" {
		t.Errorf("env override = %q, want /custom/state", got)
	}
}

func TestReindexIndexPathsMirrorCaptureScope(t *testing.T) {
	t.Setenv("NOTES_DIR", "")
	t.Setenv("PROFILE_DIR", "")
	t.Setenv("SESSIONS_DIR", "")
	root := t.TempDir()
	for _, d := range []string{"notes", "profile", "sessions", "sessions/sub"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"sessions/a.md", "sessions/b.markdown", "sessions/c.txt", "sessions/sub/deep.md"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := reindexIndexPaths(root)
	want := []string{
		filepath.Join(root, "notes"),
		filepath.Join(root, "profile"),
		filepath.Join(root, "sessions", "a.md"),
		filepath.Join(root, "sessions", "b.markdown"),
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("index paths = %v, want %v", got, want)
	}
}

func TestAcquireReindexLockExcludesAndRecovers(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "reindex.lock")

	// First acquire succeeds and records this live process as owner.
	if !acquireReindexLock(lock) {
		t.Fatal("first acquire should succeed")
	}
	if fi, err := os.Stat(lock); err != nil || !fi.IsDir() {
		t.Fatalf("lock must be a directory: %v", err)
	}
	if readPidFile(lock) != os.Getpid() {
		t.Errorf("pid file = %d, want %d", readPidFile(lock), os.Getpid())
	}
	// A second acquire while the live owner holds it must be refused.
	if acquireReindexLock(lock) {
		t.Error("second acquire should be refused while owner is alive")
	}
	releaseReindexLock(lock)
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("release should remove the lock directory")
	}

	// A lock stranded by a dead owner is reclaimed.
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatalf("spawn dead proc: %v", err)
	}
	deadPid := dead.Process.Pid
	if err := os.Mkdir(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lock, "pid"), []byte(strconv.Itoa(deadPid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !acquireReindexLock(lock) {
		t.Error("acquire should reclaim a lock held by a dead owner")
	}
	if readPidFile(lock) != os.Getpid() {
		t.Errorf("reclaimed pid file = %d, want %d", readPidFile(lock), os.Getpid())
	}
	releaseReindexLock(lock)
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
