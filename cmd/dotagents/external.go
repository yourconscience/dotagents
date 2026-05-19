package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func externalCacheDir(home string) string {
	return filepath.Join(home, ".agents", "external")
}

func repoName(url string) string {
	u := strings.TrimSpace(url)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	if idx := strings.LastIndex(u, "/"); idx >= 0 {
		u = u[idx+1:]
	}
	return strings.TrimSpace(u)
}

func syncExternalRepos(sources []externalSkillSource, home string) error {
	if len(sources) == 0 {
		return nil
	}
	cacheRoot := externalCacheDir(home)
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cacheRoot, err)
	}
	for _, src := range sources {
		name := repoName(src.URL)
		cachePath := filepath.Join(cacheRoot, name)
		if hasDir(filepath.Join(cachePath, ".git")) {
			setURL := exec.Command("git", "-C", cachePath, "remote", "set-url", "origin", src.URL)
			if err := setURL.Run(); err != nil {
				return fmt.Errorf("update remote URL for %s: %w", name, err)
			}
			if err := gitFetchReset(cachePath, src.Branch); err != nil {
				return fmt.Errorf("update external %s: %w", name, err)
			}
		} else {
			if err := gitClone(src.URL, src.Branch, cachePath); err != nil {
				return fmt.Errorf("clone external %s: %w", name, err)
			}
		}
	}
	return nil
}

func gitClone(url string, branch string, dest string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitFetchReset(repoPath string, branch string) error {
	fetch := exec.Command("git", "-C", repoPath, "fetch", "origin", branch)
	fetch.Stdout = os.Stdout
	fetch.Stderr = os.Stderr
	if err := fetch.Run(); err != nil {
		return err
	}
	reset := exec.Command("git", "-C", repoPath, "reset", "--hard", "origin/"+branch)
	reset.Stdout = os.Stdout
	reset.Stderr = os.Stderr
	return reset.Run()
}

func discoverExternalSkills(sources []externalSkillSource, home string) (map[string]string, error) {
	result := make(map[string]string)
	cacheRoot := externalCacheDir(home)
	for _, src := range sources {
		name := repoName(src.URL)
		cachePath := filepath.Join(cacheRoot, name)
		skillBase := filepath.Join(cachePath, src.SkillDir)

		if !hasDir(skillBase) {
			if !hasDir(filepath.Join(cachePath, ".git")) {
				return nil, fmt.Errorf("external source %s not cloned; run dotagents sync", src.URL)
			}
			return nil, fmt.Errorf("skill_dir %q not found in cloned external source %s", src.SkillDir, src.URL)
		}

		if hasFile(filepath.Join(skillBase, "SKILL.md")) {
			if _, exists := result[name]; exists {
				return nil, fmt.Errorf("external skill %q from %s collides with a skill already discovered from another source", name, src.URL)
			}
			result[name] = skillBase
			continue
		}

		entries, err := os.ReadDir(skillBase)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", skillBase, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			skillPath := filepath.Join(skillBase, entry.Name())
			if !hasFile(filepath.Join(skillPath, "SKILL.md")) {
				continue
			}
			if _, exists := result[entry.Name()]; exists {
				return nil, fmt.Errorf("external skill %q from %s collides with a skill already discovered from another source", entry.Name(), src.URL)
			}
			result[entry.Name()] = skillPath
		}
	}
	return result, nil
}

func externalSkillCommit(cachePath string) string {
	out, err := exec.Command("git", "-C", cachePath, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func isExternalSkillLink(linkPath string, rawTarget string, home string) bool {
	extRoot := externalCacheDir(home)
	targetAbs := absoluteTarget(linkPath, rawTarget)
	if targetAbs == extRoot || strings.HasPrefix(targetAbs, extRoot+string(os.PathSeparator)) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		return false
	}
	return resolved == extRoot || strings.HasPrefix(resolved, extRoot+string(os.PathSeparator))
}
