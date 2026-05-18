package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPackageReferencesFromCommandUVX(t *testing.T) {
	refs := packageReferencesFromCommand(mcpTestUVXCommand, []string{"linkedin-scraper-mcp==4.12.0"}, "config")
	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one", refs)
	}
	if refs[0].Ecosystem != ecosystemPyPI || refs[0].Package != "linkedin-scraper-mcp" || refs[0].Version != "4.12.0" {
		t.Fatalf("unexpected ref: %#v", refs[0])
	}
}

func TestPackageReferencesFromCommandUVXFrom(t *testing.T) {
	refs := packageReferencesFromCommand(mcpTestUVXCommand, []string{"--from", "some-pkg@1.2.3", "some-command", "--arg"}, "config")
	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one", refs)
	}
	if refs[0].Package != "some-pkg" || refs[0].Version != "1.2.3" {
		t.Fatalf("unexpected ref: %#v", refs[0])
	}
}

func TestPackageReferencesFromCommandUVToolInstallWithFlags(t *testing.T) {
	refs := packageReferencesFromCommand("uv", []string{"tool", "install", "--upgrade", "ruff==0.15.12"}, "setup")
	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one", refs)
	}
	if refs[0].Ecosystem != ecosystemPyPI || refs[0].Package != "ruff" || refs[0].Version != "0.15.12" {
		t.Fatalf("unexpected ref: %#v", refs[0])
	}
}

func TestPackageReferencesFromCommandNPMInstallMultiple(t *testing.T) {
	refs := packageReferencesFromCommand("npm", []string{"install", "pkg-one@1.0.0", "@scope/pkg-two@2.0.0"}, "setup")
	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want two", refs)
	}
	if refs[0].Package != "pkg-one" || refs[1].Package != "@scope/pkg-two" {
		t.Fatalf("unexpected refs: %#v", refs)
	}
}

func TestPackageReferencesFromCommandPNPMDlxIgnoresCommandArgs(t *testing.T) {
	refs := packageReferencesFromCommand("pnpm", []string{"dlx", "create-vue@1.0.0", "my-app"}, "setup")
	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one", refs)
	}
	if refs[0].Package != "create-vue" {
		t.Fatalf("unexpected refs: %#v", refs)
	}
}

func TestPackageReferencesFromText(t *testing.T) {
	input := `
uv tool install rdt-cli
npm install -g @openai/codex
pnpm add @scope/pkg@1.2.3
`
	refs := packageReferencesFromText(input, "SKILL.md")
	if len(refs) != 3 {
		t.Fatalf("refs = %#v, want three", refs)
	}
	if refs[1].Package != "@openai/codex" || refs[1].Version != packageVersionLatest {
		t.Fatalf("unexpected npm ref: %#v", refs[1])
	}
	if refs[2].Package != "@scope/pkg" || refs[2].Version != "1.2.3" {
		t.Fatalf("unexpected pnpm ref: %#v", refs[2])
	}
}

func TestCheckExternalPackageAgeFailsFreshPackage(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "fresh", Enabled: true, Command: mcpTestUVXCommand, Args: []string{"fresh-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, false, now, func(ref packageReference) (packageRelease, error) {
		return packageRelease{Version: "1.0.0", Released: now.Add(-48 * time.Hour)}, nil
	})
	if got.status != checkStatusFail || !strings.Contains(got.detail, "newer than 7 days") {
		t.Fatalf("got %#v, want fresh package failure", got)
	}
}

func TestCheckExternalPackageAgePassesOldPackage(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "old", Enabled: true, Command: mcpTestUVXCommand, Args: []string{"old-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, false, now, func(ref packageReference) (packageRelease, error) {
		return packageRelease{Version: "1.0.0", Released: now.Add(-8 * 24 * time.Hour)}, nil
	})
	if got.status != checkStatusPass {
		t.Fatalf("got %#v, want pass", got)
	}
}

func TestCheckExternalPackageAgeFailsRegistryOutage(t *testing.T) {
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "outage", Enabled: true, Command: mcpTestUVXCommand, Args: []string{"outage-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, false, time.Now(), func(ref packageReference) (packageRelease, error) {
		return packageRelease{}, errors.New("timeout")
	})
	if got.status != checkStatusFail || !strings.Contains(got.detail, "registry lookup failed") {
		t.Fatalf("got %#v, want outage failure", got)
	}
}

func TestCheckExternalPackageAgeSkipFlag(t *testing.T) {
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "fresh", Enabled: true, Command: mcpTestUVXCommand, Args: []string{"fresh-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, true, time.Now(), func(ref packageReference) (packageRelease, error) {
		t.Fatal("resolver should not be called when skipped")
		return packageRelease{}, nil
	})
	if got.status != checkStatusPass || !strings.Contains(got.detail, "skipped") {
		t.Fatalf("got %#v, want skipped pass", got)
	}
}
