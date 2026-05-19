package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func testHook() hookConfig {
	return hookConfig{
		Name:    "memory-stop",
		Enabled: true,
		Event:   "Stop",
		Command: "~/.agents/memory/hooks/stop.sh",
		Timeout: 15,
	}
}

func TestClaudeHookPatchPreservesUnrelatedHooks(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {"command": "echo keep", "timeout": 1},
          {"command": "~/.agents/memory/hooks/stop.sh", "timeout": 3}
        ]
      }
    ]
  }
}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if state, err := inspectClaudeHook(testHook(), home); err != nil || state != stateDrifted {
		t.Fatalf("inspect before patch = %q, %v; want drifted, nil", state, err)
	}
	if err := patchClaudeHook(testHook(), home); err != nil {
		t.Fatal(err)
	}
	if state, err := inspectClaudeHook(testHook(), home); err != nil || state != stateSynced {
		t.Fatalf("inspect after patch = %q, %v; want synced, nil", state, err)
	}

	out, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	hooksRoot := raw["hooks"].(map[string]interface{})
	groups := hooksRoot["Stop"].([]interface{})
	group := groups[0].(map[string]interface{})
	if group["matcher"] != "" {
		t.Fatalf("unrelated matcher was not preserved: %#v", group)
	}
	items := group["hooks"].([]interface{})
	foundKeep := false
	foundManaged := false
	for _, itemRaw := range items {
		item := itemRaw.(map[string]interface{})
		switch item["command"] {
		case "echo keep":
			foundKeep = true
		case "~/.agents/memory/hooks/stop.sh":
			foundManaged = item["timeout"].(float64) == 15
		}
	}
	if !foundKeep || !foundManaged {
		t.Fatalf("hooks were not preserved/patched: %#v", items)
	}
}

func TestClaudeHookPatchUpdatesExistingHookInLaterGroup(t *testing.T) {
	raw := map[string]interface{}{
		"hooks": map[string]interface{}{
			"Stop": []interface{}{
				map[string]interface{}{
					"matcher": "first",
					"hooks": []interface{}{
						map[string]interface{}{"command": "echo keep", "timeout": 1},
					},
				},
				map[string]interface{}{
					"matcher": "second",
					"hooks": []interface{}{
						map[string]interface{}{"command": "~/.agents/memory/hooks/stop.sh", "timeout": 3},
					},
				},
			},
		},
	}

	upsertClaudeHookMap(raw, testHook())

	groups := raw["hooks"].(map[string]interface{})["Stop"].([]interface{})
	managedCount := 0
	for i, groupRaw := range groups {
		group := groupRaw.(map[string]interface{})
		items := group["hooks"].([]interface{})
		for _, itemRaw := range items {
			item := itemRaw.(map[string]interface{})
			if item["command"] != "~/.agents/memory/hooks/stop.sh" {
				continue
			}
			managedCount++
			if i != 1 {
				t.Fatalf("managed hook moved to group %d, want group 1", i)
			}
			if item["timeout"].(int) != 15 {
				t.Fatalf("managed hook timeout = %#v, want 15", item["timeout"])
			}
		}
	}
	if managedCount != 1 {
		t.Fatalf("managed hook count = %d, want 1: %#v", managedCount, groups)
	}
}

func TestHermesHookPatchSessionEnd(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`hooks:
  on_session_finalize:
    - command: echo keep
      timeout: 1
`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	hook := hookConfig{
		Name:    "memory-session-end",
		Enabled: true,
		Event:   "SessionEnd",
		Command: "~/.agents/memory/hooks/session-end.sh",
		Timeout: 30,
	}
	if state, err := inspectHermesHook(hook, home); err != nil || state != stateMissing {
		t.Fatalf("inspect before patch = %q, %v; want missing, nil", state, err)
	}
	if err := patchHermesHook(hook, home); err != nil {
		t.Fatal(err)
	}
	if state, err := inspectHermesHook(hook, home); err != nil || state != stateSynced {
		t.Fatalf("inspect after patch = %q, %v; want synced, nil", state, err)
	}
	out, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	root := raw["hooks"].(map[string]interface{})
	items := root["on_session_finalize"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("unexpected hermes hooks: %#v", items)
	}
}

func TestHermesUnsupportedHookEventIsNotQueuedForSync(t *testing.T) {
	cfg := config{
		Hooks: []hookConfig{{
			Name:    "memory-stop",
			Enabled: true,
			Event:   "Stop",
			Command: "~/.agents/memory/hooks/stop.sh",
			Agents:  []string{agentHermes},
		}},
	}
	report := agentReport{Name: agentHermes, Detected: true}
	if err := augmentHookReport(&report, agentConfig{Name: agentHermes}, cfg, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if len(report.UnsupportedHook) != 1 || report.UnsupportedHook[0] != "memory-stop" {
		t.Fatalf("unsupported hooks = %#v, want memory-stop", report.UnsupportedHook)
	}
	if len(report.AddsHook) != 0 || len(report.MissingHook) != 0 {
		t.Fatalf("unsupported hook should not queue sync: adds=%#v missing=%#v", report.AddsHook, report.MissingHook)
	}
}

func TestDesiredHooksUnsupportedAgent(t *testing.T) {
	cfg := config{
		Hooks: []hookConfig{{
			Name:    "pr-triage-stop",
			Enabled: true,
			Event:   "Stop",
			Command: "~/.agents/skills/pr-triage/hooks/stop.sh",
			Agents:  []string{agentCodex},
		}},
	}
	hooks, supported := desiredHooksForAgent(cfg, agentCodex)
	if supported {
		t.Fatal("codex hooks should be unsupported until a hook surface is verified")
	}
	if len(hooks) != 1 || hooks[0].Name != "pr-triage-stop" {
		t.Fatalf("desired hooks = %#v, want pr-triage-stop", hooks)
	}
}
