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

// syncExternalRepos clones or updates external skill sources. Sources pinned in
// dotagents.lock are checked out at their locked commit; unpinned sources track
// the latest origin/<branch> and get recorded into the lock afterwards.
func syncExternalRepos(sources []externalSkillSource, home string, repoRoot string) error {
	if len(sources) == 0 {
		return nil
	}
	lock, err := readLockFile(repoRoot)
	if err != nil {
		return err
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
		} else {
			if err := gitClone(src.URL, src.Branch, cachePath); err != nil {
				return fmt.Errorf("clone external %s: %w", name, err)
			}
		}
		if pin := lockEntryFor(lock, src); pin != nil {
			if err := gitCheckoutCommit(cachePath, pin.Commit, src.Branch); err != nil {
				return fmt.Errorf("pin external %s to %s: %w", name, pin.Commit, err)
			}
		} else if err := gitFetchReset(cachePath, src.Branch); err != nil {
			return fmt.Errorf("update external %s: %w", name, err)
		}
	}
	return writeLockIfChanged(sources, home, repoRoot, lock)
}

// updateExternalRepos moves the named sources (or all when names is empty) to
// the latest origin/<branch> and rewrites their lock entries.
func updateExternalRepos(sources []externalSkillSource, home string, repoRoot string, names []string) error {
	selected, err := selectExternalSources(sources, names)
	if err != nil {
		return err
	}
	lock, err := readLockFile(repoRoot)
	if err != nil {
		return err
	}
	cacheRoot := externalCacheDir(home)
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cacheRoot, err)
	}
	for _, src := range selected {
		name := repoName(src.URL)
		cachePath := filepath.Join(cacheRoot, name)
		if !hasDir(filepath.Join(cachePath, ".git")) {
			if err := gitClone(src.URL, src.Branch, cachePath); err != nil {
				return fmt.Errorf("clone external %s: %w", name, err)
			}
		} else if err := gitFetchReset(cachePath, src.Branch); err != nil {
			return fmt.Errorf("update external %s: %w", name, err)
		}
		fmt.Printf("updated %s -> %s\n", name, externalSkillCommit(cachePath))
	}
	return writeLockIfChanged(sources, home, repoRoot, lock)
}

func selectExternalSources(sources []externalSkillSource, names []string) ([]externalSkillSource, error) {
	if len(names) == 0 {
		return sources, nil
	}
	index := make(map[string]externalSkillSource, len(sources))
	for _, src := range sources {
		index[repoName(src.URL)] = src
	}
	var selected []externalSkillSource
	for _, name := range names {
		src, ok := index[strings.TrimSpace(name)]
		if !ok {
			return nil, fmt.Errorf("unknown external source %q; configured: %s", name, strings.Join(externalSourceNames(sources), ", "))
		}
		selected = append(selected, src)
	}
	return selected, nil
}

func externalSourceNames(sources []externalSkillSource) []string {
	var names []string
	for _, src := range sources {
		names = append(names, repoName(src.URL))
	}
	return names
}

func writeLockIfChanged(sources []externalSkillSource, home string, repoRoot string, lock lockFile) error {
	entries := rebuildLockEntries(sources, home)
	if lockEntriesEqual(lock.ExternalSkills, entries) {
		return nil
	}
	lock.ExternalSkills = entries
	return writeLockFile(repoRoot, lock)
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

// gitCheckoutCommit hard-resets the cache to a pinned commit, fetching it when
// the shallow clone does not contain it yet.
func gitCheckoutCommit(repoPath string, commit string, branch string) error {
	if externalSkillCommitFull(repoPath) == commit {
		return nil
	}
	if !gitHasCommit(repoPath, commit) {
		// Hosts like GitHub allow shallow fetches of an exact commit.
		fetchSHA := exec.Command("git", "-C", repoPath, "fetch", "--depth", "1", "origin", commit)
		fetchSHA.Stdout = os.Stdout
		fetchSHA.Stderr = os.Stderr
		if err := fetchSHA.Run(); err != nil {
			// Fall back to unshallowing the branch history.
			fetchAll := exec.Command("git", "-C", repoPath, "fetch", "--unshallow", "origin", branch)
			fetchAll.Stdout = os.Stdout
			fetchAll.Stderr = os.Stderr
			if fallbackErr := fetchAll.Run(); fallbackErr != nil {
				return fmt.Errorf("fetch pinned commit: %w", err)
			}
		}
	}
	reset := exec.Command("git", "-C", repoPath, "reset", "--hard", commit)
	reset.Stdout = os.Stdout
	reset.Stderr = os.Stderr
	return reset.Run()
}

func gitHasCommit(repoPath string, commit string) bool {
	return exec.Command("git", "-C", repoPath, "cat-file", "-e", commit+"^{commit}").Run() == nil
}

func discoverExternalSkills(sources []externalSkillSource, home string) (map[string]string, error) {
	result := make(map[string]string)
	cacheRoot := externalCacheDir(home)
	for _, src := range sources {
		name := repoName(src.URL)
		cachePath := filepath.Join(cacheRoot, name)
		skillBase := filepath.Join(cachePath, src.SkillDir)

		if !hasDir(filepath.Join(cachePath, ".git")) {
			return nil, fmt.Errorf("external source %s not cloned; run dotagents sync", src.URL)
		}
		if !hasDir(skillBase) {
			return nil, fmt.Errorf("skill_dir %q not found in cloned external source %s", src.SkillDir, src.URL)
		}

		countBefore := len(result)
		allowed := skillAllowlist(src)

		if hasFile(filepath.Join(skillBase, "SKILL.md")) {
			if allowed != nil && !allowed[name] {
				return nil, fmt.Errorf("external source %s: skills allowlist does not match single skill %q", src.URL, name)
			}
			if _, exists := result[name]; exists {
				return nil, fmt.Errorf("external skill %q from %s collides with a skill already discovered from another source", name, src.URL)
			}
			result[name] = skillBase
		} else {
			entries, err := os.ReadDir(skillBase)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", skillBase, err)
			}
			found := make(map[string]bool)
			for _, entry := range entries {
				if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				skillPath := filepath.Join(skillBase, entry.Name())
				if !hasFile(filepath.Join(skillPath, "SKILL.md")) {
					continue
				}
				if allowed != nil && !allowed[entry.Name()] {
					continue
				}
				if _, exists := result[entry.Name()]; exists {
					return nil, fmt.Errorf("external skill %q from %s collides with a skill already discovered from another source", entry.Name(), src.URL)
				}
				result[entry.Name()] = skillPath
				found[entry.Name()] = true
			}
			for skill := range allowed {
				if !found[skill] {
					return nil, fmt.Errorf("external source %s: allowlisted skill %q not found in %q", src.URL, skill, src.SkillDir)
				}
			}
		}

		if len(result) == countBefore {
			return nil, fmt.Errorf("external source %s: no skills found in %q", src.URL, src.SkillDir)
		}
	}
	return result, nil
}

// skillAllowlist returns the configured skill name filter, or nil when the
// source exposes all skills.
func skillAllowlist(src externalSkillSource) map[string]bool {
	if len(src.Skills) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(src.Skills))
	for _, name := range src.Skills {
		name = strings.TrimSpace(name)
		if name != "" {
			allowed[name] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
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
