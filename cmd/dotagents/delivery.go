package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	claudeDotagentsPluginID          = "dotagents@yourconscience"
	claudeDotagentsMarketplace       = "yourconscience"
	claudeDotagentsMarketplaceSource = "yourconscience/dotagents"
)

func inspectPluginDeliveryAgent(agent agentConfig, repoRoot string, agentsSkillRoot string, cfg config, home string) (agentReport, error) {
	report := agentReport{
		Name:      agent.Name,
		SkillRoot: agent.SkillRoot,
		AgentRoot: agent.AgentRoot,
		Delivery:  deliveryPlugin,
		Detected:  isDetected(agent),
	}
	if !report.Detected {
		return report, nil
	}

	// Repo skills arrive through the native Claude plugin, so their symlinks
	// are pruned; codex-plugin skills still need symlink delivery.
	expected, err := discoverPluginSkills(cfg.Plugins, home, agent.Name)
	if err != nil {
		return agentReport{}, err
	}
	report.ExpectedSkills = expected
	sourceRoots := pluginSourceRootsForAgent(cfg.Plugins, home, agent.Name)

	seen := make(map[string]bool)
	entries, err := os.ReadDir(agent.SkillRoot)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return agentReport{}, fmt.Errorf("read %s: %w", agent.SkillRoot, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		seen[name] = true
		path := filepath.Join(agent.SkillRoot, name)
		if entry.Type()&os.ModeSymlink == 0 {
			if _, ok := expected[name]; ok {
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s exists but is not a symlink", path))
				continue
			}
			report.External = append(report.External, name)
			continue
		}
		rawTarget, err := os.Readlink(path)
		if err != nil {
			return agentReport{}, fmt.Errorf("readlink %s: %w", path, err)
		}
		if isManagedSkillLink(path, rawTarget, repoRoot, agentsSkillRoot) {
			report.StaleManaged = append(report.StaleManaged, name)
			report.Removes = append(report.Removes, name)
			continue
		}
		if target, ok := expected[name]; ok {
			if linkMatches(path, rawTarget, target) {
				report.Managed = append(report.Managed, name)
			} else {
				report.Drifted = append(report.Drifted, name)
				report.Updates = append(report.Updates, name)
			}
			continue
		}
		if isPluginSkillLink(path, rawTarget, sourceRoots) {
			report.StaleManaged = append(report.StaleManaged, name)
			report.Removes = append(report.Removes, name)
			continue
		}
		report.External = append(report.External, name)
	}
	for _, name := range sortedKeys(expected) {
		if !seen[name] {
			report.Missing = append(report.Missing, name)
			report.Adds = append(report.Adds, name)
		}
	}

	agentFiles, err := claudeManagedAgentArtifacts(agent.AgentRoot)
	if err != nil {
		return agentReport{}, err
	}
	for _, name := range agentFiles {
		report.StaleManaged = append(report.StaleManaged, "agent:"+name)
	}
	report.RemovesAgent = append(report.RemovesAgent, agentFiles...)

	if err := augmentMCPReport(&report, agent, cfg, home); err != nil {
		return agentReport{}, err
	}
	if err := augmentHookReport(&report, agent, cfg, home); err != nil {
		return agentReport{}, err
	}

	sortReportLists(&report)
	report.Synced = isReportSynced(report)
	return report, nil
}

func claudeManagedSkillArtifacts(skillRoot string, repoRoot string, agentsSkillRoot string) ([]string, []string, error) {
	var managed []string
	var external []string
	entries, err := os.ReadDir(skillRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return managed, external, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", skillRoot, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(skillRoot, name)
		if entry.Type()&os.ModeSymlink != 0 {
			rawTarget, err := os.Readlink(path)
			if err != nil {
				return nil, nil, fmt.Errorf("readlink %s: %w", path, err)
			}
			if isManagedSkillLink(path, rawTarget, repoRoot, agentsSkillRoot) {
				managed = append(managed, name)
				continue
			}
		}
		external = append(external, name)
	}
	sort.Strings(managed)
	sort.Strings(external)
	return managed, external, nil
}

func claudeManagedAgentArtifacts(agentRoot string) ([]string, error) {
	var managed []string
	if strings.TrimSpace(agentRoot) == "" {
		return managed, nil
	}
	entries, err := os.ReadDir(agentRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return managed, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", agentRoot, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(agentRoot, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if strings.Contains(string(data), generatedAgentMarker) {
			managed = append(managed, entry.Name())
		}
	}
	sort.Strings(managed)
	return managed, nil
}

