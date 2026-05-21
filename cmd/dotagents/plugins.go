package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	pluginFormatDotagents = "dotagents"
	pluginFormatCodex     = "codex-plugin"
	pluginFormatClaude    = "claude-plugin"
	pluginFormatMCP       = "mcp-plugin"

	pluginSurfaceSkills = "skills"
	pluginSurfaceAgents = "agents"
	pluginSurfaceMCP    = "mcp"
	pluginSurfaceHooks  = "hooks"
	pluginSurfaceNative = "native-plugin"
	pluginSurfaceCmds   = "commands"
	pluginSurfaceAssets = "assets"
)

func normalizePluginFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return pluginFormatDotagents
	}
	return format
}

func isSupportedPluginFormat(format string) bool {
	switch normalizePluginFormat(format) {
	case pluginFormatDotagents, pluginFormatCodex, pluginFormatClaude, pluginFormatMCP:
		return true
	default:
		return false
	}
}

func normalizePluginSurface(surface string) string {
	surface = strings.ToLower(strings.TrimSpace(surface))
	switch surface {
	case "skill":
		return pluginSurfaceSkills
	case "agent", "subagents", "subagent":
		return pluginSurfaceAgents
	case "mcps", "mcp_servers", "mcp-server", "mcp-servers":
		return pluginSurfaceMCP
	case "hook":
		return pluginSurfaceHooks
	case "native", "plugin":
		return pluginSurfaceNative
	case "command":
		return pluginSurfaceCmds
	default:
		return surface
	}
}

func isSupportedPluginSurface(surface string) bool {
	switch normalizePluginSurface(surface) {
	case pluginSurfaceSkills, pluginSurfaceAgents, pluginSurfaceMCP, pluginSurfaceHooks, pluginSurfaceNative, pluginSurfaceCmds, pluginSurfaceAssets:
		return true
	default:
		return false
	}
}

func pluginTargetsAgent(plugin pluginConfig, agentName string) bool {
	if len(plugin.Agents) == 0 {
		return true
	}
	for _, name := range plugin.Agents {
		if normalizeAgentName(name) == agentName {
			return true
		}
	}
	return false
}

func pluginHasSurface(plugin pluginConfig, surface string) bool {
	surface = normalizePluginSurface(surface)
	for _, candidate := range plugin.Surfaces {
		if normalizePluginSurface(candidate) == surface {
			return true
		}
	}
	return false
}

func discoverPluginSkills(plugins []pluginConfig, home string, agentName string) (map[string]string, error) {
	result := make(map[string]string)
	for _, plugin := range plugins {
		if !plugin.Enabled || !pluginTargetsAgent(plugin, agentName) || !pluginHasSurface(plugin, pluginSurfaceSkills) {
			continue
		}
		skillBase, err := pluginSkillBase(plugin, home)
		if err != nil {
			return nil, err
		}
		skills, err := discoverSkillsInDir(plugin.Name, skillBase)
		if err != nil {
			return nil, err
		}
		for name, path := range skills {
			if _, exists := result[name]; exists {
				return nil, fmt.Errorf("plugin skill %q from %s collides with another plugin skill", name, plugin.Name)
			}
			result[name] = path
		}
	}
	return result, nil
}

func pluginSkillBasesForAgent(plugins []pluginConfig, home string, agentName string) ([]string, error) {
	var bases []string
	for _, plugin := range plugins {
		if !plugin.Enabled || !pluginTargetsAgent(plugin, agentName) || !pluginHasSurface(plugin, pluginSurfaceSkills) {
			continue
		}
		base, err := pluginSkillBase(plugin, home)
		if err != nil {
			return nil, err
		}
		bases = append(bases, base)
	}
	return bases, nil
}

func pluginSkillBase(plugin pluginConfig, home string) (string, error) {
	source, err := pluginSourcePath(plugin, home)
	if err != nil {
		return "", err
	}
	if hasDir(filepath.Join(source, "skills")) {
		return filepath.Join(source, "skills"), nil
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return "", fmt.Errorf("read plugin source %s: %w", source, err)
	}
	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(source, entry.Name())
		if hasDir(filepath.Join(path, "skills")) {
			candidates = append(candidates, path)
		}
	}
	if len(candidates) == 1 {
		return filepath.Join(candidates[0], "skills"), nil
	}
	if len(candidates) > 1 {
		sort.Strings(candidates)
		return filepath.Join(candidates[len(candidates)-1], "skills"), nil
	}
	return "", fmt.Errorf("plugin %s source %s has no skills directory", plugin.Name, source)
}

func pluginSourcePath(plugin pluginConfig, home string) (string, error) {
	source := strings.TrimSpace(plugin.Source)
	if source == "" {
		return "", fmt.Errorf("plugin %s is missing source", plugin.Name)
	}
	if strings.HasPrefix(source, "codex:") {
		return pluginSourceRefPath(plugin.Name, source, "codex:", "DOTAGENTS_CODEX_PLUGIN_ROOT")
	}
	if strings.HasPrefix(source, "claude:") {
		return pluginSourceRefPath(plugin.Name, source, "claude:", "DOTAGENTS_CLAUDE_PLUGIN_ROOT")
	}
	return expandPath(source, home), nil
}

