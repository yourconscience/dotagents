package main

import (
	"bytes"
	"encoding/json"
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

func TestIngestPreservesNumericSnowflakeIDs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("X_SIM_DB", filepath.Join(dir, "x-sim.sqlite"))
	db, err := openDB("")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	raw := []byte(`[{"id":1879998888777666555,"text":"Numeric snowflake id.","created_at":"2026-05-01T10:00:00Z","author":{"screen_name":"builder"},"like_count":1879998888777666555}]`)
	n, err := ingestJSON(db, source{Kind: "search", Value: "ids"}, raw, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 tweet, got %d", n)
	}
	var id string
	var likes int64
	if err := db.QueryRow(`SELECT id, like_count FROM tweets`).Scan(&id, &likes); err != nil {
		t.Fatal(err)
	}
	if id != "1879998888777666555" {
		t.Fatalf("id = %q, want 1879998888777666555", id)
	}
	if likes != 1879998888777666555 {
		t.Fatalf("like_count = %d, want 1879998888777666555", likes)
	}
}

func TestExtractTweetsFromGraphQLResult(t *testing.T) {
	raw := []byte(`{"tweet_results":{"result":{"rest_id":"2064394146916229443",
		"core":{"user_results":{"result":{"rest_id":"999","legacy":{"screen_name":"claudeai","name":"Claude"}}}},
		"legacy":{"full_text":"Introducing Claude Fable 5.","created_at":"Tue Jun 09 17:08:13 +0000 2026","favorite_count":17693,"retweet_count":2100}}}}`)
	var payload any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	tweets := extractTweets(payload, source{Kind: "account", Value: "AnthropicAI"}, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC))
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}
	tw := tweets[0]
	if tw.ID != "2064394146916229443" || tw.AuthorHandle != "claudeai" || tw.AuthorName != "Claude" {
		t.Fatalf("unexpected tweet identity: %+v", tw)
	}
	if tw.LikeCount != 17693 || tw.RepostCount != 2100 {
		t.Fatalf("unexpected counts: %+v", tw)
	}
	if got, want := tw.PostedAt.Format(time.RFC3339), "2026-06-09T17:08:13Z"; got != want {
		t.Fatalf("posted_at = %s, want %s", got, want)
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

func TestParseWindowCutoffNormalizesUTC(t *testing.T) {
	loc := time.FixedZone("GET", 4*60*60)
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, loc)
	cutoff, err := parseWindowCutoff("30d", now)
	if err != nil {
		t.Fatal(err)
	}
	if cutoff.Location() != time.UTC {
		t.Fatalf("cutoff location = %v, want UTC", cutoff.Location())
	}
	if got, want := cutoff.Format(time.RFC3339), "2026-04-21T08:00:00Z"; got != want {
		t.Fatalf("cutoff = %s, want %s", got, want)
	}
}

func TestParseWindowCutoffRejectsMalformedSuffix(t *testing.T) {
	for _, raw := range []string{"1yd", "1dm", "d", "30"} {
		if _, err := parseWindowCutoff(raw, time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)); err == nil {
			t.Fatalf("parseWindowCutoff(%q) succeeded, want error", raw)
		}
	}
}
