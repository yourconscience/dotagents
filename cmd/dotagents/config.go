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

	repoRoot, _, err := findRoots()
	if err != nil {
		return "", "", config{}, nil, err
	}

	cfg, err := loadConfig(repoRoot, home, opts.ConfigPath)
	if err != nil {
		return "", "", config{}, nil, err
	}

	selected, err := selectAgents(cfg, opts.Agents)
	if err != nil {
		return "", "", config{}, nil, err
	}

	return repoRoot, home, cfg, selected, nil
}

func loadConfig(repoRoot string, home string, overridePath string) (config, error) {
	configPath := overridePath
	if strings.TrimSpace(configPath) == "" {
		configPath = defaultConfigPath(repoRoot)
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
	if err := validateConfig(&cfg, home, true); err != nil {
		return config{}, err
	}

	return cfg, nil
}

func defaultConfigPath(repoRoot string) string {
	path := filepath.Join(repoRoot, "dotagents.yaml")
	if hasFile(path) {
		return path
	}
	legacyPath := filepath.Join(repoRoot, "skills", "dotagents", "dotagents.yaml")
	if hasFile(legacyPath) {
		return legacyPath
	}
	return path
}

func validateConfig(cfg *config, home string, expand bool) error {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if len(cfg.Agents) == 0 {
		return errors.New("config has no agents")
	}

	seen := make(map[string]struct{})
	for i := range cfg.Agents {
		cfg.Agents[i].Name = normalizeAgentName(cfg.Agents[i].Name)
		if expand {
			cfg.Agents[i].SkillRoot = expandPath(cfg.Agents[i].SkillRoot, home)
			cfg.Agents[i].AgentRoot = expandPath(cfg.Agents[i].AgentRoot, home)
		}
		if cfg.Agents[i].Name == "" {
			return errors.New("config agent name cannot be empty")
		}
		if cfg.Agents[i].SkillRoot == "" {
			return fmt.Errorf("config agent %s is missing skill_root", cfg.Agents[i].Name)
		}
		if _, ok := seen[cfg.Agents[i].Name]; ok {
			return fmt.Errorf("config agent %s is duplicated", cfg.Agents[i].Name)
		}
		seen[cfg.Agents[i].Name] = struct{}{}
	}

	seenExt := make(map[string]struct{})
	for i := range cfg.ExternalSkills {
		cfg.ExternalSkills[i].URL = strings.TrimSpace(cfg.ExternalSkills[i].URL)
		if cfg.ExternalSkills[i].URL == "" {
			return errors.New("config external_skills entry has empty url")
		}
		if cfg.ExternalSkills[i].Branch == "" {
			cfg.ExternalSkills[i].Branch = "main"
		}
		if cfg.ExternalSkills[i].SkillDir == "" {
			cfg.ExternalSkills[i].SkillDir = "skills"
		}
		name := repoName(cfg.ExternalSkills[i].URL)
		if name == "" {
			return fmt.Errorf("config external_skills: cannot derive repo name from %q", cfg.ExternalSkills[i].URL)
		}
		if _, ok := seenExt[name]; ok {
			return fmt.Errorf("config external_skills repo %q is duplicated", name)
		}
		seenExt[name] = struct{}{}
	}

	seenMCP := make(map[string]struct{})
	for i := range cfg.MCPServers {
		cfg.MCPServers[i].Name = strings.TrimSpace(cfg.MCPServers[i].Name)
		if cfg.MCPServers[i].Name == "" {
			return errors.New("config MCP server name cannot be empty")
		}
		if strings.TrimSpace(cfg.MCPServers[i].Command) == "" {
			return fmt.Errorf("config MCP server %s is missing command", cfg.MCPServers[i].Name)
		}
		if _, ok := seenMCP[cfg.MCPServers[i].Name]; ok {
			return fmt.Errorf("config MCP server %s is duplicated", cfg.MCPServers[i].Name)
		}
		seenMCP[cfg.MCPServers[i].Name] = struct{}{}
		for j := range cfg.MCPServers[i].Agents {
			cfg.MCPServers[i].Agents[j] = normalizeAgentName(cfg.MCPServers[i].Agents[j])
			agentName := cfg.MCPServers[i].Agents[j]
			if _, ok := seen[agentName]; !ok {
				return fmt.Errorf("config MCP server %s targets unknown agent %q", cfg.MCPServers[i].Name, agentName)
			}
			if _, ok := mcpTargets[agentName]; !ok {
				return fmt.Errorf("config MCP server %s targets unsupported agent %q", cfg.MCPServers[i].Name, agentName)
			}
		}
	}

	seenHooks := make(map[string]struct{})
	for i := range cfg.Hooks {
		cfg.Hooks[i].Name = strings.TrimSpace(cfg.Hooks[i].Name)
		cfg.Hooks[i].Event = strings.TrimSpace(cfg.Hooks[i].Event)
		cfg.Hooks[i].Command = strings.TrimSpace(cfg.Hooks[i].Command)
		if cfg.Hooks[i].Name == "" {
			return errors.New("config hook name cannot be empty")
		}
		if cfg.Hooks[i].Event == "" {
			return fmt.Errorf("config hook %s is missing event", cfg.Hooks[i].Name)
		}
		if cfg.Hooks[i].Command == "" {
			return fmt.Errorf("config hook %s is missing command", cfg.Hooks[i].Name)
		}
		if _, ok := seenHooks[cfg.Hooks[i].Name]; ok {
			return fmt.Errorf("config hook %s is duplicated", cfg.Hooks[i].Name)
		}
		seenHooks[cfg.Hooks[i].Name] = struct{}{}
		for j := range cfg.Hooks[i].Agents {
			cfg.Hooks[i].Agents[j] = normalizeAgentName(cfg.Hooks[i].Agents[j])
			agentName := cfg.Hooks[i].Agents[j]
			if _, ok := seen[agentName]; !ok {
				return fmt.Errorf("config hook %s targets unknown agent %q", cfg.Hooks[i].Name, agentName)
			}
		}
	}

	return nil
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
	repoRoot := filepath.Clean(filepath.Join(toolDir, "..", ".."))
	skillRoot := filepath.Join(repoRoot, "skills", "dotagents")

	if !hasDir(filepath.Join(repoRoot, "skills")) || !hasFile(filepath.Join(repoRoot, "AGENTS.md")) {
		return "", "", fmt.Errorf("repo root not found from %s", toolDir)
	}
	if !hasFile(filepath.Join(skillRoot, "SKILL.md")) {
		return "", "", fmt.Errorf("skill root not found from %s", repoRoot)
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