func pruneManagedSkillLinks(skillRoot string, names []string) error {
	for _, name := range names {
		path := filepath.Join(skillRoot, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

func pruneManagedAgentFiles(agentRoot string, names []string) error {
	for _, name := range names {
		path := filepath.Join(agentRoot, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

func checkClaudeDelivery(repoRoot string, home string, cfg config) checkResult {
	agent, ok := claudeAgentConfig(cfg)
	if !ok {
		return checkResult{"claude delivery", checkStatusPass, "claude-code not configured"}
	}

	installed, err := claudeDotagentsPluginInstalled(home)
	if err != nil {
		return checkResult{"claude delivery", checkStatusFail, err.Error()}
	}
	skills, _, err := claudeManagedSkillArtifacts(agent.SkillRoot, repoRoot, filepath.Join(home, ".agents", "skills"))
	if err != nil {
		return checkResult{"claude delivery", checkStatusFail, err.Error()}
	}
	agentFiles, err := claudeManagedAgentArtifacts(agent.AgentRoot)
	if err != nil {
		return checkResult{"claude delivery", checkStatusFail, err.Error()}
	}
	artifacts := append([]string{}, skills...)
	for _, name := range agentFiles {
		artifacts = append(artifacts, "agent:"+name)
	}
	sort.Strings(artifacts)

	switch normalizeDeliveryMode(agent.Delivery) {
	case deliveryPlugin:
		if !installed {
			return checkResult{"claude delivery", checkStatusFail, "delivery=plugin but dotagents@yourconscience is not installed; run: dotagents plugin add"}
		}
		if len(artifacts) > 0 {
			return checkResult{"claude delivery", checkStatusFail, fmt.Sprintf("delivery=plugin but managed sync artifacts remain: %s; run: dotagents sync --agents=claude-code", strings.Join(artifacts, ", "))}
		}
		return checkResult{"claude delivery", checkStatusPass, "delivery=plugin and dotagents@yourconscience installed"}
	default:
		if installed {
			return checkResult{"claude delivery", checkStatusWarn, "delivery=sync but dotagents@yourconscience is installed; run: dotagents plugin remove or set delivery: plugin"}
		}
		return checkResult{"claude delivery", checkStatusPass, "delivery=sync and Claude plugin absent"}
	}
}

func claudeAgentConfig(cfg config) (agentConfig, bool) {
	for _, agent := range cfg.Agents {
		if normalizeAgentName(agent.Name) == agentClaudeCode {
			return agent, true
		}
	}
	return agentConfig{}, false
}

func claudeDotagentsPluginInstalled(home string) (bool, error) {
	path := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	var raw struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	entry, ok := raw.Plugins[claudeDotagentsPluginID]
	if !ok {
		return false, nil
	}
	var installs []json.RawMessage
	if err := json.Unmarshal(entry, &installs); err != nil {
		return false, fmt.Errorf("parse %s plugin %s: %w", path, claudeDotagentsPluginID, err)
	}
	return len(installs) > 0, nil
}

func runPlugin(args []string) error {
	if len(args) == 0 {
		return errors.New("plugin requires subcommand: add, remove")
	}
	switch args[0] {
	case "add":
		return runPluginAdd(args[1:])
	case "remove":
		return runPluginRemove(args[1:])
	default:
		return fmt.Errorf("unknown plugin subcommand %q", args[0])
	}
}

func runPluginAdd(args []string) error {
	opts, err := parseSubcommandFlags("plugin add", args)
	if err != nil {
		return err
	}
	repoRoot, home, cfg, _, err := loadContext(opts)
	if err != nil {
		return err
	}
	if _, ok := claudeAgentConfig(cfg); !ok {
		return errors.New("claude-code is not configured in dotagents.yaml")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return errors.New("claude CLI not found on PATH; install Claude Code before enabling plugin delivery")
	}
	installed, err := claudeDotagentsPluginInstalled(home)
	if err != nil {
		return err
	}
	if !installed {
		if err := runClaudePluginCommand(true, "plugin", "marketplace", "add", claudeDotagentsMarketplaceSource); err != nil {
			return err
		}
		if err := runClaudePluginCommand(false, "plugin", "install", claudeDotagentsPluginID); err != nil {
			return err
		}
	}
	if err := setClaudeDeliveryMode(opts.ConfigPath, deliveryPlugin); err != nil {
		return err
	}
	cfg, err = loadConfig(repoRoot, home, opts.ConfigPath)
	if err != nil {
		return err
	}
	agent, _ := claudeAgentConfig(cfg)
	skills, _, err := claudeManagedSkillArtifacts(agent.SkillRoot, repoRoot, filepath.Join(home, ".agents", "skills"))
	if err != nil {
		return err
	}
	agents, err := claudeManagedAgentArtifacts(agent.AgentRoot)
	if err != nil {
		return err
	}
	if err := pruneManagedSkillLinks(agent.SkillRoot, skills); err != nil {
		return err
	}
	if err := pruneManagedAgentFiles(agent.AgentRoot, agents); err != nil {
		return err
	}
	fmt.Printf("Claude Code delivery set to plugin in %s\n", editableConfigPathForMessage(repoRoot, home, opts.ConfigPath))
	fmt.Println("Next: restart Claude Code if it is already running, then use /plugin list to confirm dotagents@yourconscience.")
	return nil
}

func runPluginRemove(args []string) error {
	opts, err := parseSubcommandFlags("plugin remove", args)
	if err != nil {
		return err
	}
	repoRoot, home, _, _, err := loadContext(opts)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return errors.New("claude CLI not found on PATH; cannot uninstall Claude Code plugin delivery")
	}
	installed, err := claudeDotagentsPluginInstalled(home)
	if err != nil {
		return err
	}
	if installed {
		if err := runClaudePluginCommand(true, "plugin", "uninstall", claudeDotagentsPluginID); err != nil {
			return err
		}
	}
	if err := runClaudePluginCommand(true, "plugin", "marketplace", "remove", claudeDotagentsMarketplace); err != nil {
		return err
	}
	if err := setClaudeDeliveryMode(opts.ConfigPath, deliverySync); err != nil {
		return err
	}
	if err := runSync(runOptions{ConfigPath: opts.ConfigPath, Agents: agentClaudeCode}); err != nil {
		return err
	}
	fmt.Printf("Claude Code delivery set to sync in %s\n", editableConfigPathForMessage(repoRoot, home, opts.ConfigPath))
	return nil
}

func runClaudePluginCommand(allowAlreadyOrMissing bool, args ...string) error {
	cmd := exec.Command("claude", args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if text != "" {
		fmt.Println(text)
	}
	if err == nil {
		return nil
	}
	if allowAlreadyOrMissing && isIdempotentClaudePluginError(text) {
		return nil
	}
	return fmt.Errorf("claude %s failed: %w", strings.Join(args, " "), err)
}

func isIdempotentClaudePluginError(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "already") || strings.Contains(output, "not found") || strings.Contains(output, "does not exist")
}

func setClaudeDeliveryMode(configPath string, delivery string) error {
	cfg, path, err := loadEditableMCPConfig(configPath)
	if err != nil {
		return err
	}
	found := false
	for _, agent := range cfg.Agents {
		if normalizeAgentName(agent.Name) == agentClaudeCode {
			found = true
			break
		}
	}
	if !found {
		return errors.New("claude-code is not configured in dotagents.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("yaml decode: %w", err)
	}
	if err := setAgentDeliveryNode(&doc, agentClaudeCode, delivery); err != nil {
		return err
	}
	out, err := marshalYAMLNode(&doc)
	if err != nil {
		return fmt.Errorf("yaml encode: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	_, _, err = loadEditableMCPConfig(path)
	return err
}

func setAgentDeliveryNode(doc *yaml.Node, agentName string, delivery string) error {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return errors.New("top-level YAML node is not a mapping")
	}
	root := doc.Content[0]
	var agents *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "agents" {
			agents = root.Content[i+1]
			break
		}
	}
	if agents == nil || agents.Kind != yaml.SequenceNode {
		return errors.New("config agents is not a sequence")
	}
	for _, item := range agents.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		var name string
		for i := 0; i+1 < len(item.Content); i += 2 {
			if item.Content[i].Value == "name" {
				name = normalizeAgentName(item.Content[i+1].Value)
				break
			}
		}
		if name != agentName {
			continue
		}
		setMappingString(item, "delivery", normalizeDeliveryMode(delivery))
		return nil
	}
	return fmt.Errorf("agent %s not found in config", agentName)
}

func editableConfigPathForMessage(repoRoot string, home string, overridePath string) string {
	if strings.TrimSpace(overridePath) != "" {
		return expandPath(overridePath, home)
	}
	return defaultConfigPath(repoRoot)
}
