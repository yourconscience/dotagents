package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadContext(opts runOptions) (string, string, config, []agentConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", config{}, nil, fmt.Errorf("resolve home: %w", err)
	}

	repoRoot, skillRoot, err := findRoots()
	if err != nil {
		return "", "", config{}, nil, err
	}

	cfg, err := loadConfig(skillRoot, home, opts.ConfigPath)
	if err != nil {
		return "", "", config{}, nil, err
	}

	selected, err := selectAgents(cfg, opts.Agents)
	if err != nil {
		return "", "", config{}, nil, err
	}

	return repoRoot, home, cfg, selected, nil
}

func loadConfig(skillRoot string, home string, overridePath string) (config, error) {
	configPath := overridePath
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(skillRoot, "dotagents.yaml")
	}
	configPath = expandPath(configPath, home)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return config{}, fmt.Errorf("yaml decode: %w", err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if len(cfg.Agents) == 0 {
		return config{}, errors.New("config has no agents")
	}

	seen := make(map[string]struct{})
	for i := range cfg.Agents {
		cfg.Agents[i].Name = normalizeAgentName(cfg.Agents[i].Name)
		cfg.Agents[i].SkillRoot = expandPath(cfg.Agents[i].SkillRoot, home)
		cfg.Agents[i].AgentRoot = expandPath(cfg.Agents[i].AgentRoot, home)
		if cfg.Agents[i].Name == "" {
			return config{}, errors.New("config agent name cannot be empty")
		}
		if cfg.Agents[i].SkillRoot == "" {
			return config{}, fmt.Errorf("config agent %s is missing skill_root", cfg.Agents[i].Name)
		}
		if _, ok := seen[cfg.Agents[i].Name]; ok {
			return config{}, fmt.Errorf("config agent %s is duplicated", cfg.Agents[i].Name)
		}
		seen[cfg.Agents[i].Name] = struct{}{}
	}

	seenMCP := make(map[string]struct{})
	for i := range cfg.MCPServers {
		cfg.MCPServers[i].Name = strings.TrimSpace(cfg.MCPServers[i].Name)
		if cfg.MCPServers[i].Name == "" {
			return config{}, errors.New("config MCP server name cannot be empty")
		}
		if cfg.MCPServers[i].Command == "" {
			return config{}, fmt.Errorf("config MCP server %s is missing command", cfg.MCPServers[i].Name)
		}
		if _, ok := seenMCP[cfg.MCPServers[i].Name]; ok {
			return config{}, fmt.Errorf("config MCP server %s is duplicated", cfg.MCPServers[i].Name)
		}
		seenMCP[cfg.MCPServers[i].Name] = struct{}{}
		for j := range cfg.MCPServers[i].Agents {
			cfg.MCPServers[i].Agents[j] = normalizeAgentName(cfg.MCPServers[i].Agents[j])
		}
	}

	return cfg, nil
}

func selectAgents(cfg config, override string) ([]agentConfig, error) {
	index := make(map[string]agentConfig, len(cfg.Agents))
	for _, agent := range cfg.Agents {
		index[agent.Name] = agent
	}

	if strings.TrimSpace(override) == "" {
		var selected []agentConfig
		for _, agent := range cfg.Agents {
			if agent.Enabled {
				selected = append(selected, agent)
			}
		}
		if len(selected) == 0 {
			return nil, errors.New("config has no enabled agents; use --agents to override")
		}
		return selected, nil
	}

	var selected []agentConfig
	seen := make(map[string]struct{})
	for _, part := range strings.Split(override, ",") {
		name := normalizeAgentName(part)
		if name == "" {
			continue
		}
		agent, ok := index[name]
		if !ok {
			return nil, fmt.Errorf("unknown agent %q in --agents", name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		selected = append(selected, agent)
	}
	if len(selected) == 0 {
		return nil, errors.New("--agents did not resolve to any configured agents")
	}
	return selected, nil
}

func findRoots() (string, string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", errors.New("could not resolve tool source path")
	}

	toolDir := filepath.Dir(file)
	skillRoot := filepath.Clean(filepath.Join(toolDir, "..", ".."))
	repoRoot := filepath.Clean(filepath.Join(skillRoot, "..", ".."))

	if !hasFile(filepath.Join(skillRoot, "SKILL.md")) {
		return "", "", fmt.Errorf("skill root not found from %s", toolDir)
	}
	if !hasDir(filepath.Join(repoRoot, "skills")) || !hasFile(filepath.Join(repoRoot, "AGENTS.md")) {
		return "", "", fmt.Errorf("repo root not found from %s", toolDir)
	}

	return repoRoot, skillRoot, nil
}

func expandPath(path string, home string) string {
	path = strings.TrimSpace(path)
	switch {
	case path == "~":
		return home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	default:
		return path
	}
}

func normalizeAgentName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
