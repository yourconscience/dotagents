package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDashboardFlags(t *testing.T) {
	opts, err := parseDashboardFlags([]string{"--no-open", "--addr", ":0", "--access", "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.NoOpen {
		t.Fatal("NoOpen=false, want true")
	}
	if opts.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr=%q, want 127.0.0.1:0", opts.Addr)
	}
	if opts.Access != "ssh" {
		t.Fatalf("Access=%q, want ssh", opts.Access)
	}
}

func TestParseDashboardFlagsRejectsNonLoopback(t *testing.T) {
	if _, err := parseDashboardFlags([]string{"--addr", "0.0.0.0:8765"}); err == nil {
		t.Fatal("expected non-loopback addr to be rejected")
	}
}

func TestDashboardHandlerServesStaticShellAndHealth(t *testing.T) {
	handler := newDashboardHandler("/repo", dashboardOptions{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status=%d, want 200", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "dotagents dashboard") || !strings.Contains(got, "/api/overview") {
		t.Fatalf("static shell missing expected content: %s", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health status=%d, want 200", rec.Code)
	}
	var health dashboardHealth
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if !health.OK || health.Repo != "/repo" || health.SessionsEnabled {
		t.Fatalf("health=%+v", health)
	}
}

func TestDashboardHealthReportsSessionsFlag(t *testing.T) {
	handler := newDashboardHandler("/repo", dashboardOptions{EnableSessions: true})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var health dashboardHealth
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if !health.SessionsEnabled {
		t.Fatalf("SessionsEnabled=false, want true")
	}
}

func TestDashboardForwardTargetUsesListenerHost(t *testing.T) {
	host, port := dashboardForwardTarget(&net.TCPAddr{IP: net.ParseIP("::1"), Port: 12345})
	if host != "[::1]" || port != "12345" {
		t.Fatalf("host=%q port=%q, want [::1] 12345", host, port)
	}

	host, port = dashboardForwardTarget(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 23456})
	if host != "127.0.0.1" || port != "23456" {
		t.Fatalf("host=%q port=%q, want 127.0.0.1 23456", host, port)
	}
}

func TestDashboardDataAPIs(t *testing.T) {
	repoRoot := writeDashboardFixture(t)
	t.Setenv("HOME", filepath.Join(repoRoot, "home"))

	handler := newDashboardHandler(repoRoot, dashboardOptions{})

	var skills []dashboardSkill
	getDashboardJSON(t, handler, "/api/skills", &skills)
	if len(skills) != 2 {
		t.Fatalf("skills len=%d, want 2: %+v", len(skills), skills)
	}
	if skills[0].Path == "" || skills[0].Command == "" {
		t.Fatalf("skill missing path/command: %+v", skills[0])
	}

	var agents []dashboardAgent
	getDashboardJSON(t, handler, "/api/agents", &agents)
	if len(agents) != 1 || agents[0].Name != "builder" || agents[0].Model != "sonnet" {
		t.Fatalf("agents=%+v", agents)
	}

	var mcps []dashboardMCP
	getDashboardJSON(t, handler, "/api/mcps", &mcps)
	if len(mcps) != 1 || mcps[0].Name != "local" || !stringInSlice("TOKEN", mcps[0].Env) {
		t.Fatalf("mcps=%+v", mcps)
	}
	if strings.Contains(strings.Join(mcps[0].Env, ","), "secret") {
		t.Fatalf("MCP env leaked value: %+v", mcps[0])
	}

	var hooks []dashboardHook
	getDashboardJSON(t, handler, "/api/hooks", &hooks)
	if len(hooks) != 1 || hooks[0].Name != "sync.sh" {
		t.Fatalf("hooks=%+v", hooks)
	}

	var examples dashboardExamplesResponse
	getDashboardJSON(t, handler, "/api/examples", &examples)
	if len(examples.Examples) != 1 || examples.Examples[0].Name != "sync" {
		t.Fatalf("examples=%+v", examples)
	}
	if len(examples.Warnings) != 1 {
		t.Fatalf("warnings=%+v, want one malformed example warning", examples.Warnings)
	}

	var overview dashboardOverview
	getDashboardJSON(t, handler, "/api/overview", &overview)
	if overview.Repo != repoRoot || overview.MCPCount != 1 || overview.HookCount != 1 || len(overview.Agents) != 1 {
		t.Fatalf("overview=%+v", overview)
	}
}

func TestDashboardExamplesMissingDirectoryIsEmpty(t *testing.T) {
	repoRoot := t.TempDir()
	examples, err := collectDashboardExamples(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(examples.Examples) != 0 || len(examples.Warnings) != 0 {
		t.Fatalf("examples=%+v", examples)
	}
}

func getDashboardJSON(t *testing.T, handler http.Handler, path string, dest any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), dest); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func writeDashboardFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "skills", "dotagents", "SKILL.md"), `---
name: dotagents
description: Inspect and sync dotagents.
---

# dotagents
`)
	mustWrite(t, filepath.Join(root, "skills", "dotagents", "dotagents.yaml"), `version: 1
agents:
  - name: codex
    enabled: true
    skill_root: ~/codex-skills
    agent_root: ~/codex-agents
mcp_servers:
  - name: local
    enabled: true
    command: uvx
    args: ["local-mcp"]
    env:
      TOKEN: secret
    agents: [codex]
`)
	mustWrite(t, filepath.Join(root, "skills", "foo", "SKILL.md"), `---
name: foo
description: Foo skill.
---

# Foo
`)
	mustWrite(t, filepath.Join(root, "agents", "builder.yaml"), `name: builder
description: Builds code.
model: sonnet
effort: high
tools: [Read, Edit]
instructions: |-
  Build the requested change.
`)
	mustWrite(t, filepath.Join(root, "memory", "hooks", "sync.sh"), "#!/bin/sh\n")
	mustWrite(t, filepath.Join(root, "experimental", "dotagents-ui", "examples", "sync.yaml"), `name: sync
title: Sync skills
description: Run dotagents sync.
command: dotagents sync
`)
	mustWrite(t, filepath.Join(root, "experimental", "dotagents-ui", "examples", "bad.yaml"), "name: [\n")
	return root
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
