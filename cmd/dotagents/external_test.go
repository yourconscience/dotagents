package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/example/single-skill", "single-skill"},
		{"https://github.com/example/single-skill.git", "single-skill"},
		{"git@github.com:example/single-skill.git", "single-skill"},
		{"https://github.com/user/repo/", "repo"},
		{"simple-name", "simple-name"},
	}
	for _, tt := range tests {
		got := repoName(tt.url)
		if got != tt.want {
			t.Errorf("repoName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func makeGitDir(t *testing.T, cachePath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(cachePath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverExternalSkillsSingleSkill(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	repoDir := filepath.Join(cacheRoot, "single-skill", "skill")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	makeGitDir(t, filepath.Join(cacheRoot, "single-skill"))
	if err := os.WriteFile(filepath.Join(repoDir, "SKILL.md"), []byte("---\nname: single-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/example/single-skill", SkillDir: "skill", Branch: "main"},
	}

	result, err := discoverExternalSkills(sources, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(result))
	}
	if _, ok := result["single-skill"]; !ok {
		t.Fatalf("expected skill named 'single-skill', got %v", result)
	}
	if result["single-skill"] != repoDir {
		t.Fatalf("expected path %s, got %s", repoDir, result["single-skill"])
	}
}

func TestDiscoverExternalSkillsMultiSkill(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	makeGitDir(t, filepath.Join(cacheRoot, "shared-skills"))
	skillsDir := filepath.Join(cacheRoot, "shared-skills", "skills")
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/user/shared-skills", SkillDir: "skills", Branch: "main"},
	}

	result, err := discoverExternalSkills(sources, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(result), result)
	}
	for _, name := range []string{"alpha", "beta"} {
		if _, ok := result[name]; !ok {
			t.Errorf("missing skill %q", name)
		}
	}
}

func TestDiscoverExternalSkillsCollision(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	for _, repo := range []string{"repo-a", "repo-b"} {
		makeGitDir(t, filepath.Join(cacheRoot, repo))
		dir := filepath.Join(cacheRoot, repo, "skills", "samename")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: samename\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/user/repo-a", SkillDir: "skills", Branch: "main"},
		{URL: "https://github.com/user/repo-b", SkillDir: "skills", Branch: "main"},
	}

	_, err := discoverExternalSkills(sources, home)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
}

func TestDiscoverExternalSkillsNotCloned(t *testing.T) {
	home := t.TempDir()

	sources := []externalSkillSource{
		{URL: "https://github.com/user/nonexistent", SkillDir: "skill", Branch: "main"},
	}

	_, err := discoverExternalSkills(sources, home)
	if err == nil {
		t.Fatal("expected error for not-cloned external source, got nil")
	}
}

func TestDiscoverExternalSkillsBadSkillDir(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	repoDir := filepath.Join(cacheRoot, "single-skill")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/example/single-skill", SkillDir: "wrong-dir", Branch: "main"},
	}

	_, err := discoverExternalSkills(sources, home)
	if err == nil {
		t.Fatal("expected error for bad skill_dir, got nil")
	}
}

func TestDiscoverExternalSkillsZeroSkills(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	repoDir := filepath.Join(cacheRoot, "empty-repo")
	makeGitDir(t, repoDir)
	if err := os.MkdirAll(filepath.Join(repoDir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/user/empty-repo", SkillDir: "skills", Branch: "main"},
	}

	_, err := discoverExternalSkills(sources, home)
	if err == nil {
		t.Fatal("expected error for zero skills, got nil")
	}
}

func TestDiscoverExternalSkillsNonGitDir(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	repoDir := filepath.Join(cacheRoot, "not-a-repo", "skill")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/user/not-a-repo", SkillDir: "skill", Branch: "main"},
	}

	_, err := discoverExternalSkills(sources, home)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
}

func TestIsExternalSkillLink(t *testing.T) {
	home := t.TempDir()
	extDir := filepath.Join(home, ".agents", "external", "single-skill", "skill")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skillRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(skillRoot, "single-skill")
	if err := os.Symlink(extDir, linkPath); err != nil {
		t.Fatal(err)
	}

	rawTarget, _ := os.Readlink(linkPath)
	if !isExternalSkillLink(linkPath, rawTarget, home) {
		t.Error("expected link to be recognized as external skill link")
	}

	localDir := filepath.Join(home, ".agents", "skills", "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	localLink := filepath.Join(skillRoot, "local")
	if err := os.Symlink(localDir, localLink); err != nil {
		t.Fatal(err)
	}
	rawLocal, _ := os.Readlink(localLink)
	if isExternalSkillLink(localLink, rawLocal, home) {
		t.Error("local skill link should not be recognized as external")
	}
}
