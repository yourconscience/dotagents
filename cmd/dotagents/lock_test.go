package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLockFileMissing(t *testing.T) {
	repoRoot := t.TempDir()
	lock, err := readLockFile(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Version != 1 || len(lock.ExternalSkills) != 0 {
		t.Fatalf("expected empty v1 lock, got %+v", lock)
	}
}

func TestWriteAndReadLockFile(t *testing.T) {
	repoRoot := t.TempDir()
	want := lockFile{
		ExternalSkills: []externalLockEntry{
			{Name: "shared-skills", URL: "https://github.com/example/shared-skills", Branch: "main", Commit: "abc123def456"},
		},
	}
	if err := writeLockFile(repoRoot, want); err != nil {
		t.Fatal(err)
	}
	got, err := readLockFile(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 {
		t.Fatalf("expected version 1, got %d", got.Version)
	}
	if len(got.ExternalSkills) != 1 || got.ExternalSkills[0] != want.ExternalSkills[0] {
		t.Fatalf("round trip mismatch: %+v", got.ExternalSkills)
	}
}

func TestLockEntryFor(t *testing.T) {
	lock := lockFile{
		ExternalSkills: []externalLockEntry{
			{Name: "shared-skills", URL: "https://github.com/example/shared-skills", Branch: "main", Commit: "abc123"},
			{Name: "empty-pin", URL: "https://github.com/example/empty-pin", Branch: "main", Commit: ""},
		},
	}
	tests := []struct {
		name    string
		src     externalSkillSource
		wantPin bool
	}{
		{"match", externalSkillSource{URL: "https://github.com/example/shared-skills", Branch: "main"}, true},
		{"branch changed", externalSkillSource{URL: "https://github.com/example/shared-skills", Branch: "dev"}, false},
		{"url changed", externalSkillSource{URL: "https://github.com/other/shared-skills", Branch: "main"}, false},
		{"unknown source", externalSkillSource{URL: "https://github.com/example/new-repo", Branch: "main"}, false},
		{"empty commit", externalSkillSource{URL: "https://github.com/example/empty-pin", Branch: "main"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lockEntryFor(lock, tt.src)
			if (got != nil) != tt.wantPin {
				t.Fatalf("lockEntryFor() pin = %v, want %v", got != nil, tt.wantPin)
			}
		})
	}
}

func TestRebuildLockEntriesPreservesUnclonedPins(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	// repo-a is a real clone; repo-b only exists in the lock.
	repoA := filepath.Join(cacheRoot, "repo-a")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=test@test", "-c", "user.name=test", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repoA}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	headA := externalSkillCommitFull(repoA)
	if headA == "" {
		t.Fatal("expected repo-a HEAD")
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/example/repo-a", SkillDir: "skills", Branch: "main"},
		{URL: "https://github.com/example/repo-b", SkillDir: "skills", Branch: "main"},
	}
	lock := lockFile{ExternalSkills: []externalLockEntry{
		{Name: "repo-b", URL: "https://github.com/example/repo-b", Branch: "main", Commit: "feedface"},
	}}

	entries := rebuildLockEntries(sources, home, lock)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %+v", entries)
	}
	byName := make(map[string]externalLockEntry)
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	if byName["repo-a"].Commit != headA {
		t.Fatalf("repo-a should pin cache HEAD %s, got %+v", headA, byName["repo-a"])
	}
	if byName["repo-b"].Commit != "feedface" {
		t.Fatalf("repo-b pin should be preserved when not cloned, got %+v", byName["repo-b"])
	}
}

func TestSelectExternalSources(t *testing.T) {
	sources := []externalSkillSource{
		{URL: "https://github.com/example/repo-a", Branch: "main"},
		{URL: "https://github.com/example/repo-b", Branch: "main"},
	}
	all, err := selectExternalSources(sources, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("expected all sources, got %v (%v)", all, err)
	}
	one, err := selectExternalSources(sources, []string{"repo-b"})
	if err != nil || len(one) != 1 || repoName(one[0].URL) != "repo-b" {
		t.Fatalf("expected repo-b, got %v (%v)", one, err)
	}
	if _, err := selectExternalSources(sources, []string{"missing"}); err == nil {
		t.Fatal("expected error for unknown source name")
	} else if !strings.Contains(err.Error(), "repo-a") {
		t.Fatalf("error should list configured sources, got %v", err)
	}
}

func TestCheckExternalSkillLockUnpinned(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	cfg := config{ExternalSkills: []externalSkillSource{
		{URL: "https://github.com/example/shared-skills", SkillDir: "skills", Branch: "main"},
	}}
	result := checkExternalSkillLock(repoRoot, cfg, home)
	if result.status != checkStatusWarn {
		t.Fatalf("expected warn for unpinned source, got %s (%s)", result.status, result.detail)
	}
	if !strings.Contains(result.detail, "unpinned") {
		t.Fatalf("expected unpinned detail, got %q", result.detail)
	}
}

func TestCheckExternalSkillLockNoneConfigured(t *testing.T) {
	result := checkExternalSkillLock(t.TempDir(), config{}, t.TempDir())
	if result.status != checkStatusPass {
		t.Fatalf("expected pass, got %s (%s)", result.status, result.detail)
	}
}

func TestDiscoverExternalSkillsAllowlist(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")
	skillBase := filepath.Join(cacheRoot, "multi", "skills")
	for _, name := range []string{"alpha", "beta", "gamma"} {
		dir := filepath.Join(skillBase, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	makeGitDir(t, filepath.Join(cacheRoot, "multi"))

	sources := []externalSkillSource{
		{URL: "https://github.com/example/multi", SkillDir: "skills", Branch: "main", Skills: []string{"alpha", "gamma"}},
	}
	result, err := discoverExternalSkills(sources, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 skills, got %v", result)
	}
	if _, ok := result["beta"]; ok {
		t.Fatal("beta should be filtered out by the allowlist")
	}

	sources[0].Skills = []string{"alpha", "missing"}
	if _, err := discoverExternalSkills(sources, home); err == nil {
		t.Fatal("expected error for allowlisted skill that does not exist")
	}
}
