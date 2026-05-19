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
		{"https://github.com/yourconscience/dotknow", "dotknow"},
		{"https://github.com/yourconscience/dotknow.git", "dotknow"},
		{"git@github.com:yourconscience/dotknow.git", "dotknow"},
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

func TestDiscoverExternalSkillsSingleSkill(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

	repoDir := filepath.Join(cacheRoot, "dotknow", "skill")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "SKILL.md"), []byte("---\nname: dotknow\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sources := []externalSkillSource{
		{URL: "https://github.com/yourconscience/dotknow", SkillDir: "skill", Branch: "main"},
	}

	result, err := discoverExternalSkills(sources, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(result))
	}
	if _, ok := result["dotknow"]; !ok {
		t.Fatalf("expected skill named 'dotknow', got %v", result)
	}
	if result["dotknow"] != repoDir {
		t.Fatalf("expected path %s, got %s", repoDir, result["dotknow"])
	}
}

func TestDiscoverExternalSkillsMultiSkill(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".agents", "external")

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

func TestDiscoverExternalSkillsMissing(t *testing.T) {
	home := t.TempDir()

	sources := []externalSkillSource{
		{URL: "https://github.com/user/nonexistent", SkillDir: "skill", Branch: "main"},
	}

	result, err := discoverExternalSkills(sources, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 skills for missing cache, got %d", len(result))
	}
}

func TestIsExternalSkillLink(t *testing.T) {
	home := t.TempDir()
	extDir := filepath.Join(home, ".agents", "external", "dotknow", "skill")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skillRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(skillRoot, "dotknow")
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
