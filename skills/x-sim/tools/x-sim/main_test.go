package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIngestAndBrief(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("X_SIM_DB", filepath.Join(dir, "x-sim.sqlite"))
	db, err := openDB("")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	raw := []byte(`[
		{"id":"1","text":"Agent evals need real traces and boring measurement.","created_at":"2026-05-01T10:00:00Z","author":{"screen_name":"builder"},"like_count":10,"reply_count":3},
		{"id":"2","text":"A vague AI launch post is not enough.","created_at":"2026-05-02T10:00:00Z","author":{"screen_name":"critic"},"like_count":3}
	]`)
	n, err := ingestJSON(db, source{Kind: "search", Value: "agents"}, raw, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 tweets, got %d", n)
	}
	tweets, err := queryTweets(db, "agent", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 10)
	if err != nil {
		t.Fatal(err)
	}
	report := renderBrief("agent", tweets, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(report, "@builder") || !strings.Contains(report, "measurement") {
		t.Fatalf("brief missing expected content:\n%s", report)
	}
}

func TestEvalCommandStoresReport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("X_SIM_DB", filepath.Join(dir, "x-sim.sqlite"))
	out := filepath.Join(dir, "report.md")
	var stdout strings.Builder
	if err := run([]string{"eval-tweet", "--text", "I measured agent eval latency on a small benchmark and the cheap path was slower.", "--out", out}, &stdout, os.Stderr); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Offline Boundary") || !strings.Contains(string(data), "technical builder") {
		t.Fatalf("unexpected report:\n%s", data)
	}
}
