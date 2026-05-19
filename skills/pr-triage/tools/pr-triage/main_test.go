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

func TestRenderMarkdownIncludesRecommendedNext(t *testing.T) {
	result := inspection{PR: 42, Mergeable: "MERGEABLE", MergeState: "CLEAN", RecommendedNext: "no hard blockers found"}
	out := renderMarkdown(result)
	if !strings.Contains(out, "PR #42") || !strings.Contains(out, "no hard blockers found") {
		t.Fatalf("unexpected markdown:\n%s", out)
	}
}
