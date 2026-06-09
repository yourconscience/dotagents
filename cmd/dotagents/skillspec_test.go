package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkillSpecFixture(t *testing.T, repoRoot, dirName, content string) {
	t.Helper()
	dir := filepath.Join(repoRoot, "skills", dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSkillSpec(t *testing.T) {
	tests := []struct {
		name       string
		skills     map[string]string // dir name -> SKILL.md content
		wantStatus string
		wantDetail string
	}{
		{
			name: "valid skill passes",
			skills: map[string]string{
				"pdf-processing": "---\nname: pdf-processing\ndescription: Extract text from PDFs.\n---\n\n# PDF\n",
			},
			wantStatus: checkStatusPass,
			wantDetail: "1 skills conform to agentskills.io spec",
		},
		{
			name: "valid skill with optional fields passes",
			skills: map[string]string{
				"deploy": "---\nname: deploy\ndescription: Deploy the app.\nlicense: Apache-2.0\ncompatibility: Requires git\nallowed-tools: Bash Read\nmetadata:\n  author: kirill\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusPass,
			wantDetail: "1 skills conform to agentskills.io spec",
		},
		{
			name: "missing description warns",
			skills: map[string]string{
				"deploy": "---\nname: deploy\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "deploy: missing required field description",
		},
		{
			name: "missing name warns",
			skills: map[string]string{
				"deploy": "---\ndescription: Deploy the app.\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "deploy: missing required field name",
		},
		{
			name: "name not matching directory warns",
			skills: map[string]string{
				"deploy": "---\nname: release\ndescription: Deploy the app.\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: `deploy: name "release" does not match directory "deploy"`,
		},
		{
			name: "uppercase name warns",
			skills: map[string]string{
				"deploy": "---\nname: Deploy\ndescription: Deploy the app.\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "lowercase letters, digits, and hyphens",
		},
		{
			name: "consecutive hyphens warn",
			skills: map[string]string{
				"my--skill": "---\nname: my--skill\ndescription: Does things.\n---\n\n# Skill\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "no leading/trailing/consecutive hyphens",
		},
		{
			name: "name over 64 characters warns",
			skills: map[string]string{
				strings.Repeat("a", 65): "---\nname: " + strings.Repeat("a", 65) + "\ndescription: Long name.\n---\n\n# Skill\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "name exceeds 64 characters",
		},
		{
			name: "description over 1024 characters warns",
			skills: map[string]string{
				"deploy": "---\nname: deploy\ndescription: " + strings.Repeat("x", 1025) + "\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "description exceeds 1024 characters",
		},
		{
			name: "compatibility over 500 characters warns",
			skills: map[string]string{
				"deploy": "---\nname: deploy\ndescription: Deploy the app.\ncompatibility: " + strings.Repeat("x", 501) + "\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "compatibility exceeds 500 characters",
		},
		{
			name: "non-string name warns",
			skills: map[string]string{
				"deploy": "---\nname: 123\ndescription: Deploy the app.\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "deploy: name must be a string",
		},
		{
			name: "unknown fields pass with info detail",
			skills: map[string]string{
				"deploy": "---\nname: deploy\ndescription: Deploy the app.\nversion: 1.0.0\nauthor: kirill\n---\n\n# Deploy\n",
			},
			wantStatus: checkStatusPass,
			wantDetail: "info: deploy: unknown fields author, version",
		},
		{
			name: "missing frontmatter warns",
			skills: map[string]string{
				"deploy": "# Deploy\n\nNo frontmatter here.\n",
			},
			wantStatus: checkStatusWarn,
			wantDetail: "deploy: no frontmatter",
		},
		{
			name:       "empty skills dir passes",
			skills:     map[string]string{},
			wantStatus: checkStatusPass,
			wantDetail: "0 skills conform to agentskills.io spec",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repoRoot, "skills"), 0o755); err != nil {
				t.Fatal(err)
			}
			for dirName, content := range tc.skills {
				writeSkillSpecFixture(t, repoRoot, dirName, content)
			}

			result := checkSkillSpec(repoRoot)
			if result.status != tc.wantStatus {
				t.Fatalf("status = %q want %q (detail: %s)", result.status, tc.wantStatus, result.detail)
			}
			if !strings.Contains(result.detail, tc.wantDetail) {
				t.Fatalf("detail = %q does not contain %q", result.detail, tc.wantDetail)
			}
		})
	}
}

func TestCheckSkillSpecMissingSkillsDir(t *testing.T) {
	result := checkSkillSpec(t.TempDir())
	if result.status != checkStatusWarn {
		t.Fatalf("status = %q want %q", result.status, checkStatusWarn)
	}
	if !strings.Contains(result.detail, "cannot read skills/") {
		t.Fatalf("detail = %q", result.detail)
	}
}

func TestCheckSkillSpecMultipleViolations(t *testing.T) {
	repoRoot := t.TempDir()
	writeSkillSpecFixture(t, repoRoot, "good-skill", "---\nname: good-skill\ndescription: Fine.\n---\n")
	writeSkillSpecFixture(t, repoRoot, "bad-one", "---\nname: Bad_One\ndescription: Fine.\n---\n")
	writeSkillSpecFixture(t, repoRoot, "bad-two", "---\nname: bad-two\n---\n")

	result := checkSkillSpec(repoRoot)
	if result.status != checkStatusWarn {
		t.Fatalf("status = %q want %q (detail: %s)", result.status, checkStatusWarn, result.detail)
	}
	for _, want := range []string{"bad-one:", "bad-two: missing required field description"} {
		if !strings.Contains(result.detail, want) {
			t.Fatalf("detail = %q does not contain %q", result.detail, want)
		}
	}
	if strings.Contains(result.detail, "good-skill") {
		t.Fatalf("detail mentions conforming skill: %q", result.detail)
	}
}
