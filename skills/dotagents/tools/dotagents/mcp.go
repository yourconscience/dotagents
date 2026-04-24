package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type mcpServerConfig struct {
	Name    string            `yaml:"name"`
	Enabled bool              `yaml:"enabled"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
	Agents  []string          `yaml:"agents"`
}

func desiredMCPServersForAgent(cfg config, agentName string) []mcpServerConfig {
	var servers []mcpServerConfig
	for _, server := range cfg.MCPServers {
		if !server.Enabled {
			continue
		}
		if len(server.Agents) == 0 || stringInSlice(agentName, server.Agents) {
			servers = append(servers, server)
		}
	}
	return servers
}

func augmentMCPReport(report *agentReport, agent agentConfig, cfg config, home string) error {
	servers := desiredMCPServersForAgent(cfg, agent.Name)
	for _, server := range servers {
		state, err := inspectMCPServer(agent.Name, server, home)
		if err != nil {
			return err
		}
		switch state {
		case stateSynced:
			report.ManagedMCP = append(report.ManagedMCP, server.Name)
		case stateMissing:
			report.MissingMCP = append(report.MissingMCP, server.Name)
			report.AddsMCP = append(report.AddsMCP, server.Name)
		case stateDrifted:
			report.DriftedMCP = append(report.DriftedMCP, server.Name)
			report.UpdatesMCP = append(report.UpdatesMCP, server.Name)
		default:
			return fmt.Errorf("unsupported MCP inspect state %q for %s/%s", state, agent.Name, server.Name)
		}
	}
	return nil
}

func applyAgentMCPSync(reports []agentReport, cfg config, home string) error {
	for _, report := range reports {
		if !report.Detected {
			continue
		}
		servers := desiredMCPServersForAgent(cfg, report.Name)
		if len(servers) == 0 {
			continue
		}
		byName := make(map[string]mcpServerConfig, len(servers))
		for _, server := range servers {
			byName[server.Name] = server
		}
		for _, name := range append(append([]string{}, report.AddsMCP...), report.UpdatesMCP...) {
			server, ok := byName[name]
			if !ok {
				return fmt.Errorf("missing MCP config for %s/%s", report.Name, name)
			}
			if err := patchMCPServer(report.Name, server, home); err != nil {
				return err
			}
		}
	}
	return nil
}

func inspectMCPServer(agentName string, server mcpServerConfig, home string) (string, error) {
	switch agentName {
	case "claude-code":
		return inspectClaudeMCPServer(server, home)
	case "codex":
		return inspectCodexMCPServer(server, home)
	case agentHermes:
		return inspectHermesMCPServer(server, home)
	default:
		return stateMissing, nil
	}
}

func patchMCPServer(agentName string, server mcpServerConfig, home string) error {
	switch agentName {
	case "claude-code":
		return patchClaudeMCPServer(server, home)
	case "codex":
		return patchCodexMCPServer(server, home)
	case agentHermes:
		return patchHermesMCPServer(server, home)
	default:
		return nil
	}
}

func inspectClaudeMCPServer(server mcpServerConfig, home string) (string, error) {
	configPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return stateMissing, fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return stateMissing, fmt.Errorf("parse %s: %w", configPath, err)
	}

	serversRaw, ok := raw["mcpServers"]
	if !ok {
		return stateMissing, nil
	}
	serversMap, ok := asMap(serversRaw)
	if !ok {
		return stateDrifted, nil
	}
	entryRaw, ok := serversMap[server.Name]
	if !ok {
		return stateMissing, nil
	}
	entry, ok := asMap(entryRaw)
	if !ok {
		return stateDrifted, nil
	}
	if matchManagedMCPMap(entry, server) {
		return stateSynced, nil
	}
	return stateDrifted, nil
}

func patchClaudeMCPServer(server mcpServerConfig, home string) error {
	configPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}

	serversMap, _ := asMap(raw["mcpServers"])
	if serversMap == nil {
		serversMap = map[string]interface{}{}
		raw["mcpServers"] = serversMap
	}
	entry, _ := asMap(serversMap[server.Name])
	if entry == nil {
		entry = map[string]interface{}{}
	}
	applyManagedMCPMap(entry, server)
	serversMap[server.Name] = entry

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", configPath, err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

func inspectHermesMCPServer(server mcpServerConfig, home string) (string, error) {
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return stateMissing, fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return stateMissing, fmt.Errorf("parse %s: %w", configPath, err)
	}

	serversRaw, ok := raw["mcp_servers"]
	if !ok {
		return stateMissing, nil
	}
	serversMap, ok := asMap(serversRaw)
	if !ok {
		return stateDrifted, nil
	}
	entryRaw, ok := serversMap[server.Name]
	if !ok {
		return stateMissing, nil
	}
	entry, ok := asMap(entryRaw)
	if !ok {
		return stateDrifted, nil
	}
	if matchManagedMCPMap(entry, server) {
		return stateSynced, nil
	}
	return stateDrifted, nil
}

func patchHermesMCPServer(server mcpServerConfig, home string) error {
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}

	serversMap, _ := asMap(raw["mcp_servers"])
	if serversMap == nil {
		serversMap = map[string]interface{}{}
		raw["mcp_servers"] = serversMap
	}
	entry, _ := asMap(serversMap[server.Name])
	if entry == nil {
		entry = map[string]interface{}{}
	}
	applyManagedMCPMap(entry, server)
	serversMap[server.Name] = entry

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", configPath, err)
	}
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

func inspectCodexMCPServer(server mcpServerConfig, home string) (string, error) {
	configPath := filepath.Join(home, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return stateMissing, fmt.Errorf("read %s: %w", configPath, err)
	}

	block, ok := extractTOMLSection(string(data), fmt.Sprintf("[mcp_servers.%s]", server.Name))
	if !ok {
		return stateMissing, nil
	}
	if tomlBlockMatchesManagedMCP(block, server) {
		return stateSynced, nil
	}
	return stateDrifted, nil
}

func patchCodexMCPServer(server mcpServerConfig, home string) error {
	configPath := filepath.Join(home, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	content := string(data)
	header := fmt.Sprintf("[mcp_servers.%s]", server.Name)
	section := renderCodexMCPSection(server)
	updated := upsertTOMLSection(content, header, section)
	if updated == content {
		return nil
	}
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

func renderCodexMCPSection(server mcpServerConfig) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("[mcp_servers.%s]", server.Name))
	lines = append(lines, fmt.Sprintf("command = %q", server.Command))
	lines = append(lines, fmt.Sprintf("args = %s", renderTOMLStringArray(server.Args)))
	if len(server.Env) > 0 {
		lines = append(lines, fmt.Sprintf("env = %s", renderTOMLEnvInline(server.Env)))
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func tomlBlockMatchesManagedMCP(block string, server mcpServerConfig) bool {
	lines := strings.Split(block, "\n")
	values := make(map[string]string)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		values[key] = value
	}
	if values["command"] != fmt.Sprintf("%q", server.Command) {
		return false
	}
	if values["args"] != renderTOMLStringArray(server.Args) {
		return false
	}
	if len(server.Env) > 0 && values["env"] != renderTOMLEnvInline(server.Env) {
		return false
	}
	return true
}

func extractTOMLSection(content string, header string) (string, bool) {
	needle := header + "\n"
	start := strings.Index(content, needle)
	if start == -1 {
		if strings.HasSuffix(content, header) {
			return header + "\n", true
		}
		return "", false
	}
	searchStart := start + len(needle)
	endRel := indexNextTOMLHeader(content[searchStart:])
	if endRel == -1 {
		return content[start:], true
	}
	return content[start : searchStart+endRel], true
}

func upsertTOMLSection(content string, header string, section string) string {
	needle := header + "\n"
	start := strings.Index(content, needle)
	if start == -1 {
		insertAt := findTOMLInsertPoint(content)
		if insertAt == -1 {
			if strings.TrimSpace(content) == "" {
				return section
			}
			if strings.HasSuffix(content, "\n\n") {
				return content + section
			}
			if strings.HasSuffix(content, "\n") {
				return content + "\n" + section
			}
			return content + "\n\n" + section
		}
		prefix := content[:insertAt]
		suffix := content[insertAt:]
		if !strings.HasSuffix(prefix, "\n\n") {
			if strings.HasSuffix(prefix, "\n") {
				prefix += "\n"
			} else {
				prefix += "\n\n"
			}
		}
		return prefix + section + suffix
	}
	searchStart := start + len(needle)
	endRel := indexNextTOMLHeader(content[searchStart:])
	if endRel == -1 {
		return content[:start] + ensureTrailingBlankLine(section)
	}
	end := searchStart + endRel
	return content[:start] + ensureTrailingBlankLine(section) + content[end:]
}

func findTOMLInsertPoint(content string) int {
	candidates := []string{"\n[profiles.", "\n[projects.", "\n[tui]", "\n[analytics]", "\n[notice]", "\n[[skills.config]]", "\n[env]", "\n[agents]", "\n[marketplaces.", "\n[plugins."}
	best := -1
	for _, candidate := range candidates {
		idx := strings.Index(content, candidate)
		if idx != -1 && (best == -1 || idx < best) {
			best = idx + 1
		}
	}
	return best
}

func indexNextTOMLHeader(content string) int {
	for i := 0; i < len(content); i++ {
		if content[i] != '[' {
			continue
		}
		if i == 0 || content[i-1] == '\n' {
			return i
		}
	}
	return -1
}

func ensureTrailingBlankLine(section string) string {
	section = strings.TrimRight(section, "\n") + "\n"
	if !strings.HasSuffix(section, "\n\n") {
		section += "\n"
	}
	return section
}

func renderTOMLStringArray(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, fmt.Sprintf("%q", item))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func renderTOMLEnvInline(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s = %q", key, env[key]))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func matchManagedMCPMap(entry map[string]interface{}, server mcpServerConfig) bool {
	command, _ := entry["command"].(string)
	if command != server.Command {
		return false
	}
	args, ok := toStringSlice(entry["args"])
	if !ok || !stringSlicesEqual(args, server.Args) {
		return false
	}
	if len(server.Env) == 0 {
		return true
	}
	envMap, ok := asMap(entry["env"])
	if !ok {
		return false
	}
	for key, expected := range server.Env {
		actual, ok := envMap[key].(string)
		if !ok || actual != expected {
			return false
		}
	}
	return true
}

func applyManagedMCPMap(entry map[string]interface{}, server mcpServerConfig) {
	entry["command"] = server.Command
	args := make([]interface{}, 0, len(server.Args))
	for _, arg := range server.Args {
		args = append(args, arg)
	}
	entry["args"] = args
	if len(server.Env) > 0 {
		envMap, _ := asMap(entry["env"])
		if envMap == nil {
			envMap = map[string]interface{}{}
		}
		for key, value := range server.Env {
			envMap[key] = value
		}
		entry["env"] = envMap
	}
}

func asMap(v interface{}) (map[string]interface{}, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		return t, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for key, value := range t {
			keyStr, ok := key.(string)
			if !ok {
				return nil, false
			}
			out[keyStr] = value
		}
		return out, true
	default:
		return nil, false
	}
}

func toStringSlice(v interface{}) ([]string, bool) {
	switch t := v.(type) {
	case []string:
		return append([]string{}, t...), true
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			str, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, str)
		}
		return out, true
	default:
		return nil, false
	}
}

func stringSlicesEqual(a []string, b []string) bool {
	return bytes.Equal([]byte(strings.Join(a, "\x00")), []byte(strings.Join(b, "\x00")))
}

func stringInSlice(needle string, haystack []string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
