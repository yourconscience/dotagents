package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderClaudeAgentRoleQuotesFrontmatter(t *testing.T) {
	role := agentRole{
		Name:         "reviewer",
		Description:  "Reviews code: safely",
		Model:        "sonnet",
		Effort:       "high",
		Tools:        []string{"Read", "Grep"},
		Color:        "purple",
		Instructions: "Review the change.",
	}

	got := renderClaudeAgentRole(role)
	for _, want := range []string{
		`name: "reviewer"`,
		`description: "Reviews code: safely"`,
		`model: "sonnet"`,
		`effort: "high"`,
		`color: "purple"`,
		generatedAgentMarker,
		"Review the change.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered Claude role missing %q:\n%s", want, got)
		}
	}
}

func TestRenderCodexAgentRoleEscapesControlCharacters(t *testing.T) {
	role := agentRole{
		Name:         "researcher",
		Description:  `Find "facts"`,
		Model:        "sonnet",
		Effort:       "high",
		Instructions: "Line one\nLine two\tTabbed\rReturn",
		Codex: codexRoleOptions{
			Model:                "gpt-5.4-mini",
			ModelReasoningEffort: "medium",
		},
	}

	got := renderCodexAgentRole(role)
	for _, want := range []string{
		`name = "researcher"`,
		`description = "Find \"facts\""`,
		`model = "gpt-5.4-mini"`,
		`model_reasoning_effort = "medium"`,
		`developer_instructions = "Line one\nLine two\tTabbed\rReturn"`,
		generatedAgentMarker,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered Codex role missing %q:\n%s", want, got)
		}
	}
}

func TestRenderOMPAgentRoleHonorsOverride(t *testing.T) {
	role := agentRole{
		Name:         "researcher",
		Description:  "Find reliable evidence",
		Model:        "opus",
		Effort:       "high",
		Instructions: "Compare the sources.",
		OMP: ompRoleOptions{
			Model:         "gpt-5.6-luna",
			ThinkingLevel: "xhigh",
		},
	}

	got := renderOMPAgentRole(role)
	for _, want := range []string{
		`name: "researcher"`,
		"model:\n",
		`- "gpt-5.6-luna"`,
		`thinking-level: "xhigh"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered OMP role missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `- "opus"`) || strings.Contains(got, "effort:") {
		t.Fatalf("OMP role leaked Claude defaults into override:\n%s", got)
	}
}

func TestRenderOMPAgentRoleFallsBackToCanonicalModel(t *testing.T) {
	role := agentRole{
		Name:         "reviewer",
		Description:  "Reviews code",
		Model:        "opus",
		Effort:       "high",
		Instructions: "Review carefully.",
	}

	got := renderOMPAgentRole(role)
	if !strings.Contains(got, "model:\n") || !strings.Contains(got, `- "opus"`) {
		t.Fatalf("OMP role did not fall back to canonical model:\n%s", got)
	}
	if strings.Contains(got, "thinking-level:") {
		t.Fatalf("OMP role invented a thinking-level without an override:\n%s", got)
	}
}

func TestRenderDroidAgentRoleMapsModelAndTools(t *testing.T) {
	role := agentRole{
		Name:         "builder",
		Description:  "Builds features",
		Model:        "sonnet",
		Effort:       "high",
		Tools:        []string{"Read", "Glob", "Grep", "Bash", "Write", "Edit", "WebFetch", "WebSearch"},
		Instructions: "Implement the change.",
	}

	got := renderDroidAgentRole(role)
	for _, want := range []string{
		`name: "builder"`,
		`description: "Builds features"`,
		`model: "custom:gpt-5.5(medium)"`,
		`reasoningEffort: "high"`,
		`- "Read"`,
		`- "Glob"`,
		`- "Grep"`,
		`- "Execute"`,
		`- "Create"`,
		`- "Edit"`,
		`- "FetchUrl"`,
		`- "WebSearch"`,
		generatedAgentMarker,
		"Implement the change.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered Droid role missing %q:\n%s", want, got)
		}
	}
}

func TestCodexModelFor(t *testing.T) {
	tests := map[string]string{
		"":           "gpt-5.4",
		"sonnet":     "gpt-5.4",
		"opus":       "gpt-5.4",
		"haiku":      "gpt-5.4-mini",
		"gpt-custom": "gpt-custom",
	}

	for input, want := range tests {
		if got := codexModelFor(input); got != want {
			t.Fatalf("codexModelFor(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDroidModelFor(t *testing.T) {
	tests := map[string]string{
		"":           "inherit",
		"sonnet":     "custom:gpt-5.5(medium)",
		"opus":       "custom:gpt-5.5(high)",
		"haiku":      "custom:gpt-5.5(low)",
		"gpt-custom": "gpt-custom",
	}

	for input, want := range tests {
		if got := droidModelFor(input); got != want {
			t.Fatalf("droidModelFor(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDroidToolsForMapsWriteToCreateAndEdit(t *testing.T) {
	got := droidToolsFor([]string{"Write"})
	want := []string{"Create", "Edit"}
	if len(got) != len(want) {
		t.Fatalf("droidToolsFor(Write) = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("droidToolsFor(Write) = %#v, want %#v", got, want)
		}
	}
}

func TestDroidToolsForFallsBackToReadOnlyWhenNoToolsMap(t *testing.T) {
	got := droidToolsFor([]string{"NotebookEdit"})
	want := []string{"Read", "LS", "Grep", "Glob"}
	if len(got) != len(want) {
		t.Fatalf("droidToolsFor(unmapped) = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("droidToolsFor(unmapped) = %#v, want %#v", got, want)
		}
	}
}

func writeAgentsFixture(t *testing.T, repoRoot string, name string, data string) {
	t.Helper()
	agentsDir := filepath.Join(repoRoot, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, name), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAgentRolesMarkdown(t *testing.T) {
	repoRoot := t.TempDir()
	writeAgentsFixture(t, repoRoot, "reviewer.md", `---
name: reviewer
description: Reviews changes
model: sonnet
effort: high
tools: [Read, Grep]
color: purple
omp:
  model: gpt-5.6-luna
  thinking-level: xhigh
---

Review the change.
`)
	writeAgentsFixture(t, repoRoot, "builder.md", `---
name: builder
description: Builds features
model: sonnet
tools: Read, Grep
---

Implement the change.
`)

	roles, err := loadAgentRoles(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("loaded %d roles, want 2", len(roles))
	}
	if roles[0].Name != "builder" || roles[1].Name != "reviewer" {
		t.Fatalf("roles not sorted by name: %#v", roles)
	}
	if len(roles[0].Tools) != 2 || roles[0].Tools[0] != "Read" || roles[0].Tools[1] != "Grep" {
		t.Fatalf("unexpected scalar tools: %#v", roles[0].Tools)
	}
	role := roles[1]
	if role.Description != "Reviews changes" || role.Instructions != "Review the change." {
		t.Fatalf("unexpected role: %#v", role)
	}
	if len(role.Tools) != 2 || role.Tools[0] != "Read" || role.Tools[1] != "Grep" {
		t.Fatalf("unexpected tools: %#v", role.Tools)
	}
	if role.OMP.Model != "gpt-5.6-luna" || role.OMP.ThinkingLevel != "xhigh" {
		t.Fatalf("unexpected OMP options: %#v", role.OMP)
	}
	if filepath.Base(role.Source) != "reviewer.md" {
		t.Fatalf("unexpected source: %q", role.Source)
	}
}

func TestCanonicalResearcherRendersCodexAtMax(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	role, err := loadMarkdownAgentRole(filepath.Join(repoRoot, "agents", "researcher.md"))
	if err != nil {
		t.Fatal(err)
	}

	got := renderCodexAgentRole(role)
	for _, want := range []string{
		`name = "researcher"`,
		`model = "gpt-5.6-luna"`,
		`model_reasoning_effort = "max"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("canonical researcher Codex role missing %q:\n%s", want, got)
		}
	}
}

