package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPackageReferencesFromCommandUVX(t *testing.T) {
	refs := packageReferencesFromCommand("uvx", []string{"linkedin-scraper-mcp==4.12.0"}, "config")
	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one", refs)
	}
	if refs[0].Ecosystem != "pypi" || refs[0].Package != "linkedin-scraper-mcp" || refs[0].Version != "4.12.0" {
		t.Fatalf("unexpected ref: %#v", refs[0])
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
	if refs[1].Package != "@openai/codex" || refs[1].Version != "latest" {
		t.Fatalf("unexpected npm ref: %#v", refs[1])
	}
	if refs[2].Package != "@scope/pkg" || refs[2].Version != "1.2.3" {
		t.Fatalf("unexpected pnpm ref: %#v", refs[2])
	}
}

func TestCheckExternalPackageAgeFailsFreshPackage(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "fresh", Enabled: true, Command: "uvx", Args: []string{"fresh-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, false, now, func(ref packageReference) (packageRelease, error) {
		return packageRelease{Version: "1.0.0", Released: now.Add(-48 * time.Hour)}, nil
	})
	if got.status != "fail" || !strings.Contains(got.detail, "newer than 7 days") {
		t.Fatalf("got %#v, want fresh package failure", got)
	}
}

func TestCheckExternalPackageAgePassesOldPackage(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "old", Enabled: true, Command: "uvx", Args: []string{"old-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, false, now, func(ref packageReference) (packageRelease, error) {
		return packageRelease{Version: "1.0.0", Released: now.Add(-8 * 24 * time.Hour)}, nil
	})
	if got.status != "pass" {
		t.Fatalf("got %#v, want pass", got)
	}
}

func TestCheckExternalPackageAgeFailsRegistryOutage(t *testing.T) {
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "outage", Enabled: true, Command: "uvx", Args: []string{"outage-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, false, time.Now(), func(ref packageReference) (packageRelease, error) {
		return packageRelease{}, errors.New("timeout")
	})
	if got.status != "fail" || !strings.Contains(got.detail, "registry lookup failed") {
		t.Fatalf("got %#v, want outage failure", got)
	}
}

func TestCheckExternalPackageAgeSkipFlag(t *testing.T) {
	cfg := config{MCPServers: []mcpServerConfig{{
		Name: "fresh", Enabled: true, Command: "uvx", Args: []string{"fresh-pkg@latest"},
	}}}
	got := checkExternalPackageAgeWithResolver(t.TempDir(), cfg, true, time.Now(), func(ref packageReference) (packageRelease, error) {
		t.Fatal("resolver should not be called when skipped")
		return packageRelease{}, nil
	})
	if got.status != "pass" || !strings.Contains(got.detail, "skipped") {
		t.Fatalf("got %#v, want skipped pass", got)
	}
}
