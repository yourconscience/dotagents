package main

import (
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

func TestIntervalToScheduleWeekly(t *testing.T) {
	if got := intervalToSchedule(cronIntervalWeekly); got != "0 4 * * 1" {
		t.Fatalf("weekly schedule = %q, want Monday 04:00", got)
	}
}
