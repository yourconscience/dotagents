package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var commitMsgHookTrailerCases = map[string]string{
	agentAmp:        "Co-authored-by: amp[bot] <amp[bot]@users.noreply.github.com>",
	agentClaudeCode: "Co-authored-by: claude[bot] <claude[bot]@users.noreply.github.com>",
	agentCodex:      "Co-Authored-By: codex[bot] <codex[bot]@users.noreply.github.com>",
	agentDroid:      "Co-authored-by: factory-droid[bot] <factory-droid[bot]@users.noreply.github.com>",
	agentHermes:     "Co-Authored-By: hermes[bot] <hermes[bot]@users.noreply.github.com>",
	"openclaw":      "Co-authored-by: openclaw[bot] <openclaw[bot]@users.noreply.github.com>",
}

func TestCommitMsgHookStripsSupportedAgentTrailers(t *testing.T) {
	repoRoot, _, err := findRoots()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(repoRoot, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	var trailers []string
	for _, agent := range cfg.Agents {
		trailer, ok := commitMsgHookTrailerCases[agent.Name]
		if !ok {
			t.Fatalf("missing commit-msg hook trailer test case for configured agent %q", agent.Name)
		}
		trailers = append(trailers, trailer)
	}

	messagePath := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	original := "Subject\n\nBody before trailers\n" + strings.Join(trailers, "\n") + "\nBody after trailers\n"
	if err := os.WriteFile(messagePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	runHook(t, repoRoot, messagePath)

	data, err := os.ReadFile(messagePath)
	if err != nil {
		t.Fatal(err)
	}
	changed := string(data)
	for _, trailer := range trailers {
		if strings.Contains(changed, trailer) {
			t.Fatalf("hook did not strip %q:\n%s", trailer, changed)
		}
	}
	for _, want := range []string{"Subject", "Body before trailers", "Body after trailers"} {
		if !strings.Contains(changed, want) {
			t.Fatalf("hook removed non-trailer content %q:\n%s", want, changed)
		}
	}
	if _, err := os.Stat(messagePath + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("hook left backup file %s.bak", messagePath)
	}
}

func TestCommitMsgHookE2EStripsTrailersFromGitCommit(t *testing.T) {
	repoRoot, _, err := findRoots()
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	git(t, repo, "init")
	git(t, repo, "config", "user.name", "Test User")
	git(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "README.md")

	hookPath := filepath.Join(repo, ".git", "hooks", "commit-msg")
	hookData, err := os.ReadFile(filepath.Join(repoRoot, "hooks", "commit-msg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, hookData, 0o755); err != nil {
		t.Fatal(err)
	}
	messagePath := filepath.Join(repo, "message.txt")
	message := `Subject

Body
Co-authored-by: factory-droid[bot] <factory-droid[bot]@users.noreply.github.com>
Co-Authored-By: codex[bot] <codex[bot]@users.noreply.github.com>
`
	if err := os.WriteFile(messagePath, []byte(message), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "commit", "-F", messagePath)

	out := git(t, repo, "log", "-1", "--format=%B")
	if strings.Contains(strings.ToLower(out), "co-authored-by:") {
		t.Fatalf("commit message still contains co-author trailer:\n%s", out)
	}
	for _, want := range []string{"Subject", "Body"} {
		if !strings.Contains(out, want) {
			t.Fatalf("commit message missing %q:\n%s", want, out)
		}
	}
}

func runHook(t *testing.T, repoRoot string, messagePath string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(repoRoot, "hooks", "commit-msg"), messagePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("commit-msg hook failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	if stderr.Len() > 0 && args[0] != "init" {
		t.Logf("git %s stderr:\n%s", strings.Join(args, " "), stderr.String())
	}
	return strings.TrimSuffix(stdout.String(), "\n")
}

func TestCommitMsgHookTrailerCasesMatchConfiguredAgents(t *testing.T) {
	repoRoot, _, err := findRoots()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(repoRoot, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	configured := make(map[string]struct{}, len(cfg.Agents))
	for _, agent := range cfg.Agents {
		configured[agent.Name] = struct{}{}
	}
	for agent := range commitMsgHookTrailerCases {
		if _, ok := configured[agent]; !ok {
			t.Fatalf("commit-msg hook has trailer case for unconfigured agent %q", agent)
		}
	}
	for _, agent := range cfg.Agents {
		if _, ok := commitMsgHookTrailerCases[agent.Name]; !ok {
			t.Fatalf("commit-msg hook missing trailer case for configured agent %q", agent.Name)
		}
	}
	if len(commitMsgHookTrailerCases) != len(cfg.Agents) {
		t.Fatalf("commit-msg hook trailer case count = %d, configured agent count = %d", len(commitMsgHookTrailerCases), len(cfg.Agents))
	}
}
