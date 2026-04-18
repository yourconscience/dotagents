package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func expectedSkills(repoRoot string, home string) (map[string]string, error) {
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", skillsDir, err)
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
	return expected, nil
}

func inspectRepoLink(repoRoot string, home string) (repoLinkReport, error) {
	linkPath := filepath.Join(home, ".agents")
	report := repoLinkReport{
		Path:           linkPath,
		ExpectedTarget: repoRoot,
		State:          "missing",
	}

	info, err := os.Lstat(linkPath)
	if errors.Is(err, fs.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return repoLinkReport{}, fmt.Errorf("stat %s: %w", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		report.State = "conflict"
		report.ActualTarget = "non-symlink path exists"
		return report, nil
	}

	rawTarget, err := os.Readlink(linkPath)
	if err != nil {
		return repoLinkReport{}, fmt.Errorf("readlink %s: %w", linkPath, err)
	}
	report.ActualTarget = rawTarget
	if linkMatches(linkPath, rawTarget, repoRoot) {
		report.State = "synced"
		return report, nil
	}

	report.State = "drifted"
	return report, nil
}

func inspectAgents(selected []agentConfig, expected map[string]string, repoRoot string, home string) ([]agentReport, error) {
	reports := make([]agentReport, 0, len(selected))
	for _, agent := range selected {
		report, err := inspectAgent(agent, expected, repoRoot, filepath.Join(home, ".agents", "skills"))
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func inspectAgent(agent agentConfig, expected map[string]string, repoRoot string, agentsSkillRoot string) (agentReport, error) {
	report := agentReport{
		Name:      agent.Name,
		SkillRoot: agent.SkillRoot,
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
				if isManagedSkillLink(path, rawTarget, repoRoot, agentsSkillRoot) {
					report.StaleManaged = append(report.StaleManaged, name)
					report.Removes = append(report.Removes, name)
					continue
				}
			}
			report.External = append(report.External, name)
		}
	}

	sortReportLists(&report)
	report.Synced = len(report.Missing) == 0 && len(report.Drifted) == 0 && len(report.Conflicts) == 0 && len(report.StaleManaged) == 0
	return report, nil
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

	resolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		return false
	}
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	return resolved == repoSkillsRoot || strings.HasPrefix(resolved, repoSkillsRoot+string(os.PathSeparator))
}

func absoluteTarget(linkPath string, rawTarget string) string {
	if filepath.IsAbs(rawTarget) {
		return filepath.Clean(rawTarget)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(linkPath), rawTarget))
}
