package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadContext(opts runOptions) (string, string, config, []agentConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", config{}, nil, fmt.Errorf("resolve home: %w", err)
	}

	configPath, err := resolveConfigPath(opts.ConfigPath, home)
	if err != nil {
		return "", "", config{}, nil, err
	}
	repoRoot := filepath.Dir(configPath)

	cfg, err := loadConfig(repoRoot, home, configPath)
	if err != nil {
		return "", "", config{}, nil, err
	}

	injectPluginMCPServers(&cfg, home)

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
	if err := applyLocalOverlay(&cfg, configPath); err != nil {
		return config{}, err
	}
	if err := validateConfig(&cfg, home, true); err != nil {
		return config{}, err
	}

	return cfg, nil
}

// applyLocalOverlay merges a gitignored dotagents.local.yaml (next to the main
// config) into cfg. Entries match by name (agents, mcp_servers, hooks) or repo
// name (external_skills): a match replaces the base entry wholesale, anything
// else is appended. This keeps personal additions out of public git.
func applyLocalOverlay(cfg *config, configPath string) error {
	localPath := filepath.Join(filepath.Dir(configPath), "dotagents.local.yaml")
	data, err := os.ReadFile(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read local config %s: %w", localPath, err)
	}
	var local config
	if err := yaml.Unmarshal(data, &local); err != nil {
		return fmt.Errorf("yaml decode %s: %w", localPath, err)
	}
	mergeConfig(cfg, local)
	return nil
}

func mergeConfig(base *config, overlay config) {
	base.Agents = mergeByKey(base.Agents, overlay.Agents, func(a agentConfig) string { return normalizeAgentName(a.Name) })
	base.ExternalSkills = mergeByKey(base.ExternalSkills, overlay.ExternalSkills, func(s externalSkillSource) string { return repoName(s.URL) })
	base.MCPServers = mergeByKey(base.MCPServers, overlay.MCPServers, func(s mcpServerConfig) string { return strings.TrimSpace(s.Name) })
	base.Hooks = mergeByKey(base.Hooks, overlay.Hooks, func(h hookConfig) string { return strings.TrimSpace(h.Name) })
	if overlay.ContextNoteTokens != nil {
		base.ContextNoteTokens = overlay.ContextNoteTokens
	}
}

func mergeByKey[T any](base []T, overlay []T, key func(T) string) []T {
	if len(overlay) == 0 {
		return base
	}
	index := make(map[string]int, len(base))
	for i, item := range base {
		index[key(item)] = i
	}
	for _, item := range overlay {
		if i, ok := index[key(item)]; ok {
			base[i] = item
		} else {
			base = append(base, item)
		}
	}
	return base
}

func defaultConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, "dotagents.yaml")
}

