package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type inspection struct {
	PR              int           `json:"pr"`
	URL             string        `json:"url,omitempty"`
	BaseRefName     string        `json:"base_ref_name,omitempty"`
	HeadRefName     string        `json:"head_ref_name,omitempty"`
	Mergeable       string        `json:"mergeable,omitempty"`
	MergeState      string        `json:"merge_state_status,omitempty"`
	FailedChecks    []checkResult `json:"failed_checks,omitempty"`
	Unresolved      []thread      `json:"unresolved_threads,omitempty"`
	BotThreads      []thread      `json:"bot_threads,omitempty"`
	HumanThreads    []thread      `json:"human_threads,omitempty"`
	BodyEmpty       bool          `json:"body_empty"`
	RecommendedNext string        `json:"recommended_next"`
}

type checkResult struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion,omitempty"`
	DetailsURL string `json:"details_url,omitempty"`
}

type thread struct {
	ID       string `json:"id"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Author   string `json:"author,omitempty"`
	URL      string `json:"url,omitempty"`
	Body     string `json:"body,omitempty"`
	IsBot    bool   `json:"is_bot"`
	Severity string `json:"severity,omitempty"`
}

type hookResponse struct {
	Continue       bool   `json:"continue,omitempty"`
	SuppressOutput bool   `json:"suppressOutput,omitempty"`
	Decision       string `json:"decision,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Message        string `json:"message,omitempty"`
}

var botAuthor = regexp.MustCompile(`(?i)^(gemini|copilot|cursor|claude|codex|coderabbitai)`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pr-triage inspect|hook")
	}
	switch args[0] {
	case "inspect":
		return runInspect(args[1:])
	case "hook":
		return runHook(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "markdown", "Output format: markdown or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prArg := ""
	if fs.NArg() > 1 {
		return errors.New("usage: pr-triage inspect [--format markdown|json] [PR]")
	}
	if fs.NArg() == 1 {
		prArg = fs.Arg(0)
	}
	result, err := inspectPR(prArg)
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown":
		fmt.Print(renderMarkdown(result))
		return nil
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
}

func runHook(args []string) error {
	if len(args) != 1 || args[0] != "stop" {
		return errors.New("usage: pr-triage hook stop")
	}
	result, err := inspectPR("")
	if err != nil {
		return writeHook(hookResponse{Continue: true, SuppressOutput: true, Message: "pr-triage skipped: " + err.Error()})
	}
	blockers := hardBlockers(result)
	if len(blockers) > 0 {
		return writeHook(hookResponse{Decision: "block", Reason: strings.Join(blockers, "; ")})
	}
	return writeHook(hookResponse{Continue: true, SuppressOutput: true, Message: "pr-triage: PR has no hard blockers"})
}

func inspectPR(prArg string) (inspection, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return inspection{}, errors.New("gh CLI fallback unavailable")
	}
	if strings.TrimSpace(prArg) == "" {
		out, err := runGH("pr", "view", "--json", "number", "--jq", ".number")
		if err != nil {
			return inspection{}, errors.New("no PR detected for current branch")
		}
		prArg = strings.TrimSpace(out)
	}
	var raw struct {
		Number            int               `json:"number"`
		URL               string            `json:"url"`
		BaseRefName       string            `json:"baseRefName"`
		HeadRefName       string            `json:"headRefName"`
		Mergeable         string            `json:"mergeable"`
		MergeStateStatus  string            `json:"mergeStateStatus"`
		Body              string            `json:"body"`
		StatusCheckRollup []json.RawMessage `json:"statusCheckRollup"`
	}
	out, err := runGH("pr", "view", prArg, "--json", "number,url,baseRefName,headRefName,mergeable,mergeStateStatus,body,statusCheckRollup")
	if err != nil {
		return inspection{}, err
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return inspection{}, fmt.Errorf("parse gh pr view: %w", err)
	}
	threads, err := reviewThreads(prArg)
	if err != nil {
		return inspection{}, err
	}
	result := inspection{
		PR:          raw.Number,
		URL:         raw.URL,
		BaseRefName: raw.BaseRefName,
		HeadRefName: raw.HeadRefName,
		Mergeable:   raw.Mergeable,
		MergeState:  raw.MergeStateStatus,
		BodyEmpty:   strings.TrimSpace(raw.Body) == "",
		Unresolved:  threads,
	}
	result.FailedChecks = failedChecks(raw.StatusCheckRollup)
	for _, t := range threads {
		if t.IsBot {
			result.BotThreads = append(result.BotThreads, t)
		} else {
			result.HumanThreads = append(result.HumanThreads, t)
		}
	}
	result.RecommendedNext = recommendedNext(result)
	return result, nil
}

