package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	stateMissing     = "missing"
	stateConflict    = "conflict"
	stateSynced      = "synced"
	stateDrifted     = "drifted"
	stateUnsupported = "unsupported"
)

func expectedSkills(repoRoot string, home string, cfg config) (map[string]string, error) {
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			entries = nil
		} else {
			return nil, fmt.Errorf("read %s: %w", skillsDir, err)
		}
	}

	expected := make(map[string]string)
	agentsSkillRoot := filepath.Join(home, ".agents", "skills")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !hasFile(filepath.Join(skillsDir, name, "SKILL.md")) {
			continue
		}
		expected[name] = filepath.Join(agentsSkillRoot, name)
	}

	external, err := discoverExternalSkills(cfg.ExternalSkills, home)
	if err != nil {
		return nil, err
	}
	for name, path := range external {
		if _, exists := expected[name]; exists {
			return nil, fmt.Errorf("external skill %q collides with local skill", name)
		}
		expected[name] = path
	}

	return expected, nil
}

func expectedSkillsForAgent(base map[string]string, home string, cfg config, agentName string) (map[string]string, error) {
	expected := make(map[string]string, len(base))
	for name, path := range base {
		expected[name] = path
	}
	pluginSkills, err := discoverPluginSkills(cfg.Plugins, home, agentName)
	if err != nil {
		return nil, err
	}
	for name, path := range pluginSkills {
		if _, exists := expected[name]; exists {
			return nil, fmt.Errorf("plugin skill %q collides with existing skill", name)
		}
		expected[name] = path
	}
	return expected, nil
}

