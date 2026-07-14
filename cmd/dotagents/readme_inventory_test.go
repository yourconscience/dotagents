package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeREADMEInventoryFixture(t *testing.T, repoRoot string) {
	t.Helper()
	for _, name := range []string{"zeta", "alpha", "grill-me", "wayfinder"} {
		writeSyncTestFile(t, filepath.Join(repoRoot, "skills", name, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"))
	}
	writeSyncTestFile(t, filepath.Join(repoRoot, "skills", "not-a-skill", "README.txt"), []byte("ignored\n"))
	writeSyncTestFile(t, filepath.Join(repoRoot, "skills", ".codex-plugin", "SKILL.md"), []byte("---\nname: should-not-ship\n---\n"))
	writeSyncTestFile(t, filepath.Join(repoRoot, "README.md"), []byte("# Before\n\n"+readmeSkillsBeginMarker+"\n0 skills ship with this repo:\n\n``\n"+readmeSkillsEndMarker+"\n\n# After\n"))
}

func TestRenderREADMESkillsIsExactIdempotentAndExcludesAliasAndCodexMetadata(t *testing.T) {
	repoRoot := t.TempDir()
	writeREADMEInventoryFixture(t, repoRoot)

	if err := renderREADMESkills(repoRoot); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "# Before\n\n" +
		"<!-- BEGIN GENERATED SKILLS -->\n" +
		"3 skills ship with this repo:\n\n" +
		"`alpha` `wayfinder` `zeta`\n" +
		"<!-- END GENERATED SKILLS -->\n\n" +
		"# After\n"
	if string(first) != want {
		t.Fatalf("rendered README =\n%s\nwant =\n%s", first, want)
	}

	if err := renderREADMESkills(repoRoot); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Fatalf("second render changed README:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestDoctorDetectsREADMECountAndListDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) string
	}{
		{
			name: "count drift",
			mutate: func(content string) string {
				return strings.Replace(content, "3 skills ship with this repo:", "4 skills ship with this repo:", 1)
			},
		},
		{
			name: "list drift",
			mutate: func(content string) string {
				return strings.Replace(content, "`alpha` `wayfinder` `zeta`", "`alpha` `missing` `zeta`", 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			writeREADMEInventoryFixture(t, repoRoot)
			if err := renderREADMESkills(repoRoot); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repoRoot, "README.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeSyncTestFile(t, path, []byte(tc.mutate(string(data))))

			result := checkREADMESkillInventory(repoRoot)
			if result.status != checkStatusFail {
				t.Fatalf("doctor status = %q (%s), want fail", result.status, result.detail)
			}
			if result.detail != "generated block is stale; run: dotagents sync render" {
				t.Fatalf("doctor detail = %q", result.detail)
			}
		})
	}
}

func TestDoctorAcceptsOnlyExactREADMEInventory(t *testing.T) {
	repoRoot := t.TempDir()
	writeREADMEInventoryFixture(t, repoRoot)
	if err := renderREADMESkills(repoRoot); err != nil {
		t.Fatal(err)
	}

	result := checkREADMESkillInventory(repoRoot)
	if result.status != checkStatusPass || result.detail != "exact inventory of 3 skills" {
		t.Fatalf("doctor result = %s (%s), want exact three-skill inventory", result.status, result.detail)
	}
}

func TestRenderREADMESkillsRejectsAmbiguousMarkersWithoutChangingFile(t *testing.T) {
	repoRoot := t.TempDir()
	writeSyncTestFile(t, filepath.Join(repoRoot, "skills", "alpha", "SKILL.md"), []byte("---\nname: alpha\n---\n"))
	original := "# README\n" + readmeSkillsBeginMarker + "\n" + readmeSkillsBeginMarker + "\n" + readmeSkillsEndMarker + "\n"
	path := filepath.Join(repoRoot, "README.md")
	writeSyncTestFile(t, path, []byte(original))

	err := renderREADMESkills(repoRoot)
	if err == nil || err.Error() != "README skills markers must each appear exactly once" {
		t.Fatalf("marker error = %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != original {
		t.Fatalf("renderer changed README after marker error:\n%s", data)
	}
}

func TestCommittedREADMESkillInventoryIsFreshAndMatchesLaunchSet(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	block, count, err := expectedREADMESkillsBlock(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if count != 20 {
		t.Fatalf("committed public skill count = %d, want 20", count)
	}
	if !strings.Contains(block, "`wayfinder`") {
		t.Fatalf("committed public inventory omits wayfinder:\n%s", block)
	}
	if strings.Contains(block, "`grill-me`") {
		t.Fatalf("committed public inventory counts grill-me alias:\n%s", block)
	}
	result := checkREADMESkillInventory(repoRoot)
	if result.status != checkStatusPass {
		t.Fatalf("committed README inventory = %s (%s)", result.status, result.detail)
	}
}
