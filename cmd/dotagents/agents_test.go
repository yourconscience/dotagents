package main

import (
	"errors"
	"io/fs"
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

func TestLoadAgentRoles(t *testing.T) {
	repoRoot := t.TempDir()
	agentsDir := filepath.Join(repoRoot, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`name: reviewer
description: Reviews changes
model: sonnet
effort: high
tools: [Read, Grep]
color: purple
instructions: |-
  Review the change.
`)
	if err := os.WriteFile(filepath.Join(agentsDir, "reviewer.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	roles, err := loadAgentRoles(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 {
		t.Fatalf("loaded %d roles, want 1", len(roles))
	}
	role := roles[0]
	if role.Name != "reviewer" || role.Description != "Reviews changes" || role.Instructions != "Review the change." {
		t.Fatalf("unexpected role: %#v", role)
	}
	if len(role.Tools) != 2 || role.Tools[0] != "Read" || role.Tools[1] != "Grep" {
		t.Fatalf("unexpected tools: %#v", role.Tools)
	}
}

func TestRenderPluginAgentsIdempotent(t *testing.T) {
	repoRoot := t.TempDir()
	agentsDir := filepath.Join(repoRoot, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`name: reviewer
description: Reviews changes
model: sonnet
instructions: |-
  Review the change.
`)
	if err := os.WriteFile(filepath.Join(agentsDir, "reviewer.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := renderPluginAgents(repoRoot); err != nil {
		t.Fatal(err)
	}

	rendered := filepath.Join(repoRoot, "agents", "reviewer.md")
	first, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatalf("rendered file missing: %v", err)
	}
	if !strings.Contains(string(first), generatedAgentMarker) {
		t.Fatalf("rendered file missing generated marker:\n%s", first)
	}

	stale := filepath.Join(repoRoot, "agents", "removed-role.md")
	if err := os.WriteFile(stale, []byte("# "+generatedAgentMarker+"\nold"), 0o644); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(repoRoot, "agents", "README.md")
	if err := os.WriteFile(manual, []byte("hand-written notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := renderPluginAgents(repoRoot); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("second render changed output")
	}
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stale file not removed: %v", err)
	}
	if _, err := os.Stat(manual); err != nil {
		t.Fatalf("manual file was removed: %v", err)
	}

	roles, err := loadAgentRoles(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 {
		t.Fatalf("rendered agents/*.md leaked into loadAgentRoles: %d roles", len(roles))
	}
}

func TestCommittedPluginAgentsFresh(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := expectedPluginAgentFiles(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) == 0 {
		t.Fatal("no agent roles found in repo")
	}
	for path, content := range expected {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s missing; run: dotagents render", path)
			continue
		}
		if string(data) != content {
			t.Errorf("%s stale; run: dotagents render", path)
		}
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
