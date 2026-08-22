package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		msg := "sync knowledge " + time.Now().UTC().Format("2006-01-02T15:04:05Z")
		if out, err := git(repo, "commit", "-m", msg); err != nil {
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
