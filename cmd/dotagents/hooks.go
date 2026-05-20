package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type hookConfig struct {
	Name    string   `yaml:"name"`
	Enabled bool     `yaml:"enabled"`
	Event   string   `yaml:"event"`
	Command string   `yaml:"command"`
	Timeout int      `yaml:"timeout,omitempty"`
	Agents  []string `yaml:"agents"`
}

type hookTarget struct {
	agentName string
	inspect   func(hookConfig, string) (string, error)
	patch     func(hookConfig, string) error
}

var hookTargets = map[string]hookTarget{
	agentClaudeCode: {
		agentName: agentClaudeCode,
		inspect:   inspectClaudeHook,
		patch:     patchClaudeHook,
	},
	agentHermes: {
		agentName: agentHermes,
		inspect:   inspectHermesHook,
		patch:     patchHermesHook,
	},
}

func desiredHooksForAgent(cfg config, agentName string) ([]hookConfig, bool) {
	agentName = normalizeAgentName(agentName)
	var hooks []hookConfig
	for _, hook := range cfg.Hooks {
		if !hook.Enabled {
			continue
		}
		if len(hook.Agents) == 0 || stringInSlice(agentName, hook.Agents) {
			hooks = append(hooks, hook)
		}
	}
	_, supported := hookTargets[agentName]
	return hooks, supported
}

func augmentHookReport(report *agentReport, agent agentConfig, cfg config, home string) error {
	hooks, supported := desiredHooksForAgent(cfg, agent.Name)
	if len(hooks) == 0 {
		return nil
	}
	if !supported {
		for _, hook := range hooks {
			report.UnsupportedHook = append(report.UnsupportedHook, hook.Name)
		}
		return nil
	}
	target := hookTargets[normalizeAgentName(agent.Name)]
	for _, hook := range hooks {
		state, err := target.inspect(hook, home)
		if err != nil {
			return err
		}
		switch state {
		case stateSynced:
			report.ManagedHook = append(report.ManagedHook, hook.Name)
		case stateMissing:
			report.MissingHook = append(report.MissingHook, hook.Name)
			report.AddsHook = append(report.AddsHook, hook.Name)
		case stateDrifted:
			report.DriftedHook = append(report.DriftedHook, hook.Name)
			report.UpdatesHook = append(report.UpdatesHook, hook.Name)
		case stateUnsupported:
			report.UnsupportedHook = append(report.UnsupportedHook, hook.Name)
		default:
			return fmt.Errorf("unsupported hook inspect state %q for %s/%s", state, agent.Name, hook.Name)
		}
	}
	return nil
}

func applyAgentHookSync(reports []agentReport, cfg config, home string) error {
	for _, report := range reports {
		if !report.Detected {
			continue
		}
		target, supported := hookTargets[normalizeAgentName(report.Name)]
		if !supported {
			continue
		}
		hooks, _ := desiredHooksForAgent(cfg, report.Name)
		byName := make(map[string]hookConfig, len(hooks))
		for _, hook := range hooks {
			byName[hook.Name] = hook
		}
		for _, name := range append(report.AddsHook, report.UpdatesHook...) {
			hook, ok := byName[name]
			if !ok {
				return fmt.Errorf("missing hook config for %s/%s", report.Name, name)
			}
			if err := target.patch(hook, home); err != nil {
				return err
			}
		}
	}
	return nil
}

func inspectClaudeHook(hook hookConfig, home string) (string, error) {
	configPath := claudeHooksConfigPath(home)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return stateMissing, nil
		}
		return stateMissing, fmt.Errorf("read %s: %w", configPath, err)
	}
	var raw map[string]interface{}
	if err := parseJSONConfig(configPath, data, &raw); err != nil {
		return stateMissing, fmt.Errorf("parse %s: %w", configPath, err)
	}
	return inspectClaudeHookMap(raw, hook), nil
}

func patchClaudeHook(hook hookConfig, home string) error {
	configPath := claudeHooksConfigPath(home)
	data, err := os.ReadFile(configPath)
	raw := map[string]interface{}{}
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", configPath, err)
		}
	} else if err := parseJSONConfig(configPath, data, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}
	upsertClaudeHookMap(raw, hook)
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", configPath, err)
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(configPath), err)
	}
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

func inspectHermesHook(hook hookConfig, home string) (string, error) {
	nativeEvent, ok := hermesHookEvent(hook.Event)
	if !ok {
		return stateUnsupported, nil
	}
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return stateMissing, nil
		}
		return stateMissing, fmt.Errorf("read %s: %w", configPath, err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return stateMissing, fmt.Errorf("parse %s: %w", configPath, err)
	}
	return inspectSimpleHookMap(raw, "hooks", nativeEvent, hook), nil
}

func patchHermesHook(hook hookConfig, home string) error {
	nativeEvent, ok := hermesHookEvent(hook.Event)
	if !ok {
		return fmt.Errorf("unsupported hermes hook event %q for %s", hook.Event, hook.Name)
	}
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	raw := map[string]interface{}{}
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", configPath, err)
		}
	} else if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}
	upsertSimpleHookMap(raw, "hooks", nativeEvent, hook)
	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", configPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(configPath), err)
	}
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

func claudeHooksConfigPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

func inspectClaudeHookMap(raw map[string]interface{}, hook hookConfig) string {
	hooksRoot, ok := raw["hooks"].(map[string]interface{})
	if !ok {
		return stateMissing
	}
	groups, ok := hooksRoot[hook.Event].([]interface{})
	if !ok {
		return stateMissing
	}
	found := false
	for _, groupRaw := range groups {
		group, ok := groupRaw.(map[string]interface{})
		if !ok {
			continue
		}
		items, ok := group["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, itemRaw := range items {
			item, ok := itemRaw.(map[string]interface{})
			if !ok || !hookCommandMatches(item["command"], hook.Command) {
				continue
			}
			found = true
			if hookTimeoutMatches(item, hook.Timeout) {
				return stateSynced
			}
		}
	}
	if found {
		return stateDrifted
	}
	return stateMissing
}

func upsertClaudeHookMap(raw map[string]interface{}, hook hookConfig) {
	hooksRoot, _ := raw["hooks"].(map[string]interface{})
	if hooksRoot == nil {
		hooksRoot = map[string]interface{}{}
		raw["hooks"] = hooksRoot
	}
	groups, _ := hooksRoot[hook.Event].([]interface{})
	if len(groups) == 0 {
		groups = []interface{}{map[string]interface{}{"hooks": []interface{}{}}}
	}
	targetIndex := 0
	for i, groupRaw := range groups {
		group, ok := groupRaw.(map[string]interface{})
		if !ok {
			continue
		}
		items, _ := group["hooks"].([]interface{})
		if containsHookCommand(items, hook.Command) {
			targetIndex = i
			break
		}
	}
	for i, groupRaw := range groups {
		group, _ := groupRaw.(map[string]interface{})
		if group == nil {
			if i != targetIndex {
				continue
			}
			group = map[string]interface{}{}
			groups[i] = group
		}
		items, _ := group["hooks"].([]interface{})
		items = removeHookCommand(items, hook.Command)
		if i == targetIndex {
			items = append(items, renderHookEntry(hook))
		}
		group["hooks"] = items
	}
	hooksRoot[hook.Event] = groups
}

func inspectSimpleHookMap(raw map[string]interface{}, rootKey string, event string, hook hookConfig) string {
	root, ok := raw[rootKey].(map[string]interface{})
	if !ok {
		return stateMissing
	}
	items, ok := root[event].([]interface{})
	if !ok {
		return stateMissing
	}
	found := false
	for _, itemRaw := range items {
		item, ok := itemRaw.(map[string]interface{})
		if !ok || !hookCommandMatches(item["command"], hook.Command) {
			continue
		}
		found = true
		if hookTimeoutMatches(item, hook.Timeout) {
			return stateSynced
		}
	}
	if found {
		return stateDrifted
	}
	return stateMissing
}

func upsertSimpleHookMap(raw map[string]interface{}, rootKey string, event string, hook hookConfig) {
	root, _ := raw[rootKey].(map[string]interface{})
	if root == nil {
		root = map[string]interface{}{}
		raw[rootKey] = root
	}
	items, _ := root[event].([]interface{})
	items = removeHookCommand(items, hook.Command)
	items = append(items, renderHookEntry(hook))
	root[event] = items
}

func renderHookEntry(hook hookConfig) map[string]interface{} {
	entry := map[string]interface{}{"command": hook.Command}
	if hook.Timeout > 0 {
		entry["timeout"] = hook.Timeout
	}
	return entry
}

func removeHookCommand(items []interface{}, command string) []interface{} {
	out := items[:0]
	for _, itemRaw := range items {
		item, ok := itemRaw.(map[string]interface{})
		if ok && hookCommandMatches(item["command"], command) {
			continue
		}
		out = append(out, itemRaw)
	}
	return out
}

func containsHookCommand(items []interface{}, command string) bool {
	for _, itemRaw := range items {
		item, ok := itemRaw.(map[string]interface{})
		if ok && hookCommandMatches(item["command"], command) {
			return true
		}
	}
	return false
}

func hookCommandMatches(actual interface{}, expected string) bool {
	actualString, ok := actual.(string)
	if !ok {
		return false
	}
	if actualString == expected {
		return true
	}
	return normalizeHookCommand(actualString) == normalizeHookCommand(expected)
}

func normalizeHookCommand(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			command = filepath.Join(home, strings.TrimPrefix(command, "~/"))
		}
	}
	return filepath.Clean(command)
}

func hookTimeoutMatches(item map[string]interface{}, timeout int) bool {
	if timeout == 0 {
		_, exists := item["timeout"]
		return !exists
	}
	switch value := item["timeout"].(type) {
	case int:
		return value == timeout
	case int64:
		return int(value) == timeout
	case float64:
		return int(value) == timeout
	default:
		return false
	}
}

func hermesHookEvent(event string) (string, bool) {
	switch event {
	case "SessionEnd":
		return "on_session_finalize", true
	default:
		return "", false
	}
}
