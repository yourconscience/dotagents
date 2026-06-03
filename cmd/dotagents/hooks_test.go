package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestHookCommandMatchingNormalizesHomePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	raw := map[string]interface{}{
		"hooks": map[string]interface{}{
			"Stop": []interface{}{
				map[string]interface{}{
					"hooks": []interface{}{
						map[string]interface{}{
							"command": filepath.Join(home, ".agents", "memory", "hooks", "stop.sh"),
							"timeout": 3,
						},
					},
				},
			},
		},
	}

	if state := inspectClaudeHookMap(raw, testHook()); state != stateDrifted {
		t.Fatalf("inspectClaudeHookMap with absolute home path = %q, want drifted", state)
	}
	upsertClaudeHookMap(raw, testHook())

	groups := raw["hooks"].(map[string]interface{})["Stop"].([]interface{})
	items := groups[0].(map[string]interface{})["hooks"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("managed hook was duplicated instead of normalized: %#v", items)
	}
	item := items[0].(map[string]interface{})
	if item["command"] != "~/.agents/memory/hooks/stop.sh" || item["timeout"].(int) != 15 {
		t.Fatalf("managed hook was not updated in place: %#v", item)
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
			Name:    "cmux-amp-session-start",
			Enabled: true,
			Event:   "SessionStart",
			Command: "~/.agents/hooks/cmux/dispatch.sh amp session-start",
			Agents:  []string{agentAmp},
		}},
	}
	hooks, supported := desiredHooksForAgent(cfg, agentAmp)
	if supported {
		t.Fatal("amp hooks should be unsupported until a hook surface is verified")
	}
	if len(hooks) != 1 || hooks[0].Name != "cmux-amp-session-start" {
		t.Fatalf("desired hooks = %#v, want cmux-amp-session-start", hooks)
	}
}

func TestCodexHookPatchWritesNestedHooksAndEnablesFeature(t *testing.T) {
	home := t.TempDir()
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hooksPath, []byte(`{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {"type": "command", "command": "echo keep", "timeout": 1},
          {"command": "~/.agents/hooks/cmux/dispatch.sh codex session-start", "timeout": 1}
        ]
      }
    ]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.WriteFile(configPath, []byte("[profiles.default]\nmodel = \"gpt-5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hook := hookConfig{
		Name:    "cmux-codex-session-start",
		Enabled: true,
		Event:   "SessionStart",
		Command: "~/.agents/hooks/cmux/dispatch.sh codex session-start",
		Timeout: 5,
		Agents:  []string{agentCodex},
	}
	if state, err := inspectCodexHook(hook, home); err != nil || state != stateDrifted {
		t.Fatalf("inspect before patch = %q, %v; want drifted, nil", state, err)
	}
	if err := patchCodexHook(hook, home); err != nil {
		t.Fatal(err)
	}
	if state, err := inspectCodexHook(hook, home); err != nil || state != stateSynced {
		t.Fatalf("inspect after patch = %q, %v; want synced, nil", state, err)
	}

	out, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	items := raw["hooks"].(map[string]interface{})["SessionStart"].([]interface{})[0].(map[string]interface{})["hooks"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("nested hook count = %d, want 2: %#v", len(items), items)
	}
	managed := items[1].(map[string]interface{})
	if managed["type"] != "command" || managed["command"] != hook.Command || managed["timeout"].(float64) != 5 {
		t.Fatalf("managed hook was not rendered as nested command: %#v", managed)
	}

	configOut, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(configOut); got != "[features]\nhooks = true\n\n[profiles.default]\nmodel = \"gpt-5\"\n" {
		t.Fatalf("codex config not patched under [features]:\n%s", got)
	}
}

func TestCodexHookPatchEnablesExistingDisabledFeature(t *testing.T) {
	home := t.TempDir()
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.WriteFile(configPath, []byte("[features]\nhooks = false\nexperimental = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := hookConfig{
		Name:    "cmux-codex-session-start",
		Enabled: true,
		Event:   "SessionStart",
		Command: "~/.agents/hooks/cmux/dispatch.sh codex session-start",
		Timeout: 5,
		Agents:  []string{agentCodex},
	}
	if err := patchCodexHook(hook, home); err != nil {
		t.Fatal(err)
	}
	if state, err := inspectCodexHook(hook, home); err != nil || state != stateSynced {
		t.Fatalf("inspect after patch = %q, %v; want synced, nil", state, err)
	}
	configOut, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(configOut); got != "[features]\nhooks = true\nexperimental = true\n\n" {
		t.Fatalf("codex config not updated in existing [features]:\n%s", got)
	}
}

func TestCodexHookPatchEnablesCommentedFeaturesHeader(t *testing.T) {
	home := t.TempDir()
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.WriteFile(configPath, []byte("[features] # enabled feature flags\r\nexperimental = true\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := hookConfig{
		Name:    "cmux-codex-session-start",
		Enabled: true,
		Event:   "SessionStart",
		Command: "~/.agents/hooks/cmux/dispatch.sh codex session-start",
		Timeout: 5,
		Agents:  []string{agentCodex},
	}
	if err := patchCodexHook(hook, home); err != nil {
		t.Fatal(err)
	}
	configOut, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(configOut)
	if strings.Count(got, "[features]") != 1 {
		t.Fatalf("codex config has duplicate [features] header:\n%s", got)
	}
	if !strings.Contains(got, "[features] # enabled feature flags\r\nhooks = true\r\nexperimental = true") {
		t.Fatalf("codex config did not patch existing commented [features] header:\n%s", got)
	}
}

func TestDroidHookPatchWritesNestedHooks(t *testing.T) {
	home := t.TempDir()
	hook := hookConfig{
		Name:    "cmux-droid-stop",
		Enabled: true,
		Event:   "Stop",
		Command: "~/.agents/hooks/cmux/dispatch.sh factory stop",
		Timeout: 5000,
		Agents:  []string{agentDroid},
	}
	if state, err := inspectDroidHook(hook, home); err != nil || state != stateMissing {
		t.Fatalf("inspect before patch = %q, %v; want missing, nil", state, err)
	}
	if err := patchDroidHook(hook, home); err != nil {
		t.Fatal(err)
	}
	if state, err := inspectDroidHook(hook, home); err != nil || state != stateSynced {
		t.Fatalf("inspect after patch = %q, %v; want synced, nil", state, err)
	}
}

func TestHermesNativeCmuxHookEventsAreSupported(t *testing.T) {
	for _, event := range []string{
		"on_session_start",
		"pre_llm_call",
		"post_llm_call",
		"pre_approval_request",
		"post_approval_response",
		"on_session_end",
		"on_session_finalize",
		"on_session_reset",
		"pre_tool_call",
		"post_tool_call",
	} {
		if got, ok := hermesHookEvent(event); !ok || got != event {
			t.Fatalf("hermesHookEvent(%q) = %q, %v; want same event, true", event, got, ok)
		}
	}
}
