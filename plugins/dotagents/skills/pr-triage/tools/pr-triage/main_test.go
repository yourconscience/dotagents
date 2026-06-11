package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFailedChecksFiltersSuccessfulAndNeutral(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"name":"lint","status":"COMPLETED","conclusion":"SUCCESS"}`),
		json.RawMessage(`{"name":"test","status":"COMPLETED","conclusion":"FAILURE","detailsUrl":"https://example.test/run"}`),
		json.RawMessage(`{"name":"skip","status":"COMPLETED","conclusion":"SKIPPED"}`),
		json.RawMessage(`{"name":"pending","status":"IN_PROGRESS"}`),
	}
	got := failedChecks(raw)
	if len(got) != 1 {
		t.Fatalf("failedChecks length = %d, want 1: %#v", len(got), got)
	}
	if got[0].Name != "test" || got[0].Conclusion != "FAILURE" {
		t.Fatalf("unexpected failed check: %#v", got[0])
	}
}

func TestHardBlockersIncludesHumanThreadsAndHighBot(t *testing.T) {
	result := inspection{
		FailedChecks: []checkResult{{Name: "test"}},
		HumanThreads: []thread{{Author: "alice"}},
		BotThreads:   []thread{{Author: "coderabbitai", IsBot: true, Severity: "high"}},
	}
	got := strings.Join(hardBlockers(result), "\n")
	for _, want := range []string{"failed check", "human thread", "high-severity bot"} {
		if !strings.Contains(got, want) {
			t.Fatalf("hard blockers %q missing %q", got, want)
		}
	}
}

func TestBotAuthorIncludesHostedCodexConnector(t *testing.T) {
	if !botAuthor.MatchString("chatgpt-codex-connector") {
		t.Fatal("chatgpt-codex-connector should be treated as a bot reviewer")
	}
}

func TestOwnerRepoFromPRURL(t *testing.T) {
	got, err := ownerRepoFromPRURL("https://github.com/yourconscience/dotagents/pull/59")
	if err != nil {
		t.Fatal(err)
	}
	if got != "yourconscience/dotagents" {
		t.Fatalf("ownerRepoFromPRURL = %q, want yourconscience/dotagents", got)
	}
	if _, err := ownerRepoFromPRURL("https://example.test/not-a-pr"); err == nil {
		t.Fatal("ownerRepoFromPRURL accepted invalid URL")
	}
}

func TestParseReviewThreadsPageFiltersAndReturnsNextCursor(t *testing.T) {
	page, err := parseReviewThreadsPage([]byte(`{
  "data": {
    "repository": {
      "pullRequest": {
        "reviewThreads": {
          "pageInfo": {"hasNextPage": true, "endCursor": "cursor-2"},
          "nodes": [
            {
              "id": "active",
              "isResolved": false,
              "isOutdated": false,
              "path": "file.go",
              "line": 12,
              "comments": {"nodes": [{"author": {"login": "chatgpt-codex-connector"}, "body": "fix this", "url": "https://example.test/thread"}]}
            },
            {
              "id": "resolved",
              "isResolved": true,
              "isOutdated": false,
              "path": "file.go",
              "line": 13,
              "comments": {"nodes": [{"author": {"login": "alice"}, "body": "done", "url": "https://example.test/resolved"}]}
            },
            {
              "id": "outdated",
              "isResolved": false,
              "isOutdated": true,
              "path": "file.go",
              "line": 14,
              "comments": {"nodes": [{"author": {"login": "bob"}, "body": "stale", "url": "https://example.test/outdated"}]}
            }
          ]
        }
      }
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if page.Next != "cursor-2" {
		t.Fatalf("next cursor = %q, want cursor-2", page.Next)
	}
	if len(page.Threads) != 1 {
		t.Fatalf("threads length = %d, want 1: %#v", len(page.Threads), page.Threads)
	}
	if page.Threads[0].ID != "active" || page.Threads[0].Author != "chatgpt-codex-connector" {
		t.Fatalf("unexpected thread: %#v", page.Threads[0])
	}
}

func TestRenderMarkdownIncludesRecommendedNext(t *testing.T) {
	result := inspection{PR: 42, Mergeable: "MERGEABLE", MergeState: "CLEAN", RecommendedNext: "no hard blockers found"}
	out := renderMarkdown(result)
	if !strings.Contains(out, "PR #42") || !strings.Contains(out, "no hard blockers found") {
		t.Fatalf("unexpected markdown:\n%s", out)
	}
}
