package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDiscoveredSkill(t *testing.T, root string, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireCollisionOrigins(t *testing.T, err error, origins ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected skill collision, got nil")
	}
	for _, origin := range origins {
		if !strings.Contains(err.Error(), origin) {
			t.Fatalf("collision error %q does not identify %q", err, origin)
		}
	}
}

func TestLocalAndExternalSkillCollisionIdentifiesBothOrigins(t *testing.T) {
	home := t.TempDir()
	repoRoot := t.TempDir()
	writeDiscoveredSkill(t, filepath.Join(repoRoot, "skills"), "shared")

	const externalURL = "https://github.com/example/catalog"
	externalRoot := filepath.Join(externalCacheDir(home), "catalog")
	makeGitDir(t, externalRoot)
	writeDiscoveredSkill(t, filepath.Join(externalRoot, "skills"), "shared")

	_, err := expectedSkills(repoRoot, home, config{ExternalSkills: []externalSkillSource{{
		URL: externalURL, SkillDir: "skills", Branch: "main",
	}}})
	requireCollisionOrigins(t, err, "local skills", `external Git "`+externalURL+`"`)
}
