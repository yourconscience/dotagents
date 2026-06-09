package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	agentAmp        = "amp"
	agentClaudeCode = "claude-code"
	agentCodex      = "codex"
	agentDroid      = "droid"
	agentHermes     = "hermes"
	agentPi         = "pi"
	ampSkillsPath   = "~/.agents/skills"
)

type checkResult struct {
	name   string
	status string // "pass", "warn", "fail"
	detail string
}

const (
	checkStatusPass = "pass"
	checkStatusWarn = "warn"
	checkStatusFail = "fail"
)

func runDoctor(opts runOptions) error {
	repoRoot, home, cfg, _, err := loadContext(opts)
	if err != nil {
		return err
	}

	fmt.Println("dotagents doctor")
	fmt.Printf("repo: %s\n\n", repoRoot)

	var results []checkResult

	results = append(results, checkSkillFrontmatter(repoRoot))
	results = append(results, checkSkillSpec(repoRoot))
	results = append(results, checkAgentRoles(repoRoot))
	for _, name := range sortedHarnessNames() {
		for _, check := range getHarnesses()[name].DoctorChecks {
			results = append(results, check.Run(repoRoot, home, cfg))
		}
	}
	results = append(results, checkAgnix(repoRoot))
	results = append(results, checkAgentsMDSize(repoRoot))
	results = append(results, checkREADMESkillList(repoRoot))
	results = append(results, checkMemsearchIndex(home))
	results = append(results, checkExternalPackageAge(repoRoot, cfg, opts.SkipPackageAge, timeNow()))
	results = append(results, checkExternalSkillSources(cfg, home))
	results = append(results, checkExternalSkillLock(repoRoot, cfg, home))
	results = append(results, checkExternalSkillAudit(cfg, home))
	results = append(results, checkFirstPartyPlugins(cfg))

	fmt.Println("checks:")
	labelWidth := 0
	for _, r := range results {
		if len(r.name) > labelWidth {
			labelWidth = len(r.name)
		}
	}

	passed, warned, failed := 0, 0, 0
	for _, r := range results {
		padding := strings.Repeat(".", labelWidth-len(r.name)+3)
		fmt.Printf("  %s %s %s (%s)\n", r.name, padding, r.status, r.detail)
		switch r.status {
		case checkStatusPass:
			passed++
		case checkStatusWarn:
			warned++
		case checkStatusFail:
			failed++
		}
	}

	fmt.Printf("\n%d passed, %d warning, %d failed\n", passed, warned, failed)
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	if warned > 0 {
		return fmt.Errorf("%d warning(s)", warned)
	}
	return nil
}

func checkAgentRoles(repoRoot string) checkResult {
	roles, err := loadAgentRoles(repoRoot)
	if err != nil {
		return checkResult{"agent roles", checkStatusFail, err.Error()}
	}
	if len(roles) == 0 {
		return checkResult{"agent roles", checkStatusWarn, "no agents/*.yaml roles found"}
	}
	return checkResult{"agent roles", checkStatusPass, fmt.Sprintf("%d roles valid", len(roles))}
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func checkSkillFrontmatter(repoRoot string) checkResult {
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"skill frontmatter", checkStatusFail, fmt.Sprintf("cannot read skills/: %s", err)}
	}

	var invalid []string
	total := 0
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillMD := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if !hasFile(skillMD) {
			continue
		}
		total++
		fm, err := parseSkillFrontmatter(skillMD)
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%s (parse error: %v)", entry.Name(), err))
		} else if fm.Name == "" || fm.Description == "" {
			invalid = append(invalid, entry.Name())
		}
	}

	if len(invalid) > 0 {
		return checkResult{"skill frontmatter", checkStatusFail, fmt.Sprintf("%s missing name/description", strings.Join(invalid, ", "))}
	}
	return checkResult{"skill frontmatter", checkStatusPass, fmt.Sprintf("%d skills valid", total)}
}

func parseSkillFrontmatter(path string) (skillFrontmatter, error) {
	f, err := os.Open(path)
	if err != nil {
		return skillFrontmatter{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// First line must be ---
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return skillFrontmatter{}, fmt.Errorf("no frontmatter")
	}

	var lines []string
	foundEnd := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			foundEnd = true
			break
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return skillFrontmatter{}, err
	}
	if !foundEnd {
		return skillFrontmatter{}, fmt.Errorf("frontmatter not terminated")
	}

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &fm); err != nil {
		return skillFrontmatter{}, err
	}
	return fm, nil
}

func isAgentDetected(cfg config, name string) bool {
	for _, agent := range cfg.Agents {
		if agent.Name == name && isDetected(agent) {
			return true
		}
	}
	return false
}

