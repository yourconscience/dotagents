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

func git(dir string, args ...string) (string, error) {
	return run(dir, "git", args...)
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
		commitArgs := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "commit", "-m", msg}
		if out, err := git(repo, commitArgs...); err != nil {
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
// refresh over the canonical vault into collection "ai". It is a no-op when
// memsearch is not installed (sync-only nodes) or when another refresh holds the
// shared lock. It never fails the sync: the git work is already complete.
func reindexAfterSync(repo string) {
	bin, err := exec.LookPath("memsearch")
	if err != nil {
		return // sync-only node without a local index engine
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	stateDir := getenv("MEMSEARCH_HOME", filepath.Join(home, ".memsearch"))
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return
	}
	// Shared reindex lock: the same path is used by the reindex-after-capture
	// trigger so the two never overlap. Non-blocking: if a refresh is already
	// running, skip this one (the index will catch up on the next trigger).
	lockPath := filepath.Join(stateDir, "reindex.lock")
	rlock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = rlock.Close() }()
	if err := syscall.Flock(int(rlock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Println("knowledge-sync: reindex already running, skipped")
		return
	}
	defer func() { _ = syscall.Flock(int(rlock.Fd()), syscall.LOCK_UN) }()

	knowledge := getenv("KNOWLEDGE_DIR", repo)
	collection := getenv("MEMSEARCH_COLLECTION", "ai")
	ctx, cancel := context.WithTimeout(context.Background(), reindexTimeout())
	defer cancel()

	// Incremental by default (only files changed by the merge are re-embedded).
	cmd := exec.CommandContext(ctx, bin, "index", knowledge, "--collection", collection)
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

// reindexTimeout bounds the best-effort refresh. Override with
// MEMSEARCH_REINDEX_TIMEOUT_SECONDS; defaults to 300s.
func reindexTimeout() time.Duration {
	if v := os.Getenv("MEMSEARCH_REINDEX_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 300 * time.Second
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
