package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func runStatus(opts runOptions) error {
	repoRoot, home, cfg, selected, err := loadContext(opts)
	if err != nil {
		return err
	}

	repoReport, err := inspectRepoLink(repoRoot, home)
	if err != nil {
		return err
	}

	expected, err := expectedSkills(repoRoot, home)
	if err != nil {
		return err
	}
	reports, err := inspectAgents(selected, expected, repoRoot, home, cfg)
	if err != nil {
		return err
	}

	printReport("status", repoRoot, repoReport, reports)
	if repoReport.State != stateSynced {
		return errors.New("dotagents is not fully synced")
	}
	for _, report := range reports {
		if report.Detected && !report.Synced {
			return fmt.Errorf("%s is not synced", report.Name)
		}
	}

	return nil
}

func runSync(opts runOptions) error {
	repoRoot, home, cfg, selected, err := loadContext(opts)
	if err != nil {
		return err
	}

	repoReport, err := inspectRepoLink(repoRoot, home)
	if err != nil {
		return err
	}

	expected, err := expectedSkills(repoRoot, home)
	if err != nil {
		return err
	}
	reports, err := inspectAgents(selected, expected, repoRoot, home, cfg)
	if err != nil {
		return err
	}
	preflight := cloneReports(reports)

	var conflicts []string
	if repoReport.State == stateConflict {
		conflicts = append(conflicts, fmt.Sprintf("repo link: %s", repoReport.Path))
	}
	for _, report := range reports {
		for _, conflict := range report.Conflicts {
			conflicts = append(conflicts, fmt.Sprintf("%s: %s", report.Name, conflict))
		}
	}
	if len(conflicts) > 0 {
		printReport("sync", repoRoot, repoReport, reports)
		return fmt.Errorf("sync aborted due to conflicts: %s", strings.Join(conflicts, "; "))
	}

	if err := applyRepoLink(repoReport); err != nil {
		return err
	}
	if err := applyAgentSync(reports, expected); err != nil {
		return err
	}
	if err := applyAgentRoleSync(reports, selected, repoRoot); err != nil {
		return err
	}
	if err := applyAgentMCPSync(reports, cfg, home); err != nil {
		return err
	}
	if err := applyAgentRootInstructionSync(reports); err != nil {
		return err
	}

	repoReport, err = inspectRepoLink(repoRoot, home)
	if err != nil {
		return err
	}
	reports, err = inspectAgents(selected, expected, repoRoot, home, cfg)
	if err != nil {
		return err
	}
	restoreSyncActions(reports, preflight)

	printReport("sync", repoRoot, repoReport, reports)
	return nil
}

func applyAgentRootInstructionSync(reports []agentReport) error {
	for _, report := range reports {
		if !report.Detected || report.RootPath == "" {
			continue
		}
		switch report.RootState {
		case stateSynced:
			continue
		case stateMissing:
			if err := os.MkdirAll(filepath.Dir(report.RootPath), 0o755); err != nil {
				return fmt.Errorf("create %s: %w", filepath.Dir(report.RootPath), err)
			}
		case stateDrifted:
			if err := os.Remove(report.RootPath); err != nil {
				return fmt.Errorf("remove %s before relink: %w", report.RootPath, err)
			}
		case stateConflict:
			return fmt.Errorf("%s is not a symlink", report.RootPath)
		default:
			return fmt.Errorf("unsupported root instruction state %q for %s", report.RootState, report.Name)
		}
		if err := os.Symlink(report.RootExpected, report.RootPath); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", report.RootPath, report.RootExpected, err)
		}
	}
	return nil
}

func applyRepoLink(report repoLinkReport) error {
	switch report.State {
	case stateSynced:
		return nil
	case stateMissing:
		return os.Symlink(report.ExpectedTarget, report.Path)
	case stateDrifted:
		if err := os.Remove(report.Path); err != nil {
			return fmt.Errorf("remove %s: %w", report.Path, err)
		}
		return os.Symlink(report.ExpectedTarget, report.Path)
	case stateConflict:
		return fmt.Errorf("%s is not a symlink", report.Path)
	default:
		return fmt.Errorf("unsupported repo link state %q", report.State)
	}
}

func applyAgentSync(reports []agentReport, expected map[string]string) error {
	for _, report := range reports {
		if !report.Detected {
			continue
		}
		if report.Name == agentAmp || report.Name == agentHermes {
			continue
		}
		if len(report.Conflicts) > 0 {
			return fmt.Errorf("%s has conflicts", report.Name)
		}

		if err := os.MkdirAll(report.SkillRoot, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", report.SkillRoot, err)
		}

		for _, name := range report.Removes {
			path := filepath.Join(report.SkillRoot, name)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove %s: %w", path, err)
			}
		}

		for _, name := range append(append([]string{}, report.Adds...), report.Updates...) {
			path := filepath.Join(report.SkillRoot, name)
			if _, err := os.Lstat(path); err == nil {
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("remove %s before relink: %w", path, err)
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("stat %s: %w", path, err)
			}
			if err := os.Symlink(expected[name], path); err != nil {
				return fmt.Errorf("symlink %s -> %s: %w", path, expected[name], err)
			}
		}
	}

	return nil
}