func checkHermesDirectMirrors(repoRoot string, home string, cfg config) checkResult {
	hermesDetected := isAgentDetected(cfg, agentHermes)

	if !hermesDetected {
		return checkResult{"hermes direct mirrors", checkStatusPass, agentHermes + " not detected, skipped"}
	}

	hermesSkillsDir := filepath.Join(home, ".hermes", "skills")
	hermesSkills, err := collectDirectSkillNames(hermesSkillsDir)
	if err != nil {
		return checkResult{"hermes direct mirrors", checkStatusPass, "hermes skills dir unreadable, skipped"}
	}

	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"hermes direct mirrors", checkStatusFail, fmt.Sprintf("cannot read skills/: %s", err)}
	}

	var mirrors []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillMD := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if !hasFile(skillMD) {
			continue
		}
		fm, err := parseSkillFrontmatter(skillMD)
		if err != nil || fm.Name == "" {
			continue
		}
		if hermesPath, ok := hermesSkills[fm.Name]; ok {
			mirrors = append(mirrors, fmt.Sprintf("%s has direct hermes mirror %s at %s", entry.Name(), fm.Name, hermesPath))
		}
	}

	if len(mirrors) > 0 {
		return checkResult{"hermes direct mirrors", checkStatusWarn, strings.Join(mirrors, "; ")}
	}
	return checkResult{"hermes direct mirrors", checkStatusPass, "no direct mirrors"}
}

func collectDirectSkillNames(root string) (map[string]string, error) {
	names := make(map[string]string)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillMD := filepath.Join(root, entry.Name(), "SKILL.md")
		if !hasFile(skillMD) {
			continue
		}
		fm, err := parseSkillFrontmatter(skillMD)
		if err != nil || strings.TrimSpace(fm.Name) == "" {
			continue
		}
		names[fm.Name] = filepath.Join(entry.Name(), "SKILL.md")
	}
	return names, nil
}

type agnixSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

