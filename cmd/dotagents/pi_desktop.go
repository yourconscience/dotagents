package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// piDesktopPluginManifest represents the manifest.json structure for a Pi Desktop plugin
type piDesktopPluginManifest struct {
	SchemaVersion int                           `json:"schemaVersion"`
	ID            string                        `json:"id"`
	Name          string                        `json:"name"`
	Version       string                        `json:"version"`
	Description   string                        `json:"description"`
	Author        string                        `json:"author"`
	Main          string                        `json:"main"`
	Contributes   piDesktopPluginContributions  `json:"contributes"`
}

type piDesktopPluginContributions struct {
	Skills []string `json:"skills"`
}

// generatePiDesktopPlugin creates a loadable Pi Desktop plugin directory
// from canonical dotagents skills and agent roles
func generatePiDesktopPlugin(repoRoot string, home string) (string, error) {
	pluginDir := filepath.Join(repoRoot, ".pi-desktop-plugin")

	// Clean existing plugin directory
	if err := os.RemoveAll(pluginDir); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove existing plugin dir: %w", err)
	}

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return "", fmt.Errorf("create plugin dir: %w", err)
	}

	// Create skills directory
	skillsDir := filepath.Join(pluginDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return "", fmt.Errorf("create skills dir: %w", err)
	}

	// Generate skill wrappers for canonical skills
	canonicalSkills := filepath.Join(repoRoot, "skills")
	skillPaths, err := generateSkillWrappers(canonicalSkills, skillsDir)
	if err != nil {
		return "", fmt.Errorf("generate skill wrappers: %w", err)
	}

	// Generate skill representations of agent roles
	canonicalRoles := filepath.Join(repoRoot, "agents")
	rolePaths, err := generateRoleSkills(canonicalRoles, skillsDir)
	if err != nil {
		return "", fmt.Errorf("generate role skills: %w", err)
	}

	allSkillPaths := append(skillPaths, rolePaths...)

	// Create manifest.json
	manifest := piDesktopPluginManifest{
		SchemaVersion: 1,
		ID:            "local.dotagents",
		Name:          "dotagents",
		Version:       "1.0.0",
		Description:   "Canonical dotagents skills and agent roles for Pi Desktop",
		Author:        "dotagents",
		Main:          "main.js",
		Contributes: piDesktopPluginContributions{
			Skills: allSkillPaths,
		},
	}

	manifestPath := filepath.Join(pluginDir, "manifest.json")
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	// Create minimal main.js
	mainJS := `// dotagents plugin entry point
// This plugin contributes canonical dotagents skills and agent roles to Pi Desktop
export function activate(context) {
  // Plugin is loaded and skills are contributed via manifest
}
`
	if err := os.WriteFile(filepath.Join(pluginDir, "main.js"), []byte(mainJS), 0o644); err != nil {
		return "", fmt.Errorf("write main.js: %w", err)
	}

	return pluginDir, nil
}

// generateSkillWrappers creates skill markdown files that reference canonical skills
func generateSkillWrappers(canonicalDir string, targetDir string) ([]string, error) {
	entries, err := os.ReadDir(canonicalDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read canonical skills: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		skillMD := filepath.Join(canonicalDir, entry.Name(), "SKILL.md")
		if !hasFile(skillMD) {
			continue
		}

		// Read canonical skill content
		content, err := os.ReadFile(skillMD)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", skillMD, err)
		}

		// Write to plugin skills directory
		targetPath := filepath.Join(targetDir, entry.Name()+".md")
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", targetPath, err)
		}

		paths = append(paths, "./skills/"+entry.Name()+".md")
	}

	return paths, nil
}

// generateRoleSkills creates skill representations of agent roles
func generateRoleSkills(canonicalDir string, targetDir string) ([]string, error) {
	entries, err := os.ReadDir(canonicalDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read canonical roles: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		rolePath := filepath.Join(canonicalDir, entry.Name())
		roleData, err := os.ReadFile(rolePath)
		if err != nil {
			continue
		}

		role, err := parseAgentRoleMarkdown(rolePath, roleData)
		if err != nil {
			continue // Skip invalid roles
		}

		// Create a skill markdown that describes the role
		skillContent := fmt.Sprintf(`---
name: %s (agent role)
description: %s
---

# %s

%s

## Agent Role Information

This is an agent role from dotagents. When you need to perform tasks that match this role's expertise, consider the guidance below.

**Tools**: %s

%s
`, role.Name, role.Description, role.Name, role.Description, strings.Join(role.Tools, ", "), role.Instructions)

		targetPath := filepath.Join(targetDir, "role-"+strings.TrimSuffix(entry.Name(), ".md")+".md")
		if err := os.WriteFile(targetPath, []byte(skillContent), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", targetPath, err)
		}

		paths = append(paths, "./skills/role-"+strings.TrimSuffix(entry.Name(), ".md")+".md")
	}

	return paths, nil
}

// applyPiDesktopPluginSync generates or updates the Pi Desktop plugin
func applyPiDesktopPluginSync(detected bool, repoRoot string, home string) error {
	if !detected {
		return nil
	}

	pluginDir, err := generatePiDesktopPlugin(repoRoot, home)
	if err != nil {
		return err
	}

	fmt.Printf("Pi Desktop plugin generated at %s\n", pluginDir)
	fmt.Printf("To load: Open Pi Desktop, use PluginScaffold or manually load this directory\n")

	return nil
}

// inspectPiDesktopPlugin reports the status of the generated plugin
func inspectPiDesktopPlugin(agent agentConfig, expected map[string]string, agentsSkillRoot string, cfg config, home string) (agentReport, error) {
	report := agentReport{
		Name:           agent.Name,
		ExpectedSkills: expected,
		Detected:       isDetected(agent),
	}
	
	if !report.Detected {
		return report, nil
	}
	
	// Check if plugin directory exists
	pluginDir := filepath.Join(agentsSkillRoot, "..", ".pi-desktop-plugin")
	info, err := os.Stat(pluginDir)
	if os.IsNotExist(err) {
		// Plugin needs to be generated
		for name := range expected {
			report.Missing = append(report.Missing, name)
			report.Adds = append(report.Adds, name)
		}
		sortReportLists(&report)
		report.Synced = false
		return report, nil
	}
	if err != nil {
		return report, fmt.Errorf("stat plugin dir: %w", err)
	}
	if !info.IsDir() {
		return report, fmt.Errorf("plugin path exists but is not a directory: %s", pluginDir)
	}
	
	// Plugin exists, consider all skills managed
	for name := range expected {
		report.Managed = append(report.Managed, name)
	}
	sortReportLists(&report)
	report.Synced = true
	
	return report, nil
}