func validateConfig(cfg *config, home string, expand bool) error {
	if cfg.Version == 0 {
		cfg.Version = 1
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
		src := &cfg.ExternalSkills[i]
		src.URL = strings.TrimSpace(src.URL)
		if src.URL == "" {
			return errors.New("config external_skills entry has empty url")
		}
		src.Branch = strings.TrimSpace(src.Branch)
		if src.Branch == "" {
			src.Branch = "main"
		}
		src.SkillDir = strings.TrimSpace(src.SkillDir)
		if src.SkillDir != "" && len(src.SkillDirs) > 0 {
			return fmt.Errorf("config external_skills repo %q cannot set both skill_dir and skill_dirs", repoName(src.URL))
		}
		if src.SkillDir == "" && len(src.SkillDirs) == 0 {
			src.SkillDir = "skills"
		}
		seenDirs := make(map[string]struct{})
		if src.SkillDir != "" {
			clean, err := cleanExternalSkillDir(src.SkillDir)
			if err != nil {
				return fmt.Errorf("config external_skills repo %q: %w", repoName(src.URL), err)
			}
			src.SkillDir = clean
		}
		for j := range src.SkillDirs {
			clean, err := cleanExternalSkillDir(src.SkillDirs[j])
			if err != nil {
				return fmt.Errorf("config external_skills repo %q: %w", repoName(src.URL), err)
			}
			if _, exists := seenDirs[clean]; exists {
				return fmt.Errorf("config external_skills repo %q has duplicate skill_dirs path %q", repoName(src.URL), clean)
			}
			seenDirs[clean] = struct{}{}
			src.SkillDirs[j] = clean
		}
		name := repoName(src.URL)
		if name == "" || name == "." || name == ".." {
			return fmt.Errorf("config external_skills: cannot derive safe repo name from %q", src.URL)
		}
		if _, ok := seenExt[name]; ok {
			return fmt.Errorf("config external_skills repo %q is duplicated", name)
		}
		seenExt[name] = struct{}{}

		if src.MCP && len(src.MCPAgents) > 0 {
			kept := src.MCPAgents[:0]
			for _, agentName := range src.MCPAgents {
				agentName = normalizeAgentName(agentName)
				if agentName == "" {
					continue
				}
				if len(seen) > 0 {
					if _, ok := seen[agentName]; !ok {
						return fmt.Errorf("config external_skills repo %q mcp_agents targets unknown agent %q", name, agentName)
					}
				}
				if !hasMCPSupport(agentName) {
					fmt.Fprintf(os.Stderr, "warning: config external_skills repo %q mcp_agents targets agent %q without MCP support; target ignored\n", name, agentName)
					continue
				}
				kept = append(kept, agentName)
			}
			src.MCPAgents = kept
		}
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
		kept := cfg.MCPServers[i].Agents[:0]
		for j := range cfg.MCPServers[i].Agents {
			agentName := normalizeAgentName(cfg.MCPServers[i].Agents[j])
			if len(seen) > 0 {
				if _, ok := seen[agentName]; !ok {
					return fmt.Errorf("config MCP server %s targets unknown agent %q", cfg.MCPServers[i].Name, agentName)
				}
			}
			// Legacy configs predate the Pi/OMP split and target "pi" with MCP.
			// Dropping the target with a warning keeps setup/status/doctor usable
			// on old configs instead of hard-failing on first contact.
			if !hasMCPSupport(agentName) {
				hint := ""
				if agentName == agentPi {
					hint = ` (if this config predates the Pi/OMP split, rename "pi" to "omp" in dotagents.yaml)`
				}
				fmt.Fprintf(os.Stderr, "warning: config MCP server %s targets agent %q without MCP support; target ignored%s\n", cfg.MCPServers[i].Name, agentName, hint)
				continue
			}
			kept = append(kept, agentName)
		}
		cfg.MCPServers[i].Agents = kept
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
		if cfg.Hooks[i].Timeout < 0 {
			return fmt.Errorf("config hook %s has negative timeout", cfg.Hooks[i].Name)
		}
		if _, ok := seenHooks[cfg.Hooks[i].Name]; ok {
			return fmt.Errorf("config hook %s is duplicated", cfg.Hooks[i].Name)
		}
		seenHooks[cfg.Hooks[i].Name] = struct{}{}
		for j := range cfg.Hooks[i].Agents {
			cfg.Hooks[i].Agents[j] = normalizeAgentName(cfg.Hooks[i].Agents[j])
			agentName := cfg.Hooks[i].Agents[j]
			if len(seen) > 0 {
				if _, ok := seen[agentName]; !ok {
					return fmt.Errorf("config hook %s targets unknown agent %q", cfg.Hooks[i].Name, agentName)
				}
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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home: %w", err)
	}
	configPath, err := resolveConfigPath("", home)
	if err != nil {
		return "", "", err
	}
	root := filepath.Dir(configPath)
	return root, filepath.Join(root, "skills", "dotagents"), nil
}

func resolveConfigPath(overridePath string, home string) (string, error) {
	path := strings.TrimSpace(overridePath)
	if path != "" {
		return absoluteExpandedPath(path, home)
	}
	if env := strings.TrimSpace(os.Getenv("DOTAGENTS_HOME")); env != "" {
		root, err := absoluteExpandedPath(env, home)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, "dotagents.yaml"), nil
	}
	root := filepath.Join(home, ".agents")
	return filepath.Join(root, "dotagents.yaml"), nil
}

func absoluteExpandedPath(path string, home string) (string, error) {
	expanded := expandPath(path, home)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	return abs, nil
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
