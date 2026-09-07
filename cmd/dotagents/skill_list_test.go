package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerRepo(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.invalid/solo", "example.invalid/solo"},
		{"https://github.com/mattpocock/skills/", "mattpocock/skills"},
		{"https://github.com/mattpocock/skills.git", "mattpocock/skills"},
		{"git@github.com:yourconscience/myagents.git", "yourconscience/myagents"},
	}
	for _, tc := range cases {
		if got := ownerRepo(tc.url); got != tc.want {
			t.Errorf("ownerRepo(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestSkillOriginsFromLockAndConfig(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()

	agentsDir := filepath.Join(repoRoot, "skills", "grilling")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"grilling":     agentsDir,
		"local-skill":  filepath.Join(repoRoot, "skills", "local-skill"),
		"direct-skill": filepath.Join(home, ".agents", "external", "vercel", "skills", "direct-skill"),
	}

	lock := lockFile{Version: 1, ExternalSkills: []externalLockEntry{{
		Name:         "skills",
		URL:          "https://github.com/mattpocock/skills",
		Branch:       "main",
		Commit:       "9603c1cc8118d08bc1b3bf34cf714f62178dea3b",
		Materialized: newMaterializedSkillNames([]string{"grilling"}),
	}}}
	if err := writeLockFile(repoRoot, lock); err != nil {
		t.Fatal(err)
	}

	cfg := config{
		ExternalSkills: []externalSkillSource{{
			URL:    "https://github.com/vercel-labs/agent-skills",
			Branch: "main",
			Skills: []string{"direct-skill"},
		}},
	}
	origins, err := skillOrigins(cfg, repoRoot, home, expected)
	if err != nil {
		t.Fatal(err)
	}
	if got := origins["grilling"]; got != "mattpocock/skills@9603c1c" {
		t.Fatalf("grilling origin = %q, want mattpocock/skills@9603c1c", got)
	}
	if got := origins["local-skill"]; got != "" {
		t.Fatalf("local-skill origin = %q, want empty", got)
	}
}

func TestSkillProvenanceClassification(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "harness-skills")
	canonical := filepath.Join(home, "canonical", "real-skill")
	other := filepath.Join(home, "elsewhere", "gone-skill")
	for _, dir := range []string{root, canonical, other} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	managedLink := filepath.Join(root, "managed")
	if err := os.Symlink(canonical, managedLink); err != nil {
		t.Fatal(err)
	}
	foreignLink := filepath.Join(root, "foreign")
	if err := os.Symlink(other, foreignLink); err != nil {
		t.Fatal(err)
	}
	brokenLink := filepath.Join(root, "broken")
	if err := os.Symlink(filepath.Join(home, "does-not-exist"), brokenLink); err != nil {
		t.Fatal(err)
	}
	unmanaged := filepath.Join(root, "unmanaged")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatal(err)
	}

	report := agentReport{
		Name:      "test-agent",
		SkillRoot: root,
		Managed:   []string{"managed"},
		External:  []string{"foreign", "broken", "unmanaged"},
		Missing:   []string{"absent"},
		Drifted:   []string{"drifted"},
	}
	driftedLink := filepath.Join(root, "drifted")
	if err := os.Symlink(other, driftedLink); err != nil {
		t.Fatal(err)
	}

	origins := map[string]string{"managed": "owner/repo@abc1234"}
	cases := map[string]string{
		"managed":   "managed (external: owner/repo@abc1234)",
		"foreign":   "foreign symlink -> " + other,
		"broken":    "broken symlink -> " + filepath.Join(home, "does-not-exist"),
		"unmanaged": "unmanaged dir",
		"absent":    "missing (not linked)",
		"drifted":   "drifted symlink -> " + other,
	}
	for name, want := range cases {
		if got := skillProvenance(name, report, origins, home); got != want {
			t.Errorf("skillProvenance(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSkillInfoRejectsMissingName(t *testing.T) {
	if err := runSkillInfo(nil); err == nil || err.Error() != "skill info requires a skill name" {
		t.Fatalf("runSkillInfo(nil) = %v, want name error", err)
	}
}
