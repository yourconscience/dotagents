package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func runSetup(opts runOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	configPath, err := resolveConfigPath(opts.ConfigPath, home)
	if err != nil {
		return err
	}
	repoRoot := filepath.Dir(configPath)
	streams := setupStreams(opts)

	fmt.Fprintln(streams.out, "dotagents setup")
	fmt.Fprintf(streams.out, "config root: %s\n\n", repoRoot)

	cfg, err := loadSetupConfig(configPath, home)
	if err != nil {
		return err
	}
	detected, err := detectDefaultAgents(opts.Agents)
	if err != nil {
		return err
	}
	if len(detected) == 0 {
		return errors.New("no supported agents detected on PATH")
	}
	priorCfg := cfg
	upsertSetupAgents(&cfg, detected)
	if err := applyMemoryTier(&cfg, opts.MemoryTier, repoRoot, home); err != nil {
		return err
	}

	skills, roles, mcps, err := scanNativeImports(cfg, detected, repoRoot, home)
	if err != nil {
		return err
	}
	if err := importNativeContent(repoRoot, &cfg, detected, skills, roles, mcps, streams); err != nil {
		return err
	}
	if err := ensureStarterAssets(repoRoot, configPath); err != nil {
		return err
	}
	if err := ensureMemoryHookExecutables(repoRoot); err != nil {
		return err
	}
	if err := writeSetupConfig(configPath, cfg, home); err != nil {
		return err
	}

	for _, agent := range detected {
		patched, err := patchAgentConfig(agent, home, cfg)
		if err != nil {
			fmt.Fprintf(streams.out, "%s: config patch failed: %v\n", agent.Name, err)
			continue
		}
		if patched {
			fmt.Fprintf(streams.out, "%s: config patched (added ~/.agents/skills)\n", agent.Name)
		} else {
			fmt.Fprintf(streams.out, "%s: config already set\n", agent.Name)
		}
	}
	migrated, err := migrateLegacyMemoryHookPaths(home, repoRoot)
	if err != nil {
		fmt.Fprintf(streams.out, "memory hooks: legacy path migration failed: %v\n", err)
	} else if migrated > 0 {
		fmt.Fprintf(streams.out, "memory hooks: migrated legacy paths in %d config file(s)\n\n", migrated)
	}
	cleaned, err := removeNativeManagedMemoryHooks(home, repoRoot, priorCfg, cfg)
	if err != nil {
		fmt.Fprintf(streams.out, "memory hooks: native cleanup failed: %v\n", err)
	} else if cleaned > 0 {
		fmt.Fprintf(streams.out, "memory hooks: removed previous managed commands from %d native config file(s)\n\n", cleaned)
	}

	syncOpts := opts
	syncOpts.ConfigPath = configPath
	return runSync(syncOpts)
}

func patchAgentConfig(agent agentConfig, home string, cfg config) (bool, error) {
	h := harnessFor(agent.Name)
	if h != nil && h.Setup != nil {
		return h.Setup(home, cfg)
	}
	return false, nil
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
		return dotagentsSkillsPathValue
	}
	return strings.TrimRight(raw, ":") + ":" + dotagentsSkillsPathValue
}

func patchHermesConfig(home string, _ config) (bool, error) {
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

	targets := []string{filepath.Join(home, ".agents", "skills")}

	targetSet := make(map[string]bool, len(targets))
	for _, target := range targets {
		targetSet[target] = true
	}
	existing := make(map[string]bool)
	dirsRaw, ok := skills["external_dirs"]
	var dirs []interface{}
	changed := !ok
	if ok {
		if existingDirs, ok := dirsRaw.([]interface{}); ok {
			for _, d := range existingDirs {
				if s, isStr := d.(string); isStr {
					expanded := expandPath(strings.TrimSpace(s), home)
					existing[expanded] = true
				}
				dirs = append(dirs, d)
			}
		}
	}
	for _, target := range targets {
		if existing[target] {
			continue
		}
		dirs = append(dirs, hermesExternalDirValue(home, target))
		changed = true
	}
	if !changed {
		return false, nil
	}
	skills["external_dirs"] = dirs

	out, err := yaml.Marshal(raw)
	if err != nil {
		return false, fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", configPath, err)
	}
	return true, nil
}

func hermesExternalDirValue(home string, target string) string {
	if target == filepath.Join(home, ".agents", "skills") {
		return dotagentsSkillsPathValue
	}
	return target
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

const (
	cronIntervalDefault = "30m"
	cronIntervalWeekly  = "weekly"
)

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
	toolPath := filepath.Join(repoRoot, "cmd", "dotagents")
	mode := "pull"
	interval := opts.Interval
	if opts.Deps {
		mode = "deps update"
		if interval == "" || interval == cronIntervalDefault {
			interval = cronIntervalWeekly
		}
	}
	if interval == "" {
		interval = cronIntervalDefault
	}
	return fmt.Sprintf(". \"$HOME/.profile\" 2>/dev/null; cd %q && %q run %q %s", repoRoot, goPath, toolPath, mode), interval
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
	case cronIntervalDefault:
		return "*/30 * * * *"
	case "1h", "hourly":
		return "0 * * * *"
	case "6h":
		return "0 */6 * * *"
	case "12h":
		return "0 */12 * * *"
	case "daily":
		return "0 4 * * *"
	case cronIntervalWeekly:
		return "0 4 * * 1"
	default:
		return "*/30 * * * *"
	}
}
