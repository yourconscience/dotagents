package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type config struct {
	Version int           `yaml:"version"`
	Agents  []agentConfig `yaml:"agents"`
}

type agentConfig struct {
	Name      string `yaml:"name"`
	Enabled   bool   `yaml:"enabled"`
	SkillRoot string `yaml:"skill_root"`
}

type repoLinkReport struct {
	Path           string
	ExpectedTarget string
	ActualTarget   string
	State          string
}

type agentReport struct {
	Name         string
	SkillRoot    string
	Managed      []string
	Drifted      []string
	Missing      []string
	Conflicts    []string
	StaleManaged []string
	External     []string
	Adds         []string
	Updates      []string
	Removes      []string
	Synced       bool
}

type runOptions struct {
	ConfigPath string
	Agents     string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("missing subcommand")
	}

	switch args[0] {
	case "status":
		opts, err := parseSubcommandFlags("status", args[1:])
		if err != nil {
			return err
		}
		return runStatus(opts)
	case "sync":
		opts, err := parseSubcommandFlags("sync", args[1:])
		if err != nil {
			return err
		}
		return runSync(opts)
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func parseSubcommandFlags(name string, args []string) (runOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts runOptions
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names to use for this run")

	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	if fs.NArg() != 0 {
		return runOptions{}, fmt.Errorf("%s does not accept positional arguments", name)
	}

	return opts, nil
}

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
	reports, err := inspectAgents(selected, expected, repoRoot, home)
	if err != nil {
		return err
	}

	printReport("status", repoRoot, repoReport, reports)
	if repoReport.State != "synced" {
		return errors.New("dotagents is not fully synced")
	}
	for _, report := range reports {
		if !report.Synced {
			return fmt.Errorf("%s is not synced", report.Name)
		}
	}

	_ = cfg
	return nil
}

