package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssessHerdrPluginHealthDiagnosesServerPATHMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "hook.js"), []byte("// hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := []string{"node", "src/hook.js"}
	plugins := []herdrPlugin{{
		PluginID:   "heeler",
		PluginRoot: root,
		Events:     []herdrPluginCommand{{Command: command}},
	}}
	logs := []herdrPluginLog{{
		PluginID:     "heeler",
		Command:      command,
		Status:       "failed",
		Error:        "No such file or directory (os error 2)",
		StartedUnixM: 10,
	}}

	result := assessHerdrPluginHealth(plugins, logs, func(name string) (string, error) {
		if name == "node" {
			return "/home/test/.local/bin/node", nil
		}
		return "", errors.New("not found")
	})
	if result.status != checkStatusWarn {
		t.Fatalf("status = %q, want warn (%s)", result.status, result.detail)
	}
	for _, want := range []string{"Herdr server PATH", "/home/test/.local/bin/node", "absolute executable"} {
		if !strings.Contains(result.detail, want) {
			t.Fatalf("detail %q does not contain %q", result.detail, want)
		}
	}
}

func TestAssessHerdrPluginHealthIgnoresRecoveredFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "hook.js"), []byte("// hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "node")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := []string{executable, "src/hook.js"}
	plugins := []herdrPlugin{{
		PluginID:   "heeler",
		PluginRoot: root,
		Events:     []herdrPluginCommand{{Command: command}},
	}}
	logs := []herdrPluginLog{
		{PluginID: "heeler", Command: command, Status: "failed", Error: "old failure", StartedUnixM: 10},
		{PluginID: "heeler", Command: command, Status: "succeeded", StartedUnixM: 20},
	}

	result := assessHerdrPluginHealth(plugins, logs, func(string) (string, error) { return "", nil })
	if result.status != checkStatusPass {
		t.Fatalf("status = %q, want pass (%s)", result.status, result.detail)
	}
}

func TestHerdrCommandAppliesUsesManifestPlatformNames(t *testing.T) {
	if !herdrCommandApplies([]string{"macos"}, "darwin") {
		t.Fatal("macos command should apply on darwin")
	}
	if herdrCommandApplies([]string{"windows"}, "darwin") {
		t.Fatal("windows command should not apply on darwin")
	}
}

func TestAssessHerdrPluginHealthIgnoresInlineShellProgram(t *testing.T) {
	root := t.TempDir()
	command := []string{"bash", "-lc", `exec "$HERDR_PLUGIN_ROOT/bin/plugin"`}
	plugins := []herdrPlugin{{
		PluginID:   "shell-plugin",
		PluginRoot: root,
		Actions:    []herdrPluginCommand{{Command: command}},
	}}

	result := assessHerdrPluginHealth(plugins, nil, func(string) (string, error) {
		return "/bin/bash", nil
	})
	if result.status != checkStatusPass {
		t.Fatalf("status = %q, want pass (%s)", result.status, result.detail)
	}
}

func TestAssessHerdrPluginHealthReportsMissingHookFile(t *testing.T) {
	root := t.TempDir()
	command := []string{"node", "src/missing-hook.js"}
	plugins := []herdrPlugin{{
		PluginID:   "broken-plugin",
		PluginRoot: root,
		Events:     []herdrPluginCommand{{Command: command}},
	}}

	result := assessHerdrPluginHealth(plugins, nil, func(string) (string, error) {
		return "/usr/bin/node", nil
	})
	if result.status != checkStatusWarn {
		t.Fatalf("status = %q, want warn (%s)", result.status, result.detail)
	}
	for _, want := range []string{"missing command file", "reinstall or update"} {
		if !strings.Contains(result.detail, want) {
			t.Fatalf("detail %q does not contain %q", result.detail, want)
		}
	}
}
