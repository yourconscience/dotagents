package main

// Validates skills/*/SKILL.md against the Agent Skills standard
// (https://agentskills.io/specification).

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillSpecNameMaxLen          = 64
	skillSpecDescriptionMaxLen   = 1024
	skillSpecCompatibilityMaxLen = 500
)

// Lowercase letters, digits, and hyphens; no leading, trailing, or
// consecutive hyphens.
var skillSpecNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var skillSpecKnownFields = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"compatibility": true,
	"metadata":      true,
	"allowed-tools": true,
}

func checkSkillSpec(repoRoot string) checkResult {
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return checkResult{"skill spec", checkStatusWarn, fmt.Sprintf("cannot read skills/: %s", err)}
	}

	var violations []string
	var infos []string
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
		fm, err := parseSkillSpecFrontmatter(skillMD)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		for _, problem := range validateSkillSpecFields(fm, entry.Name()) {
			violations = append(violations, fmt.Sprintf("%s: %s", entry.Name(), problem))
		}
		if unknown := unknownSkillSpecFields(fm); len(unknown) > 0 {
			infos = append(infos, fmt.Sprintf("%s: unknown fields %s", entry.Name(), strings.Join(unknown, ", ")))
		}
	}

	if len(violations) > 0 {
		return checkResult{"skill spec", checkStatusWarn, strings.Join(violations, "; ")}
	}
	detail := fmt.Sprintf("%d skills conform to agentskills.io spec", total)
	if len(infos) > 0 {
		detail += "; info: " + strings.Join(infos, "; ")
	}
	return checkResult{"skill spec", checkStatusPass, detail}
}

func validateSkillSpecFields(fm map[string]interface{}, dirName string) []string {
	var problems []string

	name, ok := fm["name"].(string)
	switch {
	case fm["name"] == nil:
		problems = append(problems, "missing required field name")
	case !ok:
		problems = append(problems, "name must be a string")
	default:
		if len(name) > skillSpecNameMaxLen {
			problems = append(problems, fmt.Sprintf("name exceeds %d characters", skillSpecNameMaxLen))
		}
		if !skillSpecNamePattern.MatchString(name) {
			problems = append(problems, fmt.Sprintf("name %q must be lowercase letters, digits, and hyphens with no leading/trailing/consecutive hyphens", name))
		} else if name != dirName {
			problems = append(problems, fmt.Sprintf("name %q does not match directory %q", name, dirName))
		}
	}

	description, ok := fm["description"].(string)
	switch {
	case fm["description"] == nil:
		problems = append(problems, "missing required field description")
	case !ok:
		problems = append(problems, "description must be a string")
	case description == "":
		problems = append(problems, "description must not be empty")
	case len(description) > skillSpecDescriptionMaxLen:
		problems = append(problems, fmt.Sprintf("description exceeds %d characters", skillSpecDescriptionMaxLen))
	}

	if raw, present := fm["compatibility"]; present {
		compatibility, ok := raw.(string)
		switch {
		case !ok:
			problems = append(problems, "compatibility must be a string")
		case len(compatibility) > skillSpecCompatibilityMaxLen:
			problems = append(problems, fmt.Sprintf("compatibility exceeds %d characters", skillSpecCompatibilityMaxLen))
		}
	}

	return problems
}

func unknownSkillSpecFields(fm map[string]interface{}) []string {
	var unknown []string
	for field := range fm {
		if !skillSpecKnownFields[field] {
			unknown = append(unknown, field)
		}
	}
	sort.Strings(unknown)
	return unknown
}

func parseSkillSpecFrontmatter(path string) (map[string]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return nil, fmt.Errorf("no frontmatter")
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
		return nil, err
	}
	if !foundEnd {
		return nil, fmt.Errorf("frontmatter not terminated")
	}

	fm := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &fm); err != nil {
		return nil, fmt.Errorf("frontmatter parse error: %v", err)
	}
	return fm, nil
}
