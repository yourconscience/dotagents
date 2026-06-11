package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initCodexPluginTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repoRoot
}

func writeCodexPluginTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderCodexPluginSkillsMirrorsTrackedAndPrunesStale(t *testing.T) {
	repoRoot := initCodexPluginTestRepo(t)
	writeCodexPluginTestFile(t, filepath.Join(repoRoot, "skills", "alpha", "SKILL.md"), "---\nname: alpha\n---\n")
	writeCodexPluginTestFile(t, filepath.Join(repoRoot, ".gitignore"), "skills/alpha/data/\n")
	writeCodexPluginTestFile(t, filepath.Join(repoRoot, "skills", "alpha", "data", "secret.txt"), "ignored\n")
	staleDest := filepath.Join(codexPluginSkillsDir(repoRoot), "removed", "SKILL.md")
	writeCodexPluginTestFile(t, staleDest, "old\n")

	if err := renderCodexPluginSkills(repoRoot); err != nil {
		t.Fatal(err)
	}

	rendered, err := os.ReadFile(filepath.Join(codexPluginSkillsDir(repoRoot), "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected rendered skill: %v", err)
	}
	if string(rendered) != "---\nname: alpha\n---\n" {
		t.Fatalf("rendered content = %q", rendered)
	}
	if _, err := os.Stat(filepath.Join(codexPluginSkillsDir(repoRoot), "alpha", "data", "secret.txt")); !os.IsNotExist(err) {
		t.Fatalf("gitignored file was rendered, stat err = %v", err)
	}
	if _, err := os.Stat(staleDest); !os.IsNotExist(err) {
		t.Fatalf("stale rendered file still exists, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(staleDest)); !os.IsNotExist(err) {
		t.Fatalf("empty stale dir still exists, stat err = %v", err)
	}
}

func TestCheckCodexPluginDetectsDrift(t *testing.T) {
	repoRoot := initCodexPluginTestRepo(t)
	writeCodexPluginTestFile(t, filepath.Join(repoRoot, "skills", "alpha", "SKILL.md"), "---\nname: alpha\n---\n")
	writeCodexPluginTestFile(t, filepath.Join(repoRoot, filepath.FromSlash(codexPluginRelDir), ".codex-plugin", "plugin.json"), "{}\n")
	writeCodexPluginTestFile(t, filepath.Join(repoRoot, filepath.FromSlash(codexMarketplaceRelPath)), "{}\n")

	result := checkCodexPlugin(repoRoot)
	if result.status != checkStatusFail || !strings.Contains(result.detail, "dotagents render") {
		t.Fatalf("pre-render check = %#v, want stale fail", result)
	}

	if err := renderCodexPluginSkills(repoRoot); err != nil {
		t.Fatal(err)
	}
	result = checkCodexPlugin(repoRoot)
	if result.status != checkStatusPass {
		t.Fatalf("post-render check = %#v, want pass", result)
	}

	writeCodexPluginTestFile(t, filepath.Join(repoRoot, "skills", "alpha", "SKILL.md"), "---\nname: alpha\ndescription: changed\n---\n")
	result = checkCodexPlugin(repoRoot)
	if result.status != checkStatusFail {
		t.Fatalf("post-edit check = %#v, want fail", result)
	}
}
