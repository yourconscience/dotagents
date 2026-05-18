package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func runSetup(opts runOptions) error {
	repoRoot, home, _, selected, err := loadContext(opts)
	if err != nil {
		return err
	}

	fmt.Println("dotagents setup")
	fmt.Printf("repo: %s\n\n", repoRoot)

	// 1. Fix ~/.agents symlink
	repoReport, err := inspectRepoLink(repoRoot, home)
	if err != nil {
		return err
	}
	if err := applyRepoLink(repoReport); err != nil {
		return err
	}
	repoReport, err = inspectRepoLink(repoRoot, home)
	if err != nil {
		return err
	}
	fmt.Printf("~/.agents: %s -> %s\n\n", repoReport.State, repoReport.ExpectedTarget)

	// 2. Patch agent configs for detected agents
	for _, agent := range selected {
		if !isDetected(agent) {
			fmt.Printf("%s: not detected, skipping\n", agent.Name)
			continue
		}
		patched, err := patchAgentConfig(agent, home)
		if err != nil {
			fmt.Printf("%s: config patch failed: %v\n", agent.Name, err)
			continue
		}
		if patched {
			fmt.Printf("%s: config patched (added ~/.agents/skills)\n", agent.Name)
		} else {
			fmt.Printf("%s: config already set\n", agent.Name)
		}
	}
	fmt.Println()

	migrated, err := migrateLegacyMemoryHookPaths(home, repoRoot)
	if err != nil {
		fmt.Printf("memory hooks: legacy path migration failed: %v\n", err)
	} else if migrated > 0 {
		fmt.Printf("memory hooks: migrated legacy paths in %d config file(s)\n\n", migrated)
	}

	// 3. Run sync
	return runSync(opts)
}

func patchAgentConfig(agent agentConfig, home string) (bool, error) {
	switch agent.Name {
	case agentAmp:
		return patchAmpConfig(home)
	case agentHermes:
		return patchHermesConfig(home)
	case "openclaw":
		return patchOpenClawConfig(home)
	default:
		return false, nil
	}
}

func patchAmpConfig(home string) (bool, error) {
	configPath := ampSettingsPath(home)
	data, err := os.ReadFile(configPath)
	raw := map[string]interface{}{}
	if err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("read %s: %w", configPath, err)
		}
	} else if err := parseJSONConfig(configPath, data, &raw); err != nil {
		return false, fmt.Errorf("parse %s: %w", configPath, err)
	}

	current, _ := raw["amp.skills.path"].(string)
	if ampSkillsPathConfigured(current, home) {
		return false, nil
	}
	raw["amp.skills.path"] = appendAmpSkillsPath(current)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal %s: %w", configPath, err)
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", filepath.Dir(configPath), err)
	}
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", configPath, err)
	}
	return true, nil
}

func ampSettingsPath(home string) string {
	repoRoot, _, err := findRoots()
	if err == nil {
		if path := existingAmpSettingsPath(filepath.Join(repoRoot, ".amp")); path != "" {
			return path
		}
	}
	return ampSettingsPathForRoots("", home)
}

func ampSettingsPathForRoots(repoRoot string, home string) string {
	if repoRoot != "" {
		if path := existingAmpSettingsPath(filepath.Join(repoRoot, ".amp")); path != "" {
			return path
		}
	}
	return defaultAmpSettingsPath(filepath.Join(home, ".config", "amp"))
}

func existingAmpSettingsPath(configDir string) string {
	jsonPath := filepath.Join(configDir, "settings.json")
	jsoncPath := filepath.Join(configDir, "settings.jsonc")
	if hasFile(jsonPath) {
		return jsonPath
	}
	if hasFile(jsoncPath) {
		return jsoncPath
	}
	return ""
}

func defaultAmpSettingsPath(configDir string) string {
	jsonPath := filepath.Join(configDir, "settings.json")
	jsoncPath := filepath.Join(configDir, "settings.jsonc")
	if hasFile(jsonPath) || !hasFile(jsoncPath) {
		return jsonPath
	}
	return jsoncPath
}

func ampSkillsPathConfigured(raw string, home string) bool {
	target := filepath.Join(home, ".agents", "skills")
	for _, part := range strings.Split(raw, ":") {
		if expandPath(strings.TrimSpace(part), home) == target {
			return true
		}
	}
	return false
}

func appendAmpSkillsPath(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ampSkillsPath
	}
	return strings.TrimRight(raw, ":") + ":" + ampSkillsPath
}

