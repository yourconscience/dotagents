package main

import (
	"strings"
	"testing"
)

func TestPlanQueriesComparison(t *testing.T) {
	got := planQueries("OpenClaw vs Hermes")
	joined := strings.Join(got, "\n")
	for _, want := range []string{"OpenClaw vs Hermes", "OpenClaw", "Hermes", "OpenClaw vs Hermes comparison"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("planQueries missing %q in %#v", want, got)
		}
	}
}
func TestNormalizeSearchArgsAllowsTopicBeforeFlags(t *testing.T) {
	got := normalizeSearchArgs([]string{"Claude Code hooks", "--sources", "hn,github", "--format=json"})
	want := []string{"--sources", "hn,github", "--format=json", "Claude Code hooks"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("normalizeSearchArgs = %#v, want %#v", got, want)
	}
}
func TestHNThreadURLPrefersDiscussion(t *testing.T) {
	got := hnThreadURL("12345")
	if got != "https://news.ycombinator.com/item?id=12345" {
		t.Fatalf("hnThreadURL = %q", got)
	}
}

func TestGitHubIssueSearchQueryScopesExactRepo(t *testing.T) {
	req := searchRequest{Topic: "owner/repo", Query: "owner/repo issues"}
	got := githubIssueSearchQuery(req, "2026-06-01")
	want := "repo:owner/repo updated:>=2026-06-01"
	if got != want {
		t.Fatalf("githubIssueSearchQuery = %q, want %q", got, want)
	}
}

func TestCanonicalURLStripsTrackingAndRedditSlug(t *testing.T) {
	got := canonicalURL("http://www.reddit.com/r/ClaudeAI/comments/abc123/some_slug/?utm_source=x&ref=share#frag")
	want := "https://reddit.com/r/ClaudeAI/comments/abc123"
	if got != want {
		t.Fatalf("canonicalURL = %q, want %q", got, want)
	}
}

func TestRankAndDedupeMergesTrackedURLs(t *testing.T) {
	items := []item{
		{Source: "hn", Title: "Claude Code hooks launch", URL: "https://example.com/post?utm_source=hn", Score: 10},
		{Source: "reddit", Title: "Claude Code hooks launch discussion", URL: "https://example.com/post?ref=reddit", Score: 20},
	}
	got := rankAndDedupe(items, "Claude Code hooks", 10)
	if len(got) != 1 {
		t.Fatalf("rankAndDedupe length = %d, want 1: %#v", len(got), got)
	}
	if got[0].Deduped != 1 {
		t.Fatalf("Deduped = %d, want 1", got[0].Deduped)
	}
}

func TestParseRedditFeedAtom(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><feed><entry><title>OpenClaw workflow</title><id>https://www.reddit.com/r/ClaudeCode/comments/abc/openclaw/</id><updated>2026-06-01T12:00:00Z</updated><author><name>alice</name></author><content type="html">&lt;p&gt;useful thread&lt;/p&gt;</content><link href="https://www.reddit.com/r/ClaudeCode/comments/abc/openclaw/" rel="alternate" /></entry></feed>`)
	got, err := parseRedditFeed("OpenClaw", body, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "OpenClaw workflow" || got[0].Published != "2026-06-01" {
		t.Fatalf("unexpected feed parse: %#v", got)
	}
	if got[0].Summary != "useful thread" {
		t.Fatalf("Summary = %q, want useful thread", got[0].Summary)
	}
}

func TestParseGenericSocialJSONFindsNestedItems(t *testing.T) {
	body := []byte(`{"data":{"posts":[{"title":"Hermes workflow","url":"https://x.com/a/status/1","author":"a","likes":42,"text":"Hermes production workflow"}]}}`)
	got := parseGenericSocialJSON("x", "Hermes", body)
	if len(got) != 1 {
		t.Fatalf("items = %d, want 1: %#v", len(got), got)
	}
	if got[0].Score != 42 || got[0].Author != "a" {
		t.Fatalf("unexpected item: %#v", got[0])
	}
}
