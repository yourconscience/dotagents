package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
)

func TestCommittedCodexMarketplaceContract(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	var marketplace struct {
		Plugins []struct {
			Name   string `json:"name"`
			Source struct {
				Kind string `json:"source"`
				Path string `json:"path"`
			} `json:"source"`
			Policy struct {
				Installation   string `json:"installation"`
				Authentication string `json:"authentication"`
			} `json:"policy"`
			Category string `json:"category"`
		} `json:"plugins"`
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &marketplace); err != nil {
		t.Fatalf("parse marketplace: %v", err)
	}
	pluginIndex := -1
	for i, plugin := range marketplace.Plugins {
		if plugin.Name == "dotagents" {
			pluginIndex = i
			break
		}
	}
	if pluginIndex == -1 {
		t.Fatal("dotagents marketplace plugin is missing")
	}
	plugin := marketplace.Plugins[pluginIndex]
	if plugin.Source.Kind != "local" || plugin.Source.Path != "./skills" {
		t.Fatalf("dotagents marketplace source = %q %q, want local ./skills", plugin.Source.Kind, plugin.Source.Path)
	}
	if plugin.Policy.Installation != "AVAILABLE" || plugin.Policy.Authentication != "ON_INSTALL" {
		t.Fatalf("dotagents marketplace policy = installation %q authentication %q, want AVAILABLE and ON_INSTALL", plugin.Policy.Installation, plugin.Policy.Authentication)
	}
	if plugin.Category != "Productivity" {
		t.Fatalf("dotagents marketplace category = %q, want Productivity", plugin.Category)
	}

	var manifest struct {
		Name   string `json:"name"`
		Skills string `json:"skills"`
	}
	data, err = os.ReadFile(filepath.Join(repoRoot, "skills", ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse Codex manifest: %v", err)
	}
	if manifest.Name != "dotagents" || manifest.Skills != "./" {
		t.Fatalf("Codex manifest = name %q skills %q, want dotagents and ./", manifest.Name, manifest.Skills)
	}

	for _, removed := range []string{
		filepath.Join(repoRoot, "plugins", "dotagents"),
		filepath.Join(repoRoot, ".agents", "plugins", "dotagents"),
	} {
		if _, err := os.Lstat(removed); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("removed Codex mirror still exists at %s (stat error %v)", removed, err)
		}
	}
}

func TestSyncRenderDoesNotCreateCodexMirror(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("DOTAGENTS_ROOT", repoRoot)
	t.Setenv("HOME", home)

	writeSyncTestFile(t, filepath.Join(repoRoot, "dotagents.yaml"), []byte(`version: 1
agents:
  - name: omp
    enabled: true
    skill_root: ~/.omp/agent/skills
    agent_root: ~/.omp/agent/agents
`))
	writeSyncTestFile(t, filepath.Join(repoRoot, "agents", "subagents.yaml"), []byte(`version: 1
agents:
  - name: reviewer
    description: Reviews changes
    instructions: Review the change.
`))
	writeSyncTestFile(t, filepath.Join(repoRoot, "skills", "sample", "SKILL.md"), []byte("---\nname: sample\n---\n"))
	writeSyncTestFile(t, filepath.Join(repoRoot, "README.md"), []byte("# Skills\n\n"+readmeSkillsBeginMarker+"\n0 skills ship with this repo:\n\n``\n"+readmeSkillsEndMarker+"\n"))

	if err := run([]string{"sync", "render"}); err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{
		filepath.Join(repoRoot, "plugins", "dotagents"),
		filepath.Join(repoRoot, ".agents", "plugins", "dotagents"),
	} {
		if _, err := os.Lstat(removed); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("sync render recreated Codex mirror at %s (stat error %v)", removed, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "1 skills ship with this repo:\n\n`sample`"; !bytes.Contains(data, []byte(want)) {
		t.Fatalf("sync render did not complete canonical outputs:\n%s", data)
	}
}

func TestDotagentsBinaryDoesNotCarryCodexMirrorCopyDependency(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("Go build information unavailable")
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/otiai10/copy" {
			t.Fatalf("obsolete Codex mirror dependency remains in binary: %s", dep.Path)
		}
	}
}
