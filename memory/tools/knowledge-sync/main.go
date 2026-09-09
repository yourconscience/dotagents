package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func run(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// canonicalCollection is the one memsearch collection the automatic freshness
// triggers keep fresh; it is not configurable so it cannot drift.
const canonicalCollection = "ai"

func git(dir string, args ...string) (string, error) {
	return run(dir, "git", args...)
}

// commitPinned commits with the identity pinned two ways so a partial/stale git
// env in the launchd context cannot leak a host-detected identity past the pin:
// the `-c user.*` config flags, and the GIT_AUTHOR_*/GIT_COMMITTER_* env (which
// takes precedence over `-c`) both forced to the resolved name/email.
func commitPinned(dir, name, email, msg string) (string, error) {
	cmd := exec.Command("git",
		"-c", "user.name="+name, "-c", "user.email="+email,
		"commit", "-m", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal("user home", err, "")
	}
	repo := getenv("KNOWLEDGE_REPO", filepath.Join(home, "Workspace", "knowledge"))
	remote := getenv("KNOWLEDGE_REMOTE", "vps")
	branch := getenv("KNOWLEDGE_BRANCH", "main")

	lockPath := filepath.Join(repo, ".git", "knowledge-sync.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		fatal("open lock", err, "")
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Println("knowledge-sync: another sync is running")
		return
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()

	if out, err := git(repo, "status", "--short"); err != nil {
		fatal("git status", err, out)
	} else if strings.TrimSpace(out) != "" {
		if out, err := git(repo, "add", "-A"); err != nil {
			fatal("git add", err, out)
		}
		// Pin an explicit identity so launchd runs (which lack GIT_AUTHOR_* and
		// may have no git config) can never fall back to a host-detected
		// user@hostname identity. See resolveIdentity and README R6.
		name, email, err := resolveIdentity(repo)
		if err != nil {
			fatal("git identity", err, "")
		}
		msg := "sync knowledge " + time.Now().UTC().Format("2006-01-02T15:04:05Z")
		if out, err := commitPinned(repo, name, email, msg); err != nil {
			// Nothing to commit after add is harmless; anything else is not.
			if !strings.Contains(out, "nothing to commit") && !strings.Contains(out, "no changes added") {
				fatal("git commit", err, out)
			}
		} else {
			fmt.Print(out)
		}
	}
	if out, err := git(repo, "rev-parse", "--abbrev-ref", "HEAD"); err != nil {
		fatal("current branch", err, out)
	} else if current := strings.TrimSpace(out); current != branch {
		fatal("branch guard", fmt.Errorf("checkout is on %q, not %q", current, branch),
			"knowledge-sync: refusing to sync: worktree checked out on "+current+", expected "+branch+"\n")
	}

	if out, err := git(repo, "fetch", remote, branch); err != nil {
		fatal("git fetch", err, out)
	}
	if out, err := git(repo, "merge", "--no-edit", remote+"/"+branch); err != nil {
		fmt.Print(out)
		conflict := "sync-conflict-" + time.Now().UTC().Format("20060102T150405Z")
		_, _ = git(repo, "branch", conflict)
		_, _ = git(repo, "merge", "--abort")
		fatal("git merge", err, "created conflict branch "+conflict+"\n")
	}
	if out, err := git(repo, "push", remote, branch); err != nil {
		fatal("git push", err, out)
	} else if strings.TrimSpace(out) != "" {
		fmt.Print(out)
	}
	fmt.Println("knowledge-sync: ok")

	// R2: the vault is now settled (pulled/merged/pushed); refresh the derived
	// memsearch index. Best-effort and bounded so it never blocks the sync, and
	// guarded by a shared lock so it never overlaps a concurrent refresh (e.g.
	// the reindex-after-capture trigger). The git work above is already done.
	reindexAfterSync(repo)
}

// resolveIdentity returns the git author/committer identity to pin on
// knowledge-sync commits. Resolution order:
//  1. KNOWLEDGE_GIT_AUTHOR_NAME / KNOWLEDGE_GIT_AUTHOR_EMAIL (dedicated pin,
//     e.g. set in the LaunchAgent plist).
//  2. GIT_AUTHOR_NAME / GIT_AUTHOR_EMAIL (standard git env used by interactive
//     agent sessions).
//  3. repo-local or global `git config user.name` / `user.email`.
//
// If none resolve, it returns an error rather than letting git synthesize a
// host-detected user@hostname identity (the historical source of the
// "configured automatically based on your username and hostname" warning).
func resolveIdentity(repo string) (name, email string, err error) {
	name = firstNonEmpty(os.Getenv("KNOWLEDGE_GIT_AUTHOR_NAME"), os.Getenv("GIT_AUTHOR_NAME"))
	email = firstNonEmpty(os.Getenv("KNOWLEDGE_GIT_AUTHOR_EMAIL"), os.Getenv("GIT_AUTHOR_EMAIL"))
	if name == "" {
		name = gitConfigValue(repo, "user.name")
	}
	if email == "" {
		email = gitConfigValue(repo, "user.email")
	}
	if name == "" || email == "" {
		return "", "", fmt.Errorf("no git identity configured; set repo-local `git -C %s config user.name/user.email` "+
			"or KNOWLEDGE_GIT_AUTHOR_NAME/KNOWLEDGE_GIT_AUTHOR_EMAIL (refusing a host-detected identity)", repo)
	}
	return name, email, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// gitConfigValue reads a git config key from the repo (local + global), returning
// "" if unset or on error.
func gitConfigValue(repo, key string) string {
	out, err := git(repo, "config", "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// reindexAfterSync runs a bounded, best-effort, non-overlapping memsearch index
// refresh after the vault settles. It mirrors the capture-side trigger
// (memory/hooks/common.sh refresh_index_async) exactly so the two mutually
// exclude: same lock path (MEMSEARCH_STATE_DIR/reindex.lock), same atomic
// mkdir + pid + stale-recovery convention, and the same index scope
// (notes + profile + sessions/*.md) into the canonical "ai" collection. It is a
// no-op when memsearch is absent and never fails the sync (git work is done).
func reindexAfterSync(repo string) {
	bin, err := exec.LookPath("memsearch")
	if err != nil {
		return // sync-only node without a local index engine
	}

	stateDir := reindexStateDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return // best-effort: cannot prepare the state dir, skip
	}
	lockPath := filepath.Join(stateDir, "reindex.lock")
	if !acquireReindexLock(lockPath) {
		fmt.Println("knowledge-sync: reindex already running, skipped")
		return
	}
	defer releaseReindexLock(lockPath)

	knowledge := getenv("KNOWLEDGE_DIR", repo)
	paths := reindexIndexPaths(knowledge)

	ctx, cancel := context.WithTimeout(context.Background(), reindexTimeout())
	defer cancel()
	// Always the canonical "ai" collection; MEMSEARCH_COLLECTION drift is ignored
	// so `ai` cannot go silently stale.
	args := append([]string{"index"}, paths...)
	args = append(args, "--collection", canonicalCollection)
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		fmt.Println("knowledge-sync: reindex timed out (best-effort), skipped")
		return
	}
	if err != nil {
		fmt.Printf("knowledge-sync: reindex warning: %s\n", strings.TrimSpace(string(out)))
		return
	}
	fmt.Println("knowledge-sync: reindex ok")
}

// reindexStateDir resolves the engine state dir the same way the hooks do:
// MEMSEARCH_STATE_DIR, else ~/.memsearch/state. The reindex lock lives here so
// the sync-side and capture-side triggers share exactly one lock.
func reindexStateDir() string {
	if v := os.Getenv("MEMSEARCH_STATE_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".memsearch", "state")
	}
	return filepath.Join(home, ".memsearch", "state")
}

// reindexIndexPaths mirrors the capture side's scope: the notes and profile
// directories plus every sessions/*.md and *.markdown file.
func reindexIndexPaths(knowledge string) []string {
	notes := getenv("NOTES_DIR", filepath.Join(knowledge, "notes"))
	profile := getenv("PROFILE_DIR", filepath.Join(knowledge, "profile"))
	sessions := getenv("SESSIONS_DIR", filepath.Join(knowledge, "sessions"))
	paths := []string{notes, profile}
	for _, pat := range []string{"*.md", "*.markdown"} {
		matches, _ := filepath.Glob(filepath.Join(sessions, pat))
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && !fi.IsDir() {
				paths = append(paths, m)
			}
		}
	}
	return paths
}

// acquireReindexLock implements the capture side's atomic-mkdir lock with pid
// and stale-recovery. mkdir is atomic: it fails when a refresh already holds the
// lock. A lock whose recorded owner is gone (e.g. SIGKILLed mid-run) is
// reclaimed so a dead process can't suppress reindex forever. Returns true when
// the lock is held by this process.
func acquireReindexLock(lockPath string) bool {
	if err := os.Mkdir(lockPath, 0o755); err == nil {
		writePidFile(lockPath)
		return true
	}
	// Lock exists: reclaim only if its owner is no longer alive.
	if owner := readPidFile(lockPath); owner > 0 && processAlive(owner) {
		return false
	}
	_ = os.Remove(filepath.Join(lockPath, "pid"))
	_ = os.Remove(lockPath)
	if err := os.Mkdir(lockPath, 0o755); err != nil {
		return false
	}
	writePidFile(lockPath)
	return true
}

func releaseReindexLock(lockPath string) {
	_ = os.Remove(filepath.Join(lockPath, "pid"))
	_ = os.Remove(lockPath)
}

func writePidFile(lockPath string) {
	_ = os.WriteFile(filepath.Join(lockPath, "pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
}

func readPidFile(lockPath string) int {
	data, err := os.ReadFile(filepath.Join(lockPath, "pid"))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// processAlive reports whether a pid is a live process, mirroring `kill -0`.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// reindexTimeout bounds the best-effort refresh, matching the capture side's
// watchdog knob MEMSEARCH_REINDEX_TIMEOUT (seconds); defaults to 120s.
func reindexTimeout() time.Duration {
	if v := os.Getenv("MEMSEARCH_REINDEX_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 120 * time.Second
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(step string, err error, out string) {
	if out != "" {
		fmt.Print(out)
	}
	fmt.Fprintf(os.Stderr, "knowledge-sync: %s failed: %v\n", step, err)
	os.Exit(1)
}
