package main

import (
	"strings"
	"testing"
)

func TestCronCommandForWeeklyDeps(t *testing.T) {
	cmd, interval := cronCommandForOptions("/repo", "/usr/bin/go", cronOptions{Deps: true, Interval: "30m"})
	if interval != "weekly" {
		t.Fatalf("interval = %q, want weekly", interval)
	}
	if !strings.Contains(cmd, "deps update") {
		t.Fatalf("cmd = %q, want deps update", cmd)
	}
	if strings.Contains(cmd, " pull") {
		t.Fatalf("cmd = %q, should not use pull mode", cmd)
	}
}

func TestIntervalToScheduleWeekly(t *testing.T) {
	if got := intervalToSchedule("weekly"); got != "0 4 * * 1" {
		t.Fatalf("weekly schedule = %q, want Monday 04:00", got)
	}
}