func reviewThreads(prArg string) ([]thread, error) {
	ownerRepo, err := runGH("repo", "view", "--json", "owner,name", "--jq", ".owner.login + \"/\" + .name")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(ownerRepo), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected gh repo owner/name: %q", ownerRepo)
	}
	query := `query($owner:String!, $repo:String!, $number:Int!){ repository(owner:$owner, name:$repo){ pullRequest(number:$number){ reviewThreads(first:100){ nodes { id isResolved isOutdated path line comments(first:20){ nodes { author{ login } body url createdAt } } } } } } }`
	out, err := runGH("api", "graphql", "-F", "owner="+parts[0], "-F", "repo="+parts[1], "-F", "number="+prArg, "-f", "query="+query, "--jq", `[.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false and .isOutdated==false) | {id, path, line, author: .comments.nodes[0].author.login, url: .comments.nodes[0].url, body: .comments.nodes[0].body}]`)
	if err != nil {
		return nil, err
	}
	var threads []thread
	if err := json.Unmarshal([]byte(out), &threads); err != nil {
		return nil, fmt.Errorf("parse review threads: %w", err)
	}
	for i := range threads {
		threads[i].IsBot = botAuthor.MatchString(threads[i].Author)
		threads[i].Severity = inferSeverity(threads[i].Body)
	}
	return threads, nil
}

func failedChecks(items []json.RawMessage) []checkResult {
	var checks []checkResult
	for _, item := range items {
		var raw map[string]interface{}
		if err := json.Unmarshal(item, &raw); err != nil {
			continue
		}
		status, _ := raw["status"].(string)
		conclusion, _ := raw["conclusion"].(string)
		if status != "COMPLETED" {
			continue
		}
		if conclusion == "" || conclusion == "SUCCESS" || conclusion == "NEUTRAL" || conclusion == "SKIPPED" {
			continue
		}
		name, _ := raw["name"].(string)
		if name == "" {
			name, _ = raw["workflowName"].(string)
		}
		url, _ := raw["detailsUrl"].(string)
		checks = append(checks, checkResult{Name: name, Conclusion: conclusion, DetailsURL: url})
	}
	return checks
}

func recommendedNext(result inspection) string {
	switch {
	case result.Mergeable == "CONFLICTING" || result.MergeState == "DIRTY":
		return "resolve merge conflicts before review triage"
	case len(result.FailedChecks) > 0:
		return "inspect failed checks and fix CI before resolving review threads"
	case len(result.HumanThreads) > 0:
		return "fix human feedback silently and leave threads for the human reviewer"
	case len(result.BotThreads) > 0:
		return "triage bot feedback; fix valid issues and resolve wrong/low-value bot threads"
	case result.BodyEmpty:
		return "update PR body with summary and verification"
	default:
		return "no hard blockers found"
	}
}

func hardBlockers(result inspection) []string {
	var blockers []string
	if result.Mergeable == "CONFLICTING" || result.MergeState == "DIRTY" {
		blockers = append(blockers, "merge conflicts must be resolved")
	}
	if len(result.FailedChecks) > 0 {
		blockers = append(blockers, fmt.Sprintf("%d failed check(s)", len(result.FailedChecks)))
	}
	if len(result.HumanThreads) > 0 {
		blockers = append(blockers, fmt.Sprintf("%d unresolved human thread(s)", len(result.HumanThreads)))
	}
	for _, thread := range result.BotThreads {
		if thread.Severity == "critical" || thread.Severity == "high" {
			blockers = append(blockers, "untriaged high-severity bot review thread")
			break
		}
	}
	return blockers
}

func inferSeverity(body string) string {
	lower := strings.ToLower(body)
	switch {
	case strings.Contains(lower, "critical"):
		return "critical"
	case strings.Contains(lower, "high"):
		return "high"
	case strings.Contains(lower, "medium"):
		return "medium"
	default:
		return ""
	}
}

func renderMarkdown(result inspection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## PR #%d Status\n", result.PR)
	fmt.Fprintf(&b, "Merge: %s / %s\n", valueOrDash(result.Mergeable), valueOrDash(result.MergeState))
	if len(result.FailedChecks) == 0 {
		fmt.Fprintf(&b, "CI: PASS or pending checks only\n")
	} else {
		fmt.Fprintf(&b, "CI: %d failed check(s)\n", len(result.FailedChecks))
		for _, check := range result.FailedChecks {
			fmt.Fprintf(&b, "- %s: %s %s\n", valueOrDash(check.Name), valueOrDash(check.Conclusion), check.DetailsURL)
		}
	}
	fmt.Fprintf(&b, "Reviews: %d unresolved (%d bot, %d human)\n", len(result.Unresolved), len(result.BotThreads), len(result.HumanThreads))
	fmt.Fprintf(&b, "Body: ")
	if result.BodyEmpty {
		fmt.Fprintf(&b, "empty\n")
	} else {
		fmt.Fprintf(&b, "present\n")
	}
	fmt.Fprintf(&b, "\nRecommended next: %s\n", result.RecommendedNext)
	return b.String()
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func writeHook(response hookResponse) error {
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(response)
}

func runGH(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
