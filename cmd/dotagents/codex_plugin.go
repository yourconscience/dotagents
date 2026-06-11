package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// codexPluginRelDir is the whole-repo Codex plugin. Its skills/ subtree is a
// rendered copy of the repo's skills/ because Codex plugin installs ignore
// symlinked skills directories (verified against codex 0.136.0).
const codexPluginRelDir = "plugins/dotagents"

const codexMarketplaceRelPath = ".agents/plugins/marketplace.json"

// codexPluginSourceFiles lists the skill files to mirror, as paths relative to
// the repo root (slash-separated). Tracked and untracked-but-not-ignored files
// are included so gitignored build artifacts and local data stay out.
func codexPluginSourceFiles(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "skills")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files skills: %w", err)
	}
	var files []string
	for _, raw := range bytes.Split(out, []byte{0}) {
		name := string(raw)
		if name == "" {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	return files, nil
}

func codexPluginSkillsDir(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(codexPluginRelDir), "skills")
}

// renderCodexPluginSkills mirrors skills/ into the Codex plugin dir, removing
// files that no longer exist in the source tree.
func renderCodexPluginSkills(repoRoot string) error {
	files, err := codexPluginSourceFiles(repoRoot)
	if err != nil {
		return err
	}
	destRoot := codexPluginSkillsDir(repoRoot)

	expected := make(map[string]bool, len(files))
	written, unchanged := 0, 0
	for _, rel := range files {
		sub := strings.TrimPrefix(rel, "skills/")
		src := filepath.Join(repoRoot, filepath.FromSlash(rel))
		dest := filepath.Join(destRoot, filepath.FromSlash(sub))
		data, err := os.ReadFile(src)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // tracked but deleted in worktree; leave unexpected so the copy is pruned
			}
			return fmt.Errorf("read %s: %w", src, err)
		}
		expected[filepath.FromSlash(sub)] = true
		current, err := os.ReadFile(dest)
		if err == nil && bytes.Equal(current, data) {
			unchanged++
			continue
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read %s: %w", dest, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
		}
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("stat %s: %w", src, err)
		}
		if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		written++
	}

	removed := 0
	if err := filepath.WalkDir(destRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(destRoot, path)
		if err != nil {
			return err
		}
		if expected[rel] {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove stale %s: %w", path, err)
		}
		removed++
		return nil
	}); err != nil {
		return err
	}
	if err := removeEmptyDirs(destRoot); err != nil {
		return err
	}

	fmt.Printf("rendered %s/skills: %d written, %d unchanged, %d removed\n", codexPluginRelDir, written, unchanged, removed)
	return nil
}

func removeEmptyDirs(root string) error {
	var dirs []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	// Deepest first so emptied parents are removable in one pass.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkCodexPlugin(repoRoot string) checkResult {
	const name = "codex plugin"
	manifest := filepath.Join(repoRoot, filepath.FromSlash(codexPluginRelDir), ".codex-plugin", "plugin.json")
	if !hasFile(manifest) {
		return checkResult{name, checkStatusFail, fmt.Sprintf("missing %s/.codex-plugin/plugin.json", codexPluginRelDir)}
	}
	if !hasFile(filepath.Join(repoRoot, filepath.FromSlash(codexMarketplaceRelPath))) {
		return checkResult{name, checkStatusFail, "missing " + codexMarketplaceRelPath}
	}

	files, err := codexPluginSourceFiles(repoRoot)
	if err != nil {
		return checkResult{name, checkStatusFail, err.Error()}
	}
	destRoot := codexPluginSkillsDir(repoRoot)

	expected := make(map[string]bool, len(files))
	var stale []string
	fresh := 0
	for _, rel := range files {
		sub := filepath.FromSlash(strings.TrimPrefix(rel, "skills/"))
		expected[sub] = true
		src, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				delete(expected, sub)
				continue
			}
			return checkResult{name, checkStatusFail, err.Error()}
		}
		dest, err := os.ReadFile(filepath.Join(destRoot, sub))
		if err != nil || !bytes.Equal(src, dest) {
			stale = append(stale, sub)
			continue
		}
		fresh++
	}
	if err := filepath.WalkDir(destRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(destRoot, path)
		if err != nil {
			return err
		}
		if !expected[rel] {
			stale = append(stale, rel+" (extra)")
		}
		return nil
	}); err != nil {
		return checkResult{name, checkStatusFail, err.Error()}
	}

	if len(stale) > 0 {
		sort.Strings(stale)
		sample := stale
		if len(sample) > 5 {
			sample = sample[:5]
		}
		return checkResult{name, checkStatusFail, fmt.Sprintf("%d file(s) stale (%s); run: dotagents render", len(stale), strings.Join(sample, ", "))}
	}
	return checkResult{name, checkStatusPass, fmt.Sprintf("%d rendered skill files fresh in %s", fresh, codexPluginRelDir)}
}