func TestCanonicalResearcherRendersOMPAtMaximum(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	role, err := loadMarkdownAgentRole(filepath.Join(repoRoot, "agents", "researcher.md"))
	if err != nil {
		t.Fatal(err)
	}

	got := renderOMPAgentRole(role)
	for _, want := range []string{
		`name: "researcher"`,
		`- "gpt-5.6-luna"`,
		`thinking-level: "max"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("canonical researcher OMP role missing %q:\n%s", want, got)
		}
	}
}

func TestLoadAgentRolesMarkdownRejectsMappingTools(t *testing.T) {
	repoRoot := t.TempDir()
	writeAgentsFixture(t, repoRoot, "invalid.md", `---
name: invalid
description: Invalid tools
tools: {name: Read}
---

Do work.
`)
	_, err := loadAgentRoles(repoRoot)
	if err == nil || !strings.Contains(err.Error(), "tools must be a YAML sequence or comma-separated string") {
		t.Fatalf("mapping tools error = %v", err)
	}
}

func TestLoadAgentRolesMarkdownRejectsMissingFrontmatter(t *testing.T) {
	repoRoot := t.TempDir()
	writeAgentsFixture(t, repoRoot, "reviewer.md", "Review the change.\n")

	if _, err := loadAgentRoles(repoRoot); err == nil || !strings.Contains(err.Error(), "missing YAML frontmatter") {
		t.Fatalf("want missing frontmatter error, got %v", err)
	}
}

func TestLoadAgentRolesMarkdownRejectsMissingInstructions(t *testing.T) {
	repoRoot := t.TempDir()
	writeAgentsFixture(t, repoRoot, "reviewer.md", `---
name: reviewer
description: Reviews changes
---
`)

	if _, err := loadAgentRoles(repoRoot); err == nil || !strings.Contains(err.Error(), "missing instructions") {
		t.Fatalf("want missing instructions error, got %v", err)
	}
}

func TestIsManagedAgentFile(t *testing.T) {
	repoRoot := t.TempDir()
	managed := filepath.Join(repoRoot, "managed.toml")
	if err := os.WriteFile(managed, []byte("# "+generatedAgentMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isManagedAgentFile(managed, []byte("# "+generatedAgentMarker+"\n"), repoRoot) {
		t.Fatal("generated marker should be managed")
	}

	unmanaged := filepath.Join(repoRoot, "unmanaged.toml")
	if err := os.WriteFile(unmanaged, []byte("name = \"local\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isManagedAgentFile(unmanaged, []byte("name = \"local\"\n"), repoRoot) {
		t.Fatal("unmarked real file should not be managed")
	}
}