type agnixDiagnostic struct {
	Level   string `json:"level"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type agnixReport struct {
	Summary     agnixSummary      `json:"summary"`
	Diagnostics []agnixDiagnostic `json:"diagnostics"`
}

func checkAgnix(repoRoot string) checkResult {
	if _, err := exec.LookPath("agnix"); err != nil {
		return checkResult{"agnix", checkStatusFail, "agnix not installed; run npm install -g agnix"}
	}
	out, err := runAgnix(repoRoot)
	report, parseErr := parseAgnixReport(out)
	if parseErr != nil {
		return checkResult{"agnix", checkStatusFail, fmt.Sprintf("could not parse agnix output: %s", parseErr)}
	}
	if err != nil || report.Summary.Errors > 0 {
		return checkResult{"agnix", checkStatusFail, agnixDetail(report)}
	}
	return checkResult{"agnix", checkStatusPass, agnixDetail(report)}
}

func runAgnix(repoRoot string) ([]byte, error) {
	cmd := exec.Command("agnix", "--format", "json", ".")
	cmd.Dir = repoRoot
	return cmd.Output()
}

func parseAgnixReport(data []byte) (agnixReport, error) {
	var report agnixReport
	if err := json.Unmarshal(data, &report); err != nil {
		return agnixReport{}, err
	}
	return report, nil
}

func agnixDetail(report agnixReport) string {
	detail := fmt.Sprintf("%d errors, %d warnings, %d info", report.Summary.Errors, report.Summary.Warnings, report.Summary.Info)
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Level != "error" {
			continue
		}
		location := diagnostic.File
		if diagnostic.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, diagnostic.Line)
		}
		return fmt.Sprintf("%s; first error: %s: %s", detail, location, diagnostic.Message)
	}
	return detail
}

func checkAgentsMDSize(repoRoot string) checkResult {
	const limit = 8192
	path := filepath.Join(repoRoot, "AGENTS.md")
	info, err := os.Stat(path)
	if err != nil {
		return checkResult{"AGENTS.md size", checkStatusFail, "AGENTS.md not found"}
	}
	size := info.Size()
	sizeKB := float64(size) / 1024.0
	if size > limit {
		return checkResult{"AGENTS.md size", checkStatusWarn, fmt.Sprintf("%.1fKB / 8.0KB limit", sizeKB)}
	}
	return checkResult{"AGENTS.md size", checkStatusPass, fmt.Sprintf("%.1fKB / 8.0KB limit", sizeKB)}
}

func checkREADMESkillList(repoRoot string) checkResult {
	readmePath := filepath.Join(repoRoot, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return checkResult{"README skill list", checkStatusFail, "README.md not found"}
	}
	readmeContent := string(data)

	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"README skill list", checkStatusFail, fmt.Sprintf("cannot read skills/: %s", err)}
	}

	var missing []string
	total := 0
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !hasFile(filepath.Join(skillsDir, entry.Name(), "SKILL.md")) {
			continue
		}
		total++
		if !strings.Contains(readmeContent, "`"+entry.Name()+"`") {
			missing = append(missing, entry.Name())
		}
	}

	if len(missing) > 0 {
		return checkResult{"README skill list", checkStatusWarn, fmt.Sprintf("not listed: %s", strings.Join(missing, ", "))}
	}
	return checkResult{"README skill list", checkStatusPass, fmt.Sprintf("all %d skills listed", total)}
}

func checkMemsearchIndex(home string) checkResult {
	if _, err := exec.LookPath("memsearch"); err == nil {
		out, err := exec.Command("memsearch", "stats", "--collection", "ai").CombinedOutput()
		if err == nil {
			if chunks, ok := parseMemsearchChunks(string(out)); ok {
				if chunks == 0 {
					return checkResult{"memsearch index", checkStatusWarn, "collection ai has 0 chunks"}
				}
				return checkResult{"memsearch index", checkStatusPass, fmt.Sprintf("collection ai has %d chunks", chunks)}
			}
		}
	}

	stateDir := filepath.Join(home, ".memsearch", "state")
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return checkResult{"memsearch index", checkStatusWarn, "state dir missing or unreadable"}
	}
	count := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			count++
		}
	}
	if count == 0 {
		return checkResult{"memsearch index", checkStatusWarn, "state dir empty, index never built"}
	}
	return checkResult{"memsearch index", checkStatusPass, fmt.Sprintf("state dir has %d files", count)}
}

func parseMemsearchChunks(statsOutput string) (int, bool) {
	re := regexp.MustCompile(`(?m)Total indexed chunks:\s*([0-9]+)[.,]?\s*$`)
	match := re.FindStringSubmatch(statsOutput)
	if len(match) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

func checkHermesHooks(home string, cfg config) checkResult {
	if !isAgentDetected(cfg, agentHermes) {
		return checkResult{"hermes hooks", checkStatusPass, agentHermes + " not detected, skipped"}
	}

	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return checkResult{"hermes hooks", checkStatusWarn, "config.yaml not readable"}
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return checkResult{"hermes hooks", checkStatusWarn, "config.yaml parse error"}
	}

	hooksRaw, ok := raw["hooks"]
	if !ok {
		return checkResult{"hermes hooks", checkStatusWarn, "no hooks section configured"}
	}

	// Count hooks if it's a list or map
	count := 0
	switch v := hooksRaw.(type) {
	case []interface{}:
		count = len(v)
	case map[string]interface{}:
		count = len(v)
	default:
		if hooksRaw != nil {
			count = 1
		}
	}

	if count == 0 {
		return checkResult{"hermes hooks", checkStatusWarn, "hooks section present but empty"}
	}
	return checkResult{"hermes hooks", checkStatusPass, fmt.Sprintf("%d hook configured", count)}
}

func checkExternalSkillSources(cfg config, home string) checkResult {
	if len(cfg.ExternalSkills) == 0 {
		return checkResult{"external skills", checkStatusPass, "none configured"}
	}
	cacheRoot := externalCacheDir(home)
	var issues []string
	valid := 0
	for _, src := range cfg.ExternalSkills {
		name := repoName(src.URL)
		cachePath := filepath.Join(cacheRoot, name)
		if !hasDir(filepath.Join(cachePath, ".git")) {
			issues = append(issues, fmt.Sprintf("%s not cloned", name))
			continue
		}
		skillBase := filepath.Join(cachePath, src.SkillDir)
		if !hasDir(skillBase) {
			issues = append(issues, fmt.Sprintf("%s missing skill_dir %s", name, src.SkillDir))
			continue
		}
		hasSkill := hasFile(filepath.Join(skillBase, "SKILL.md"))
		if !hasSkill {
			entries, err := os.ReadDir(skillBase)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() && hasFile(filepath.Join(skillBase, e.Name(), "SKILL.md")) {
						hasSkill = true
						break
					}
				}
			}
		}
		if !hasSkill {
			issues = append(issues, fmt.Sprintf("%s has no SKILL.md in %s", name, src.SkillDir))
			continue
		}
		valid++
	}
	if len(issues) > 0 {
		return checkResult{"external skills", checkStatusWarn, strings.Join(issues, "; ")}
	}
	return checkResult{"external skills", checkStatusPass, fmt.Sprintf("%d sources valid", valid)}
}