func inspectRepoLink(repoRoot string, home string) (repoLinkReport, error) {
	linkPath := filepath.Join(home, ".agents")
	report := repoLinkReport{
		Path:           linkPath,
		ExpectedTarget: repoRoot,
		State:          stateMissing,
	}

	info, err := os.Lstat(linkPath)
	if errors.Is(err, fs.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return repoLinkReport{}, fmt.Errorf("stat %s: %w", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		report.State = stateConflict
		report.ActualTarget = "non-symlink path exists"
		return report, nil
	}

	rawTarget, err := os.Readlink(linkPath)
	if err != nil {
		return repoLinkReport{}, fmt.Errorf("readlink %s: %w", linkPath, err)
	}
	report.ActualTarget = rawTarget
	if linkMatches(linkPath, rawTarget, repoRoot) {
		report.State = stateSynced
		return report, nil
	}

	report.State = stateDrifted
	return report, nil
}

func inspectAgents(selected []agentConfig, expected map[string]string, repoRoot string, home string, cfg config) ([]agentReport, error) {
	reports := make([]agentReport, 0, len(selected))
	for _, agent := range selected {
		if !isDetected(agent) {
			report, err := inspectAgent(agent, expected, repoRoot, filepath.Join(home, ".agents", "skills"), cfg, home)
			if err != nil {
				return nil, err
			}
			reports = append(reports, report)
			continue
		}
		agentExpected, err := expectedSkillsForAgent(expected, home, cfg, agent.Name)
		if err != nil {
			return nil, err
		}
		report, err := inspectAgent(agent, agentExpected, repoRoot, filepath.Join(home, ".agents", "skills"), cfg, home)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func inspectAgent(agent agentConfig, expected map[string]string, repoRoot string, agentsSkillRoot string, cfg config, home string) (agentReport, error) {
	report := agentReport{
		Name:           agent.Name,
		SkillRoot:      agent.SkillRoot,
		AgentRoot:      agent.AgentRoot,
		Delivery:       normalizeDeliveryMode(agent.Delivery),
		ExpectedSkills: expected,
		Detected:       isDetected(agent),
	}
	if !report.Detected {
		return report, nil
	}
	if usesPluginDelivery(agent) {
		return inspectPluginDeliveryAgent(agent, repoRoot, agentsSkillRoot, cfg, home)
	}
	h := harnessFor(agent.Name)
	if h != nil && h.Skills == SkillsConfigDriven && h.InspectSkills != nil {
		return h.InspectSkills(agent, expected, agentsSkillRoot, cfg, home)
	}

	expectedNames := sortedKeys(expected)
	rootInfo, err := os.Stat(agent.SkillRoot)
	rootMissing := false
	switch {
	case errors.Is(err, fs.ErrNotExist):
		rootMissing = true
	case err != nil:
		return agentReport{}, fmt.Errorf("stat %s: %w", agent.SkillRoot, err)
	case !rootInfo.IsDir():
		report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s exists but is not a directory", agent.SkillRoot))
		report.Missing = append(report.Missing, expectedNames...)
		report.Adds = append(report.Adds, expectedNames...)
		sortReportLists(&report)
		report.Synced = false
		return report, nil
	}

	entryMap := make(map[string]fs.DirEntry)
	if !rootMissing {
		entries, err := os.ReadDir(agent.SkillRoot)
		if err != nil {
			return agentReport{}, fmt.Errorf("read %s: %w", agent.SkillRoot, err)
		}
		for _, entry := range entries {
			entryMap[entry.Name()] = entry
		}
	}

	for _, name := range expectedNames {
		linkPath := filepath.Join(agent.SkillRoot, name)
		entry, ok := entryMap[name]
		if !ok || rootMissing {
			report.Missing = append(report.Missing, name)
			report.Adds = append(report.Adds, name)
			continue
		}

		mode := entry.Type()
		if mode&os.ModeSymlink == 0 {
			report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s exists but is not a symlink", linkPath))
			continue
		}

		rawTarget, err := os.Readlink(linkPath)
		if err != nil {
			return agentReport{}, fmt.Errorf("readlink %s: %w", linkPath, err)
		}
		if linkMatches(linkPath, rawTarget, expected[name]) {
			report.Managed = append(report.Managed, name)
			continue
		}

		report.Drifted = append(report.Drifted, name)
		report.Updates = append(report.Updates, name)
	}

	pluginSkillBases, err := allPluginSkillBasesForAgent(cfg.Plugins, home, agent.Name)
	if err != nil {
		return agentReport{}, err
	}
	pluginSkillBases = append(pluginSkillBases, pluginSourceRootsForAgent(cfg.Plugins, home, agent.Name)...)
	if !rootMissing {
		for name, entry := range entryMap {
			if _, ok := expected[name]; ok {
				continue
			}
			path := filepath.Join(agent.SkillRoot, name)
			if entry.Type()&os.ModeSymlink != 0 {
				rawTarget, err := os.Readlink(path)
				if err != nil {
					return agentReport{}, fmt.Errorf("readlink %s: %w", path, err)
				}
				if isManagedSkillLink(path, rawTarget, repoRoot, agentsSkillRoot) || isExternalSkillLink(path, rawTarget, home) || isPluginSkillLink(path, rawTarget, pluginSkillBases) {
					report.StaleManaged = append(report.StaleManaged, name)
					report.Removes = append(report.Removes, name)
					continue
				}
			}
			report.External = append(report.External, name)
		}
	}

	if err := augmentMCPReport(&report, agent, cfg, home); err != nil {
		return agentReport{}, err
	}
	if err := augmentHookReport(&report, agent, cfg, home); err != nil {
		return agentReport{}, err
	}
	if err := inspectAgentRoles(&report, repoRoot, agent); err != nil {
		return agentReport{}, err
	}
	if h != nil && h.RootInstructions != nil {
		if err := inspectRootInstructions(&report, h.RootInstructions, home); err != nil {
			return agentReport{}, err
		}
	}

	sortReportLists(&report)
	report.Synced = isReportSynced(report)
	return report, nil
}

func isReportSynced(report agentReport) bool {
	if len(report.Missing) > 0 || len(report.Drifted) > 0 || len(report.Conflicts) > 0 || len(report.StaleManaged) > 0 {
		return false
	}
	if len(report.MissingMCP) > 0 || len(report.DriftedMCP) > 0 || len(report.MissingAgent) > 0 || len(report.DriftedAgent) > 0 {
		return false
	}
	if len(report.MissingHook) > 0 || len(report.DriftedHook) > 0 {
		return false
	}
	return report.RootState == "" || report.RootState == stateSynced
}

func inspectRootInstructions(report *agentReport, ri *RootInstructionsCapability, home string) error {
	linkPath := ri.Path(home)
	expectedTarget := ri.Expected(home)
	report.RootPath = linkPath
	report.RootExpected = expectedTarget
	report.RootState = stateMissing

	info, err := os.Lstat(linkPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		report.RootState = stateConflict
		report.RootActual = "non-symlink path exists"
		report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s exists but is not a symlink", linkPath))
		return nil
	}

	rawTarget, err := os.Readlink(linkPath)
	if err != nil {
		return fmt.Errorf("readlink %s: %w", linkPath, err)
	}
	report.RootActual = rawTarget
	if linkMatches(linkPath, rawTarget, expectedTarget) {
		report.RootState = stateSynced
		return nil
	}

	report.RootState = stateDrifted
	return nil
}

func inspectAmpAgent(agent agentConfig, expected map[string]string, cfg config, home string) (agentReport, error) {
	report := agentReport{
		Name:           agent.Name,
		SkillRoot:      agent.SkillRoot,
		AgentRoot:      agent.AgentRoot,
		Delivery:       normalizeDeliveryMode(agent.Delivery),
		ExpectedSkills: expected,
		Detected:       isDetected(agent),
	}
	if !report.Detected {
		return report, nil
	}

	if ok, err := ampHasSkillsPath(home); err != nil {
		return agentReport{}, err
	} else if !ok {
		report.Missing = append(report.Missing, "config amp.skills.path -> ~/.agents/skills")
		report.Adds = append(report.Adds, "config amp.skills.path -> ~/.agents/skills")
	}
	report.Managed = append(report.Managed, sortedKeys(expected)...)
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

func ampHasSkillsPath(home string) (bool, error) {
	configPath := ampSettingsPath(home)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := parseJSONConfig(configPath, data, &raw); err != nil {
		return false, fmt.Errorf("parse %s: %w", configPath, err)
	}
	path, _ := raw["amp.skills.path"].(string)
	return ampSkillsPathConfigured(path, home), nil
}

func inspectHermesAgent(agent agentConfig, expected map[string]string, agentsSkillRoot string, cfg config, home string) (agentReport, error) {
	report := agentReport{
		Name:           agent.Name,
		SkillRoot:      agent.SkillRoot,
		AgentRoot:      agent.AgentRoot,
		Delivery:       normalizeDeliveryMode(agent.Delivery),
		ExpectedSkills: expected,
		Detected:       isDetected(agent),
	}
	if !report.Detected {
		return report, nil
	}

	entries, err := os.ReadDir(agent.SkillRoot)
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			report.External = append(report.External, name)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return agentReport{}, fmt.Errorf("read %s: %w", agent.SkillRoot, err)
	}

	expectedDirs, err := hermesExternalSkillDirs(agentsSkillRoot, cfg, home)
	if err != nil {
		return agentReport{}, err
	}
	ok, stale, err := hermesExternalSkillsDirsState(expectedDirs, cfg, home)
	if err != nil {
		return agentReport{}, err
	}
	if !ok {
		report.Missing = append(report.Missing, "config skills.external_dirs")
		report.Adds = append(report.Adds, "config skills.external_dirs")
	}
	for _, dir := range stale {
		report.StaleManaged = append(report.StaleManaged, "config dir "+dir)
		report.Removes = append(report.Removes, "config dir "+dir)
	}
	report.Managed = append(report.Managed, sortedKeys(report.ExpectedSkills)...)
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

func hermesExternalSkillDirs(agentsSkillRoot string, cfg config, home string) ([]string, error) {
	dirs := []string{agentsSkillRoot}
	pluginDirs, err := pluginSkillBasesForAgent(cfg.Plugins, home, agentHermes)
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, pluginDirs...)
	return dirs, nil
}

func hermesExternalSkillsDirsState(expected []string, cfg config, configHome string) (bool, []string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, nil, fmt.Errorf("resolve home: %w", err)
	}
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, nil, fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, nil, fmt.Errorf("parse %s: %w", configPath, err)
	}

	skillsRaw, ok := raw["skills"]
	if !ok {
		return false, nil, nil
	}
	skills, ok := skillsRaw.(map[string]interface{})
	if !ok {
		return false, nil, nil
	}
	dirsRaw, ok := skills["external_dirs"]
	if !ok {
		return false, nil, nil
	}
	dirs, ok := dirsRaw.([]interface{})
	if !ok {
		return false, nil, nil
	}

	expectedSet := make(map[string]bool, len(expected))
	for _, dir := range expected {
		expectedSet[dir] = true
	}
	pruneRoots := hermesStaleDirPruneRoots(cfg, configHome)

	var stale []string
	found := make(map[string]bool, len(expected))
	for _, d := range dirs {
		s, ok := d.(string)
		if !ok {
			continue
		}
		expanded := expandPath(strings.TrimSpace(s), home)
		if expectedSet[expanded] {
			found[expanded] = true
			continue
		}
		if pathUnderAny(expanded, pruneRoots) {
			stale = append(stale, expanded)
		}
	}

	for _, dir := range expected {
		if !found[dir] {
			return false, stale, nil
		}
	}
	return true, stale, nil
}

func linkMatches(linkPath string, rawTarget string, expectedTarget string) bool {
	if rawTarget == expectedTarget {
		return true
	}

	actualAbs := absoluteTarget(linkPath, rawTarget)
	expectedAbs := absoluteTarget(linkPath, expectedTarget)
	if actualAbs == expectedAbs {
		return true
	}

	actualResolved, err := filepath.EvalSymlinks(actualAbs)
	if err != nil {
		return false
	}
	expectedResolved, err := filepath.EvalSymlinks(expectedAbs)
	if err != nil {
		return false
	}
	return actualResolved == expectedResolved
}

func isManagedSkillLink(linkPath string, rawTarget string, repoRoot string, agentsSkillRoot string) bool {
	targetAbs := absoluteTarget(linkPath, rawTarget)
	if targetAbs == agentsSkillRoot || strings.HasPrefix(targetAbs, agentsSkillRoot+string(os.PathSeparator)) {
		return true
	}
	if resolvedTarget, err := filepath.EvalSymlinks(targetAbs); err == nil {
		if resolvedAgentsRoot, err := filepath.EvalSymlinks(agentsSkillRoot); err == nil && (resolvedTarget == resolvedAgentsRoot || strings.HasPrefix(resolvedTarget, resolvedAgentsRoot+string(os.PathSeparator))) {
			return true
		}
	}

	resolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		return false
	}
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	if resolved == repoSkillsRoot || strings.HasPrefix(resolved, repoSkillsRoot+string(os.PathSeparator)) {
		return true
	}
	resolvedRepoSkillsRoot, err := filepath.EvalSymlinks(repoSkillsRoot)
	if err != nil {
		return false
	}
	return resolved == resolvedRepoSkillsRoot || strings.HasPrefix(resolved, resolvedRepoSkillsRoot+string(os.PathSeparator))
}

func absoluteTarget(linkPath string, rawTarget string) string {
	if filepath.IsAbs(rawTarget) {
		return filepath.Clean(rawTarget)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(linkPath), rawTarget))
}