func patchHermesConfig(home string) (bool, error) {
	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parse %s: %w", configPath, err)
	}

	skillsRaw, ok := raw["skills"]
	if !ok {
		skillsRaw = map[string]interface{}{}
		raw["skills"] = skillsRaw
	}
	skills, ok := skillsRaw.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("skills key in %s is not a map", configPath)
	}

	targetExpanded := filepath.Join(home, ".agents", "skills")
	dirsRaw, ok := skills["external_dirs"]
	if ok {
		dirs, ok := dirsRaw.([]interface{})
		if ok {
			for _, d := range dirs {
				if s, ok := d.(string); ok {
					expanded := expandPath(strings.TrimSpace(s), home)
					if s == ampSkillsPath || expanded == targetExpanded {
						return false, nil
					}
				}
			}
			skills["external_dirs"] = append(dirs, ampSkillsPath)
		} else {
			skills["external_dirs"] = []interface{}{ampSkillsPath}
		}
	} else {
		skills["external_dirs"] = []interface{}{ampSkillsPath}
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return false, fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", configPath, err)
	}
	return true, nil
}

func patchOpenClawConfig(home string) (bool, error) {
	configPath := filepath.Join(home, ".openclaw", "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", configPath, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parse %s: %w", configPath, err)
	}

	skillsRaw, ok := raw["skills"]
	if !ok {
		skillsRaw = map[string]interface{}{}
		raw["skills"] = skillsRaw
	}
	skills, ok := skillsRaw.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("skills key in %s is not a map", configPath)
	}

	loadRaw, ok := skills["load"]
	if !ok {
		loadRaw = map[string]interface{}{}
		skills["load"] = loadRaw
	}
	load, ok := loadRaw.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("skills.load in %s is not a map", configPath)
	}

	dirsRaw, ok := load["extraDirs"]
	if ok {
		dirs, ok := dirsRaw.([]interface{})
		if ok {
			for _, d := range dirs {
				if s, ok := d.(string); ok && s == ampSkillsPath {
					return false, nil
				}
			}
			load["extraDirs"] = append(dirs, ampSkillsPath)
		} else {
			load["extraDirs"] = []interface{}{ampSkillsPath}
		}
	} else {
		load["extraDirs"] = []interface{}{ampSkillsPath}
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", configPath, err)
	}
	return true, nil
}

func runPull(opts runOptions) error {
	repoRoot, _, _, _, err := loadContext(opts)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "-C", repoRoot, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	return runSync(opts)
}

type cronOptions struct {
	runOptions
	Remove   bool
	Deps     bool
	Interval string
}

func runCron(opts cronOptions) error {
	repoRoot, _, _, _, err := loadContext(opts.runOptions)
	if err != nil {
		return err
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found on PATH: %w", err)
	}

	cronCmd, interval := cronCommandForOptions(repoRoot, goPath, opts)

	if opts.Remove {
		return removeCronEntry(cronCmd)
	}
	return installCronEntry(cronCmd, interval)
}

func cronCommandForOptions(repoRoot string, goPath string, opts cronOptions) (string, string) {
	toolPath := filepath.Join(repoRoot, "skills", "dotagents", "tools", "dotagents")
	mode := "pull"
	interval := opts.Interval
	if opts.Deps {
		mode = "deps update"
		if interval == "" || interval == "30m" {
			interval = "weekly"
		}
	}
	if interval == "" {
		interval = "30m"
	}
	return fmt.Sprintf(". \"$HOME/.profile\" 2>/dev/null; cd %s && %s run %s %s", repoRoot, goPath, toolPath, mode), interval
}

func installCronEntry(cronCmd string, interval string) error {
	schedule := intervalToSchedule(interval)
	entry := fmt.Sprintf("%s %s", schedule, cronCmd)

	existing, _ := exec.Command("crontab", "-l").Output()
	lines := strings.Split(string(existing), "\n")
	for _, line := range lines {
		if strings.Contains(line, cronCmd) {
			fmt.Println("cron entry already exists:")
			fmt.Printf("  %s\n", line)
			return nil
		}
	}

	newCrontab := string(existing)
	if !strings.HasSuffix(newCrontab, "\n") && newCrontab != "" {
		newCrontab += "\n"
	}
	newCrontab += entry + "\n"

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("crontab install: %w", err)
	}

	fmt.Printf("installed cron entry:\n  %s\n", entry)
	return nil
}

func removeCronEntry(cronCmd string) error {
	existing, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return fmt.Errorf("crontab read: %w", err)
	}

	var kept []string
	removed := 0
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.Contains(line, cronCmd) {
			fmt.Printf("removed: %s\n", line)
			removed++
			continue
		}
		kept = append(kept, line)
	}

	if removed == 0 {
		fmt.Println("no dotagents cron entry found")
		return nil
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(kept, "\n"))
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func intervalToSchedule(interval string) string {
	switch interval {
	case "5m":
		return "*/5 * * * *"
	case "15m":
		return "*/15 * * * *"
	case "30m":
		return "*/30 * * * *"
	case "1h", "hourly":
		return "0 * * * *"
	case "6h":
		return "0 */6 * * *"
	case "12h":
		return "0 */12 * * *"
	case "daily":
		return "0 4 * * *"
	case "weekly":
		return "0 4 * * 1"
	default:
		return "*/30 * * * *"
	}
}