func pluginSourceRefPath(pluginName string, source string, prefix string, envName string) (string, error) {
	ref := strings.Trim(strings.TrimPrefix(source, prefix), "/")
	if ref == "" {
		return "", fmt.Errorf("plugin %s source %q is missing a source id", pluginName, source)
	}
	root := strings.TrimSpace(os.Getenv(envName))
	if root == "" {
		return "", fmt.Errorf("plugin %s source %q requires %s", pluginName, source, envName)
	}
	return filepath.Join(root, filepath.FromSlash(ref)), nil
}

func discoverSkillsInDir(pluginName string, skillBase string) (map[string]string, error) {
	result := make(map[string]string)
	if hasFile(filepath.Join(skillBase, "SKILL.md")) {
		result[filepath.Base(skillBase)] = skillBase
		return result, nil
	}

	entries, err := os.ReadDir(skillBase)
	if err != nil {
		return nil, fmt.Errorf("read plugin %s skills %s: %w", pluginName, skillBase, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(skillBase, entry.Name())
		if !hasFile(filepath.Join(path, "SKILL.md")) {
			continue
		}
		result[entry.Name()] = path
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("plugin %s has no skills in %s", pluginName, skillBase)
	}
	return result, nil
}

func isPluginSkillLink(linkPath string, rawTarget string, skillBases []string) bool {
	targetAbs := absoluteTarget(linkPath, rawTarget)
	for _, base := range skillBases {
		if targetAbs == base || strings.HasPrefix(targetAbs, base+string(os.PathSeparator)) {
			return true
		}
		resolved, err := filepath.EvalSymlinks(targetAbs)
		if err == nil && (resolved == base || strings.HasPrefix(resolved, base+string(os.PathSeparator))) {
			return true
		}
	}
	return false
}

func pluginNativeSurface(format string) string {
	switch normalizePluginFormat(format) {
	case pluginFormatCodex:
		return pluginFormatCodex
	case pluginFormatClaude:
		return pluginFormatClaude
	default:
		return ""
	}
}

func agentPluginSurfaceSupport(agentName string) map[string]bool {
	support := map[string]bool{
		pluginSurfaceSkills: true,
		pluginSurfaceMCP:    true,
		pluginSurfaceAssets: true,
	}
	switch agentName {
	case agentClaudeCode:
		support[pluginSurfaceAgents] = true
		support[pluginSurfaceHooks] = true
		support[pluginSurfaceCmds] = true
		support[pluginFormatClaude] = true
	case agentCodex:
		support[pluginSurfaceAgents] = true
		support[pluginFormatCodex] = true
	case agentDroid:
		support[pluginSurfaceAgents] = true
	case agentHermes:
		support[pluginSurfaceHooks] = true
	case agentAmp:
		// Amp currently consumes shared skills and MCP entries through settings.
	}
	return support
}

func pluginCompatibility(plugin pluginConfig, agentName string) (string, string) {
	agentName = normalizeAgentName(agentName)
	if !pluginTargetsAgent(plugin, agentName) {
		return "not targeted", ""
	}

	var required []string
	native := pluginNativeSurface(plugin.Format)
	for _, surface := range plugin.Surfaces {
		surface = normalizePluginSurface(surface)
		if surface == pluginSurfaceNative && native != "" {
			required = append(required, native)
			continue
		}
		required = append(required, surface)
	}
	if len(required) == 0 {
		return "metadata", "no declared runtime surface"
	}

	support := agentPluginSurfaceSupport(agentName)
	var missing []string
	var matched int
	for _, surface := range required {
		surface = normalizePluginSurface(surface)
		if support[surface] {
			matched++
			continue
		}
		missing = append(missing, surface)
	}

	if len(missing) == 0 {
		return "supported", ""
	}
	if matched > 0 {
		return "partial", fmt.Sprintf("missing %s", strings.Join(missing, ", "))
	}
	return "unsupported", fmt.Sprintf("missing %s", strings.Join(missing, ", "))
}

func pluginCompatibilitySummary(plugin pluginConfig) string {
	agents := []string{agentClaudeCode, agentCodex, agentAmp, agentHermes, agentDroid}
	parts := make([]string, 0, len(agents))
	for _, agent := range agents {
		state, detail := pluginCompatibility(plugin, agent)
		if detail != "" {
			parts = append(parts, fmt.Sprintf("%s=%s (%s)", agent, state, detail))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", agent, state))
	}
	return strings.Join(parts, "; ")
}

func checkFirstPartyPlugins(cfg config) checkResult {
	if len(cfg.Plugins) == 0 {
		return checkResult{"first-party plugins", checkStatusPass, "no plugin catalog entries"}
	}

	enabled := 0
	examples := 0
	var unsupported []string
	for _, plugin := range cfg.Plugins {
		if plugin.Enabled {
			enabled++
		} else {
			examples++
			continue
		}
		for _, agent := range plugin.Agents {
			state, detail := pluginCompatibility(plugin, agent)
			if state == "unsupported" {
				if detail != "" {
					unsupported = append(unsupported, fmt.Sprintf("%s:%s (%s)", plugin.Name, agent, detail))
				} else {
					unsupported = append(unsupported, fmt.Sprintf("%s:%s", plugin.Name, agent))
				}
			}
		}
	}

	if len(unsupported) > 0 {
		return checkResult{"first-party plugins", checkStatusWarn, strings.Join(unsupported, "; ")}
	}
	return checkResult{"first-party plugins", checkStatusPass, fmt.Sprintf("%d enabled, %d review examples", enabled, examples)}
}
