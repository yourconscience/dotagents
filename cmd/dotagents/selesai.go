package main

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// getSelesaiBundledSkills discovers Selesai's bundled skills by inspecting
// the installed npm package. Returns a set of bundled skill names to exclude
// from dotagents sync.
func getSelesaiBundledSkills() (map[string]struct{}, error) {
	// Find selesai executable
	selesaiPath, err := exec.LookPath("selesai")
	if err != nil {
		// Selesai not installed, return empty set
		return make(map[string]struct{}), nil
	}

	// Resolve symlink if the executable is a symlink (common with npm global installs)
	selesaiPath, err = filepath.EvalSymlinks(selesaiPath)
	if err != nil {
		return nil, err
	}

	// npm global installs typically have structure:
	// /path/to/npm/prefix/lib/node_modules/@selesai/code/bin/selesai.js
	// We need to find the package directory: /path/to/npm/prefix/lib/node_modules/@selesai/code
	packageDir := selesaiPath
	for {
		parent := filepath.Dir(packageDir)
		if parent == packageDir {
			// Reached root without finding package.json
			return nil, errors.New("could not find @selesai/code package directory")
		}
		packageJSON := filepath.Join(parent, "package.json")
		if hasFile(packageJSON) {
			packageDir = parent
			break
		}
		packageDir = parent
	}

	// Look for bundled skills in dist/skills/ or src/skills/
	bundled := make(map[string]struct{})
	for _, skillsDir := range []string{
		filepath.Join(packageDir, "dist", "skills"),
		filepath.Join(packageDir, "src", "skills"),
	} {
		entries, err := os.ReadDir(skillsDir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() && hasFile(filepath.Join(skillsDir, entry.Name(), "SKILL.md")) {
				bundled[entry.Name()] = struct{}{}
			}
		}
	}

	return bundled, nil
}

// filterSelesaiExpectedSkills removes bundled skills from the expected skills map
// for Selesai agent to avoid conflicts with Selesai's built-in skills.
func filterSelesaiExpectedSkills(expected map[string]string) (map[string]string, error) {
	bundled, err := getSelesaiBundledSkills()
	if err != nil {
		return nil, err
	}

	if len(bundled) == 0 {
		// No bundled skills found or Selesai not installed, sync all skills
		return expected, nil
	}

	filtered := make(map[string]string)
	for name, path := range expected {
		if _, isBundled := bundled[name]; !isBundled {
			filtered[name] = path
		}
	}

	return filtered, nil
}

// getSelesaiBundledSkillNames returns a sorted list of bundled skill names for display.
func getSelesaiBundledSkillNames() []string {
	bundled, err := getSelesaiBundledSkills()
	if err != nil || len(bundled) == 0 {
		return nil
	}
	names := make([]string, 0, len(bundled))
	for name := range bundled {
		names = append(names, name)
	}
	return sortedStrings(names)
}

func sortedStrings(s []string) []string {
	sorted := append([]string{}, s...)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}
