package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCronCommandForWeeklyDeps(t *testing.T) {
	cmd, interval := cronCommandForOptions("/repo", "/usr/bin/go", cronOptions{Deps: true, Interval: cronIntervalDefault})
	if interval != cronIntervalWeekly {
		t.Fatalf("interval = %q, want %s", interval, cronIntervalWeekly)
	}
	if !strings.Contains(cmd, "deps update") {
		t.Fatalf("cmd = %q, want deps update", cmd)
	}
	if !strings.Contains(cmd, "/repo/cmd/dotagents") {
		t.Fatalf("cmd = %q, want root CLI path", cmd)
	}
	if strings.Contains(cmd, " pull") {
		t.Fatalf("cmd = %q, should not use pull mode", cmd)
	}
}

func TestPathContainsDir(t *testing.T) {
	path := strings.Join([]string{"/usr/bin", "/Users/me/go/bin"}, string(':'))
	if !pathContainsDir(path, "/Users/me/go/bin") {
		t.Fatal("expected PATH to contain Go bin dir")
	}
	if pathContainsDir(path, "/Users/me/.local/bin") {
		t.Fatal("unexpected PATH match")
	}
}

func TestGoBinDirUsesEffectiveGOBIN(t *testing.T) {
	fakeGo := writeFakeGo(t)
	t.Setenv("FAKE_GOBIN", "/custom/go/bin")
	t.Setenv("FAKE_GOPATH", "/wrong/go")

	got, err := goBinDir(fakeGo)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/go/bin" {
		t.Fatalf("goBinDir = %q, want effective GOBIN", got)
	}
}

func TestGoBinDirFallsBackToGOPATHBin(t *testing.T) {
	fakeGo := writeFakeGo(t)
	t.Setenv("FAKE_GOBIN", "")
	t.Setenv("FAKE_GOPATH", strings.Join([]string{"/first/go", "/second/go"}, string(os.PathListSeparator)))

	got, err := goBinDir(fakeGo)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("/first/go", "bin") {
		t.Fatalf("goBinDir = %q, want first GOPATH bin", got)
	}
}

func writeFakeGo(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go")
	data := `#!/bin/sh
if [ "$1" = "env" ] && [ "$2" = "GOBIN" ]; then
  printf '%s\n' "$FAKE_GOBIN"
  exit 0
fi
if [ "$1" = "env" ] && [ "$2" = "GOPATH" ]; then
  printf '%s\n' "$FAKE_GOPATH"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIntervalToScheduleWeekly(t *testing.T) {
	if got := intervalToSchedule(cronIntervalWeekly); got != "0 4 * * 1" {
		t.Fatalf("weekly schedule = %q, want Monday 04:00", got)
	}
}
