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

const (
	deliverySync   = "sync"
	deliveryPlugin = "plugin"
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
	if err := applyLocalOverlay(&cfg, configPath); err != nil {
		return config{}, err
	}
	if err := validateConfig(&cfg, home, true); err != nil {
		return config{}, err
	}

	return cfg, nil
}

// applyLocalOverlay merges a gitignored dotagents.local.yaml (next to the main
// config) into cfg. Entries match by name (agents, mcp_servers, hooks, plugins)
// or repo name (external_skills): a match replaces the base entry wholesale,
// anything else is appended. This keeps personal additions out of public git.
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
	base.Plugins = mergeByKey(base.Plugins, overlay.Plugins, func(p pluginConfig) string { return strings.TrimSpace(p.Name) })
	base.Sources = mergeByKey(base.Sources, overlay.Sources, func(s sourceConfig) string { return strings.TrimSpace(s.Name) })
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
		cfg.Agents[i].Delivery = normalizeDeliveryMode(cfg.Agents[i].Delivery)
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
		if !isSupportedDeliveryMode(cfg.Agents[i].Delivery) {
			return fmt.Errorf("config agent %s has unsupported delivery %q", cfg.Agents[i].Name, cfg.Agents[i].Delivery)
		}
		if cfg.Agents[i].Delivery == deliveryPlugin && cfg.Agents[i].Name != agentClaudeCode {
			return fmt.Errorf("config agent %s cannot use delivery: plugin (only claude-code supports native plugin delivery)", cfg.Agents[i].Name)
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
			if !hasMCPSupport(agentName) {
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

	seenPlugins := make(map[string]struct{})
	for i := range cfg.Plugins {
		cfg.Plugins[i].Name = strings.TrimSpace(cfg.Plugins[i].Name)
		cfg.Plugins[i].Source = strings.TrimSpace(cfg.Plugins[i].Source)
		cfg.Plugins[i].Format = normalizePluginFormat(cfg.Plugins[i].Format)
		cfg.Plugins[i].Description = strings.TrimSpace(cfg.Plugins[i].Description)
		cfg.Plugins[i].Review = strings.TrimSpace(cfg.Plugins[i].Review)
		if cfg.Plugins[i].Name == "" {
			return errors.New("config plugin name cannot be empty")
		}
		if cfg.Plugins[i].Enabled && cfg.Plugins[i].Source == "" {
			return fmt.Errorf("config plugin %s is enabled but missing source", cfg.Plugins[i].Name)
		}
		if _, ok := seenPlugins[cfg.Plugins[i].Name]; ok {
			return fmt.Errorf("config plugin %s is duplicated", cfg.Plugins[i].Name)
		}
		seenPlugins[cfg.Plugins[i].Name] = struct{}{}
		if !isSupportedPluginFormat(cfg.Plugins[i].Format) {
			return fmt.Errorf("config plugin %s has unsupported format %q", cfg.Plugins[i].Name, cfg.Plugins[i].Format)
		}
		for j := range cfg.Plugins[i].Surfaces {
			cfg.Plugins[i].Surfaces[j] = normalizePluginSurface(cfg.Plugins[i].Surfaces[j])
			if !isSupportedPluginSurface(cfg.Plugins[i].Surfaces[j]) {
				return fmt.Errorf("config plugin %s has unsupported surface %q", cfg.Plugins[i].Name, cfg.Plugins[i].Surfaces[j])
			}
		}
		for j := range cfg.Plugins[i].Agents {
			cfg.Plugins[i].Agents[j] = normalizeAgentName(cfg.Plugins[i].Agents[j])
			agentName := cfg.Plugins[i].Agents[j]
			if _, ok := seen[agentName]; !ok {
				return fmt.Errorf("config plugin %s targets unknown agent %q", cfg.Plugins[i].Name, agentName)
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
	// 1. Explicit env override
	if env := os.Getenv("DOTAGENTS_ROOT"); env != "" {
		root := env
		if home, err := os.UserHomeDir(); err == nil {
			root = expandPath(root, home)
		}
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		if isValidRoot(root) {
			return root, filepath.Join(root, "skills", "dotagents"), nil
		}
		return "", "", fmt.Errorf("DOTAGENTS_ROOT=%s is not a valid dotagents root (needs skills/ dir with dotagents.yaml)", env)
	}

	// 2. ~/.agents symlink (the standard user config location)
	if home, err := os.UserHomeDir(); err == nil {
		agentsLink := filepath.Join(home, ".agents")
		if target, err := filepath.EvalSymlinks(agentsLink); err == nil {
			if isValidRoot(target) {
				return target, filepath.Join(target, "skills", "dotagents"), nil
			}
		}
		if isValidRoot(agentsLink) {
			return agentsLink, filepath.Join(agentsLink, "skills", "dotagents"), nil
		}
	}

	// 3. Walk CWD upward looking for dotagents.yaml
	if cwd, err := os.Getwd(); err == nil {
		if root := walkUpForRoot(cwd); root != "" {
			return root, filepath.Join(root, "skills", "dotagents"), nil
		}
	}

	// 4. Fall back to runtime.Caller (dev mode: binary built from source repo)
	if _, file, _, ok := runtime.Caller(0); ok {
		toolDir := filepath.Dir(file)
		repoRoot := filepath.Clean(filepath.Join(toolDir, "..", ".."))
		if isValidRoot(repoRoot) {
			return repoRoot, filepath.Join(repoRoot, "skills", "dotagents"), nil
		}
	}

	return "", "", errors.New("dotagents root not found; set DOTAGENTS_ROOT, create ~/.agents, or run from a directory containing dotagents.yaml")
}

// OR: a fresh user repo may have dotagents.yaml but no skills/ yet (created on first sync).
// CWD walk (walkUpForRoot) is stricter: requires dotagents.yaml explicitly.
func isValidRoot(path string) bool {
	return hasDir(filepath.Join(path, "skills")) || hasFile(filepath.Join(path, "dotagents.yaml"))
}

func walkUpForRoot(dir string) string {
	for {
		if hasFile(filepath.Join(dir, "dotagents.yaml")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
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

func normalizeDeliveryMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return deliverySync
	}
	return mode
}

func isSupportedDeliveryMode(mode string) bool {
	switch normalizeDeliveryMode(mode) {
	case deliverySync, deliveryPlugin:
		return true
	default:
		return false
	}
}

func usesPluginDelivery(agent agentConfig) bool {
	return normalizeAgentName(agent.Name) == agentClaudeCode && normalizeDeliveryMode(agent.Delivery) == deliveryPlugin
}