func runSync(opts runOptions) error {
	repoRoot, home, _, selected, err := loadContext(opts)
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
	reports, err := inspectAgents(selected, expected, repoRoot, home)
	if err != nil {
		return err
	}
	preflight := cloneReports(reports)

	var conflicts []string
	if repoReport.State == "conflict" {
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

	repoReport, err = inspectRepoLink(repoRoot, home)
	if err != nil {
		return err
	}
	reports, err = inspectAgents(selected, expected, repoRoot, home)
	if err != nil {
		return err
	}
	restoreSyncActions(reports, preflight)

	printReport("sync", repoRoot, repoReport, reports)
	return nil
}

func loadContext(opts runOptions) (string, string, config, []agentConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", config{}, nil, fmt.Errorf("resolve home: %w", err)
	}

	repoRoot, err := findRepoRoot()
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
		configPath = filepath.Join(repoRoot, "dotagents.yaml")
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
	if err != nil {
		return config{}, fmt.Errorf("parse config %s: %w", configPath, err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if len(cfg.Agents) == 0 {
		return config{}, errors.New("config has no agents")
	}

	seen := make(map[string]struct{})
	for i := range cfg.Agents {
		cfg.Agents[i].Name = normalizeAgentName(cfg.Agents[i].Name)
		cfg.Agents[i].SkillRoot = expandPath(cfg.Agents[i].SkillRoot, home)
		if cfg.Agents[i].Name == "" {
			return config{}, errors.New("config agent name cannot be empty")
		}
		if cfg.Agents[i].SkillRoot == "" {
			return config{}, fmt.Errorf("config agent %s is missing skill_root", cfg.Agents[i].Name)
		}
		if _, ok := seen[cfg.Agents[i].Name]; ok {
			return config{}, fmt.Errorf("config agent %s is duplicated", cfg.Agents[i].Name)
		}
		seen[cfg.Agents[i].Name] = struct{}{}
	}

	return cfg, nil
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

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	for {
		if hasDir(filepath.Join(dir, "skills")) && hasFile(filepath.Join(dir, "AGENTS.md")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("could not find dotagents repo root from current directory")
}

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
		expectedTarget := expected[name]
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
		if linkMatches(linkPath, rawTarget, expectedTarget) {
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

func applyRepoLink(report repoLinkReport) error {
	switch report.State {
	case "synced":
		return nil
	case "missing":
		return os.Symlink(report.ExpectedTarget, report.Path)
	case "drifted":
		if err := os.Remove(report.Path); err != nil {
			return fmt.Errorf("remove %s: %w", report.Path, err)
		}
		return os.Symlink(report.ExpectedTarget, report.Path)
	case "conflict":
		return fmt.Errorf("%s is not a symlink", report.Path)
	default:
		return fmt.Errorf("unsupported repo link state %q", report.State)
	}
}

func applyAgentSync(reports []agentReport, expected map[string]string) error {
	for _, report := range reports {
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

func printReport(mode string, repoRoot string, repoReport repoLinkReport, reports []agentReport) {
	fmt.Printf("dotagents %s\n", mode)
	fmt.Printf("repo: %s\n", repoRoot)
	fmt.Printf("~/.agents: %s", repoReport.State)
	if repoReport.State == "synced" {
		fmt.Printf(" -> %s\n", repoReport.ExpectedTarget)
	} else {
		fmt.Printf(" (expected %s", repoReport.ExpectedTarget)
		if repoReport.ActualTarget != "" {
			fmt.Printf(", actual %s", repoReport.ActualTarget)
		}
		fmt.Printf(")\n")
	}
	fmt.Println()

	for _, report := range reports {
		fmt.Printf("%s\n", report.Name)
		fmt.Printf("  skill root: %s\n", report.SkillRoot)
		if report.Synced {
			fmt.Println("  sync: synced")
		} else {
			fmt.Println("  sync: drifted")
		}
		fmt.Printf("  managed (%d): %s\n", len(report.Managed), displayList(report.Managed))
		fmt.Printf("  external (%d): %s\n", len(report.External), displayList(report.External))
		if len(report.Missing) > 0 {
			fmt.Printf("  missing (%d): %s\n", len(report.Missing), displayList(report.Missing))
		}
		if len(report.Drifted) > 0 {
			fmt.Printf("  drifted (%d): %s\n", len(report.Drifted), displayList(report.Drifted))
		}
		if len(report.StaleManaged) > 0 {
			fmt.Printf("  stale managed (%d): %s\n", len(report.StaleManaged), displayList(report.StaleManaged))
		}
		if len(report.Conflicts) > 0 {
			fmt.Printf("  conflicts (%d): %s\n", len(report.Conflicts), displayList(report.Conflicts))
		}
		if mode == "sync" {
			fmt.Printf("  sync actions: add=%d update=%d remove=%d\n", len(report.Adds), len(report.Updates), len(report.Removes))
		}
		fmt.Println()
	}
}

func displayList(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func sortReportLists(report *agentReport) {
	sort.Strings(report.Managed)
	sort.Strings(report.Drifted)
	sort.Strings(report.Missing)
	sort.Strings(report.Conflicts)
	sort.Strings(report.StaleManaged)
	sort.Strings(report.External)
	sort.Strings(report.Adds)
	sort.Strings(report.Updates)
	sort.Strings(report.Removes)
}

func cloneReports(reports []agentReport) []agentReport {
	cloned := make([]agentReport, len(reports))
	copy(cloned, reports)
	return cloned
}

func restoreSyncActions(current []agentReport, preflight []agentReport) {
	index := make(map[string]agentReport, len(preflight))
	for _, report := range preflight {
		index[report.Name] = report
	}
	for i := range current {
		if original, ok := index[current[i].Name]; ok {
			current[i].Adds = append([]string{}, original.Adds...)
			current[i].Updates = append([]string{}, original.Updates...)
			current[i].Removes = append([]string{}, original.Removes...)
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func normalizeAgentName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
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

func hasDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func hasFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func printUsage() {
	fmt.Println("dotagents")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./bin/dotagents status [--config path] [--agents codex,claude-code]")
	fmt.Println("  go run ./bin/dotagents sync   [--config path] [--agents codex,claude-code]")
}
