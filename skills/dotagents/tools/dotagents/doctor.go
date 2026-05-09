package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	agentClaudeCode = "claude-code"
	agentCodex      = "codex"
	agentDroid      = "droid"
	agentHermes     = "hermes"
)

type checkResult struct {
	name   string
	status string // "pass", "warn", "fail"
	detail string
}

func runDoctor(opts runOptions) error {
	repoRoot, home, cfg, _, err := loadContext(opts)
	if err != nil {
		return err
	}

	fmt.Println("dotagents doctor")
	fmt.Printf("repo: %s\n\n", repoRoot)

	var results []checkResult

	results = append(results, checkSkillFrontmatter(repoRoot))
	results = append(results, checkAgentRoles(repoRoot))
	results = append(results, checkSkillNameCollisions(repoRoot, home, cfg))
	results = append(results, checkAgentsMDSize(repoRoot))
	results = append(results, checkREADMESkillList(repoRoot))
	results = append(results, checkMemsearchIndex(home))
	results = append(results, checkHermesHooks(home, cfg))

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
		case "pass":
			passed++
		case "warn":
			warned++
		case "fail":
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
		return checkResult{"agent roles", "fail", err.Error()}
	}
	if len(roles) == 0 {
		return checkResult{"agent roles", "warn", "no agents/*.yaml roles found"}
	}
	return checkResult{"agent roles", "pass", fmt.Sprintf("%d roles valid", len(roles))}
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func checkSkillFrontmatter(repoRoot string) checkResult {
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"skill frontmatter", "fail", fmt.Sprintf("cannot read skills/: %s", err)}
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
		return checkResult{"skill frontmatter", "fail", fmt.Sprintf("%s missing name/description", strings.Join(invalid, ", "))}
	}
	return checkResult{"skill frontmatter", "pass", fmt.Sprintf("%d skills valid", total)}
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

func checkSkillNameCollisions(repoRoot string, home string, cfg config) checkResult {
	hermesDetected := isAgentDetected(cfg, agentHermes)

	if !hermesDetected {
		return checkResult{"skill name collisions", "pass", agentHermes + " not detected, skipped"}
	}

	hermesSkillsDir := filepath.Join(home, ".hermes", "skills")
	hermesEntries, err := os.ReadDir(hermesSkillsDir)
	if err != nil {
		return checkResult{"skill name collisions", "pass", "hermes skills dir unreadable, skipped"}
	}

	hermesBuiltins := make(map[string]struct{})
	for _, e := range hermesEntries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			hermesBuiltins[e.Name()] = struct{}{}
		}
	}

	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"skill name collisions", "fail", fmt.Sprintf("cannot read skills/: %s", err)}
	}

	var collisions []string
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
		if _, ok := hermesBuiltins[fm.Name]; ok {
			collisions = append(collisions, fmt.Sprintf("%s collides with hermes builtin %s", entry.Name(), fm.Name))
		}
	}

	if len(collisions) > 0 {
		return checkResult{"skill name collisions", "warn", strings.Join(collisions, "; ")}
	}
	return checkResult{"skill name collisions", "pass", "no collisions"}
}

func checkAgentsMDSize(repoRoot string) checkResult {
	const limit = 8192
	path := filepath.Join(repoRoot, "AGENTS.md")
	info, err := os.Stat(path)
	if err != nil {
		return checkResult{"AGENTS.md size", "fail", "AGENTS.md not found"}
	}
	size := info.Size()
	sizeKB := float64(size) / 1024.0
	if size > limit {
		return checkResult{"AGENTS.md size", "warn", fmt.Sprintf("%.1fKB / 8.0KB limit", sizeKB)}
	}
	return checkResult{"AGENTS.md size", "pass", fmt.Sprintf("%.1fKB / 8.0KB limit", sizeKB)}
}

func checkREADMESkillList(repoRoot string) checkResult {
	readmePath := filepath.Join(repoRoot, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return checkResult{"README skill list", "fail", "README.md not found"}
	}
	readmeContent := string(data)

	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"README skill list", "fail", fmt.Sprintf("cannot read skills/: %s", err)}
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
		return checkResult{"README skill list", "warn", fmt.Sprintf("not listed: %s", strings.Join(missing, ", "))}
	}
	return checkResult{"README skill list", "pass", fmt.Sprintf("all %d skills listed", total)}
}

func checkMemsearchIndex(home string) checkResult {
	if _, err := exec.LookPath("memsearch"); err == nil {
		out, err := exec.Command("memsearch", "stats", "--collection", "ai").CombinedOutput()
		if err != nil {
			return checkResult{"memsearch index", "warn", "memsearch stats failed"}
		}
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			value := strings.TrimRight(fields[len(fields)-1], ".,")
			if count, err := strconv.Atoi(value); err == nil {
				if count == 0 {
					return checkResult{"memsearch index", "warn", "collection ai has 0 chunks"}
				}
				return checkResult{"memsearch index", "pass", fmt.Sprintf("collection ai has %d chunks", count)}
			}
		}
		return checkResult{"memsearch index", "warn", "could not parse memsearch stats"}
	}

	stateDir := filepath.Join(home, ".memsearch", "state")
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return checkResult{"memsearch index", "warn", "state dir missing or unreadable"}
	}
	count := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			count++
		}
	}
	if count == 0 {
		return checkResult{"memsearch index", "warn", "state dir empty, index never built"}
	}
	return checkResult{"memsearch index", "pass", fmt.Sprintf("state dir has %d files", count)}
}

func checkHermesHooks(home string, cfg config) checkResult {
	if !isAgentDetected(cfg, agentHermes) {
		return checkResult{"hermes hooks", "pass", agentHermes + " not detected, skipped"}
	}

	configPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return checkResult{"hermes hooks", "warn", "config.yaml not readable"}
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return checkResult{"hermes hooks", "warn", "config.yaml parse error"}
	}

	hooksRaw, ok := raw["hooks"]
	if !ok {
		return checkResult{"hermes hooks", "warn", "no hooks section configured"}
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
		return checkResult{"hermes hooks", "warn", "hooks section present but empty"}
	}
	return checkResult{"hermes hooks", "pass", fmt.Sprintf("%d hook configured", count)}
}
