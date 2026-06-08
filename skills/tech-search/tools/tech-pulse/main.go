package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultUserAgent = "dotagents-tech-pulse/0.1 (+https://github.com/yourconscience/dotagents)"
	redditUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

var (
	vsRegexp      = regexp.MustCompile(`(?i)\s+(?:vs\.?|versus)\s+`)
	intentRegexp  = regexp.MustCompile(`(?i)\b(how to|use cases?|workflows?|examples?|tutorials?|reviews?|in practice|production use)\b`)
	htmlTagRegexp = regexp.MustCompile(`<[^>]+>`)
	dateRegexp    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	repoRegexp    = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

type report struct {
	Topic        string                  `json:"topic"`
	GeneratedAt  string                  `json:"generated_at"`
	LookbackDays int                     `json:"lookback_days"`
	Queries      []string                `json:"queries"`
	Sources      map[string]sourceStatus `json:"sources"`
	Items        []item                  `json:"items"`
	Warnings     []string                `json:"warnings,omitempty"`
}

type sourceStatus struct {
	Requested bool   `json:"requested"`
	Available bool   `json:"available"`
	Count     int    `json:"count"`
	Error     string `json:"error,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type item struct {
	Source    string  `json:"source"`
	Title     string  `json:"title"`
	URL       string  `json:"url"`
	Author    string  `json:"author,omitempty"`
	Container string  `json:"container,omitempty"`
	Published string  `json:"published,omitempty"`
	Score     int     `json:"score,omitempty"`
	Comments  int     `json:"comments,omitempty"`
	Summary   string  `json:"summary,omitempty"`
	Query     string  `json:"query,omitempty"`
	Repo      string  `json:"repo,omitempty"`
	Relevance float64 `json:"relevance"`
	Deduped   int     `json:"deduped,omitempty"`
}

type sourceRunner func(context.Context, searchRequest) ([]item, error)

type searchRequest struct {
	Topic string
	Query string
	Days  int
	Limit int
}

type app struct {
	httpClient *http.Client
	runners    map[string]sourceRunner
}

func main() {
	if err := newApp().run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newApp() *app {
	a := &app{httpClient: &http.Client{Timeout: 20 * time.Second}}
	a.runners = map[string]sourceRunner{
		"hn":     a.searchHN,
		"github": a.searchGitHub,
		"reddit": a.searchReddit,
		"x":      a.searchX,
	}
	return a
}

func (a *app) run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tech-pulse diagnose|search")
	}
	switch args[0] {
	case "diagnose":
		return a.runDiagnose(args[1:])
	case "search":
		return a.runSearch(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *app) runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "markdown", "Output format: markdown or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	statuses := diagnoseSources([]string{"hn", "reddit", "x", "github"})
	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(statuses)
	}
	if *format != "markdown" {
		return fmt.Errorf("unsupported format %q", *format)
	}
	fmt.Print(renderDiagnoseMarkdown(statuses))
	return nil
}

func (a *app) runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	days := fs.Int("days", 30, "Lookback window in days")
	sourcesFlag := fs.String("sources", "hn,reddit,x,github", "Comma-separated sources: hn,reddit,x,github")
	limit := fs.Int("limit", 10, "Maximum results per source")
	format := fs.String("format", "markdown", "Output format: markdown or json")
	rawPath := fs.String("raw", "", "Optional path to write raw JSON report")
	timeout := fs.Duration("timeout", 45*time.Second, "Total search timeout")
	if err := fs.Parse(normalizeSearchArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: tech-pulse search [--days N] [--sources hn,reddit,x,github] [--format markdown|json] <topic>")
	}
	topic := strings.TrimSpace(fs.Arg(0))
	if topic == "" {
		return errors.New("topic is required")
	}
	if *days <= 0 {
		return errors.New("--days must be positive")
	}
	if *limit <= 0 {
		return errors.New("--limit must be positive")
	}
	sources, err := parseSources(*sourcesFlag)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	result := a.search(ctx, topic, *days, *limit, sources)
	if *rawPath != "" {
		if err := writeJSON(*rawPath, result); err != nil {
			return err
		}
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
func normalizeSearchArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	valueFlags := map[string]bool{
		"days": true, "sources": true, "limit": true, "format": true, "raw": true, "timeout": true,
	}
	var flags []string
	var topics []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			topics = append(topics, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if valueFlags[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, topics...)
}

func parseSources(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("--sources requires at least one source")
	}
	aliases := map[string]string{"hackernews": "hn", "twitter": "x", "x.com": "x", "github": "github", "gh": "github"}
	valid := map[string]bool{"hn": true, "reddit": true, "x": true, "github": true}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		s := strings.ToLower(strings.TrimSpace(part))
		if s == "" {
			continue
		}
		if alias, ok := aliases[s]; ok {
			s = alias
		}
		if !valid[s] {
			return nil, fmt.Errorf("unsupported source %q", s)
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("--sources requires at least one source")
	}
	return out, nil
}

func (a *app) search(ctx context.Context, topic string, days int, limit int, sources []string) report {
	queries := planQueries(topic)
	statuses := diagnoseSources(sources)
	result := report{
		Topic:        topic,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		LookbackDays: days,
		Queries:      queries,
		Sources:      statuses,
	}
	type streamResult struct {
		source string
		items  []item
		err    error
	}
	jobs := make(chan streamResult, len(sources)*len(queries))
	var wg sync.WaitGroup
	for _, source := range sources {
		runner := a.runners[source]
		status := statuses[source]
		if !status.Available && source != "hn" && source != "github" && source != "reddit" {
			continue
		}
		for _, q := range queries {
			wg.Add(1)
			go func(source string, query string) {
				defer wg.Done()
				items, err := runner(ctx, searchRequest{Topic: topic, Query: query, Days: days, Limit: limit})
				jobs <- streamResult{source: source, items: items, err: err}
			}(source, q)
		}
	}
	go func() {
		wg.Wait()
		close(jobs)
	}()
	var all []item
	for sr := range jobs {
		status := result.Sources[sr.source]
		status.Requested = true
		if sr.err != nil {
			if status.Error == "" {
				status.Error = sr.err.Error()
			}
			result.Sources[sr.source] = status
			continue
		}
		all = append(all, sr.items...)
		status.Count += len(sr.items)
		result.Sources[sr.source] = status
	}
	result.Items = rankAndDedupe(all, topic, limit*len(sources))
	for source := range result.Sources {
		status := result.Sources[source]
		status.Count = 0
		for _, item := range result.Items {
			if item.Source == source {
				status.Count++
			}
		}
		result.Sources[source] = status
	}
	if len(result.Items) == 0 {
		result.Warnings = append(result.Warnings, "no source returned usable items")
	}
	return result
}

func diagnoseSources(sources []string) map[string]sourceStatus {
	out := map[string]sourceStatus{}
	for _, source := range sources {
		status := sourceStatus{Requested: true, Available: true}
		switch source {
		case "hn":
			status.Detail = "HN Algolia API, no credentials required"
		case "reddit":
			if _, err := exec.LookPath("rdt"); err == nil {
				status.Detail = "rdt available; RSS fallback remains available"
			} else {
				status.Detail = "rdt unavailable; will use Reddit RSS fallback"
			}
		case "x":
			if _, err := exec.LookPath("x-cli"); err == nil {
				status.Detail = "x-cli available"
			} else {
				status.Available = false
				status.Detail = "x-cli unavailable; source skipped"
			}
		case "github":
			if tokenPresent("GITHUB_TOKEN") || tokenPresent("GH_TOKEN") {
				status.Detail = "GitHub REST API with token"
			} else {
				status.Detail = "GitHub REST API without token; lower rate limit"
			}
		}
		out[source] = status
	}
	return out
}

func tokenPresent(name string) bool { return strings.TrimSpace(os.Getenv(name)) != "" }

func renderDiagnoseMarkdown(statuses map[string]sourceStatus) string {
	var b strings.Builder
	b.WriteString("# tech-pulse diagnose\n\n")
	for _, source := range sortedSourceNames(statuses) {
		st := statuses[source]
		mark := "ok"
		if !st.Available {
			mark = "skipped"
		}
		fmt.Fprintf(&b, "- **%s**: %s", source, mark)
		if st.Detail != "" {
			fmt.Fprintf(&b, " - %s", st.Detail)
		}
		if st.Error != "" {
			fmt.Fprintf(&b, " - %s", st.Error)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func planQueries(topic string) []string {
	clean := compactSpaces(topic)
	if clean == "" {
		return nil
	}
	queries := []string{clean}
	for _, part := range splitComparison(clean) {
		if part != clean {
			queries = append(queries, part)
		}
	}
	lower := strings.ToLower(clean)
	switch {
	case strings.Contains(lower, " vs ") || strings.Contains(lower, " versus "):
		queries = append(queries, clean+" comparison", clean+" reddit")
	case strings.Contains(lower, "how to") || strings.Contains(lower, "workflow") || strings.Contains(lower, "use case"):
		base := stripIntentModifiers(clean)
		queries = append(queries, base+" workflow", base+" production", base+" examples")
	case maybeRepo(clean):
		queries = append(queries, clean+" issues", clean+" releases")
	default:
		queries = append(queries, clean+" review", clean+" discussion")
	}
	return uniqueStringsLimited(queries, 5)
}

func splitComparison(topic string) []string {
	parts := vsRegexp.Split(topic, -1)
	var out []string
	for _, part := range parts {
		part = compactSpaces(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func stripIntentModifiers(topic string) string {
	return compactSpaces(intentRegexp.ReplaceAllString(topic, ""))
}

func uniqueStringsLimited(values []string, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = compactSpaces(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (a *app) searchHN(ctx context.Context, req searchRequest) ([]item, error) {
	threshold := time.Now().UTC().AddDate(0, 0, -req.Days).Unix()
	values := url.Values{}
	values.Set("query", req.Query)
	values.Set("tags", "story")
	values.Set("hitsPerPage", strconv.Itoa(req.Limit))
	values.Set("numericFilters", fmt.Sprintf("created_at_i>%d", threshold))
	endpoint := "https://hn.algolia.com/api/v1/search?" + values.Encode()
	var payload struct {
		Hits []struct {
			Title     string `json:"title"`
			URL       string `json:"url"`
			ObjectID  string `json:"objectID"`
			Author    string `json:"author"`
			Points    int    `json:"points"`
			Comments  int    `json:"num_comments"`
			CreatedAt string `json:"created_at"`
			StoryText string `json:"story_text"`
		} `json:"hits"`
	}
	if err := a.getJSON(ctx, endpoint, nil, &payload); err != nil {
		return nil, err
	}
	items := make([]item, 0, len(payload.Hits))
	for _, hit := range payload.Hits {
		link := strings.TrimSpace(hit.URL)
		if link == "" && hit.ObjectID != "" {
			link = "https://news.ycombinator.com/item?id=" + hit.ObjectID
		}
		items = append(items, item{
			Source: "hn", Title: cleanText(hit.Title), URL: link, Author: hit.Author,
			Published: dateOnly(hit.CreatedAt), Score: hit.Points, Comments: hit.Comments,
			Summary: cleanText(hit.StoryText), Query: req.Query,
		})
	}
	return items, nil
}

func (a *app) searchGitHub(ctx context.Context, req searchRequest) ([]item, error) {
	var items []item
	repos, err := a.searchGitHubRepos(ctx, req)
	if err != nil {
		return nil, err
	}
	items = append(items, repos...)
	issues, err := a.searchGitHubIssues(ctx, req)
	if err == nil {
		items = append(items, issues...)
	}
	if err != nil && len(items) == 0 {
		return nil, err
	}
	return items, nil
}

func (a *app) searchGitHubRepos(ctx context.Context, req searchRequest) ([]item, error) {
	values := url.Values{}
	values.Set("q", req.Query+" in:name,description")
	values.Set("sort", "updated")
	values.Set("order", "desc")
	values.Set("per_page", strconv.Itoa(min(req.Limit, 20)))
	endpoint := "https://api.github.com/search/repositories?" + values.Encode()
	var payload struct {
		Items []struct {
			FullName    string `json:"full_name"`
			HTMLURL     string `json:"html_url"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			OpenIssues  int    `json:"open_issues_count"`
			UpdatedAt   string `json:"updated_at"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"items"`
	}
	if err := a.getJSON(ctx, endpoint, githubHeaders(), &payload); err != nil {
		return nil, err
	}
	items := make([]item, 0, len(payload.Items))
	for _, repo := range payload.Items {
		items = append(items, item{
			Source: "github", Title: repo.FullName, URL: repo.HTMLURL, Author: repo.Owner.Login,
			Published: dateOnly(repo.UpdatedAt), Score: repo.Stars, Comments: repo.OpenIssues,
			Summary: cleanText(repo.Description), Query: req.Query, Repo: repo.FullName,
		})
	}
	return items, nil
}

func (a *app) searchGitHubIssues(ctx context.Context, req searchRequest) ([]item, error) {
	since := time.Now().UTC().AddDate(0, 0, -req.Days).Format("2006-01-02")
	values := url.Values{}
	values.Set("q", req.Query+" updated:>="+since)
	values.Set("sort", "updated")
	values.Set("order", "desc")
	values.Set("per_page", strconv.Itoa(min(req.Limit, 20)))
	endpoint := "https://api.github.com/search/issues?" + values.Encode()
	var payload struct {
		Items []struct {
			Title     string `json:"title"`
			HTMLURL   string `json:"html_url"`
			State     string `json:"state"`
			Comments  int    `json:"comments"`
			UpdatedAt string `json:"updated_at"`
			User      struct {
				Login string `json:"login"`
			} `json:"user"`
			Repository  string    `json:"repository_url"`
			PullRequest *struct{} `json:"pull_request"`
		} `json:"items"`
	}
	if err := a.getJSON(ctx, endpoint, githubHeaders(), &payload); err != nil {
		return nil, err
	}
	items := make([]item, 0, len(payload.Items))
	for _, issue := range payload.Items {
		repo := repoFromAPIURL(issue.Repository)
		title := issue.Title
		if repo != "" {
			title = repo + ": " + title
		}
		kind := "issue"
		if issue.PullRequest != nil {
			kind = "PR"
		}
		items = append(items, item{
			Source: "github", Title: cleanText(title), URL: issue.HTMLURL, Author: issue.User.Login,
			Published: dateOnly(issue.UpdatedAt), Comments: issue.Comments,
			Summary: "GitHub " + kind + ", state: " + issue.State, Query: req.Query, Repo: repo,
		})
	}
	return items, nil
}

func githubHeaders() map[string]string {
	headers := map[string]string{"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		headers["Authorization"] = "Bearer " + token
	} else if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

func (a *app) searchReddit(ctx context.Context, req searchRequest) ([]item, error) {
	if _, err := exec.LookPath("rdt"); err == nil {
		items, err := searchRedditRDT(ctx, req)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}
	return a.searchRedditRSS(ctx, req)
}

func searchRedditRDT(ctx context.Context, req searchRequest) ([]item, error) {
	args := []string{"search", req.Query, "-s", "relevance", "-t", redditWindow(req.Days), "-n", strconv.Itoa(req.Limit), "--compact", "--json"}
	out, err := runCommand(ctx, "rdt", args...)
	if err != nil {
		return nil, err
	}
	return parseGenericSocialJSON("reddit", req.Query, out), nil
}

func (a *app) searchRedditRSS(ctx context.Context, req searchRequest) ([]item, error) {
	values := url.Values{}
	values.Set("q", req.Query)
	values.Set("sort", "relevance")
	values.Set("t", redditWindow(req.Days))
	endpoint := "https://www.reddit.com/search.rss?" + values.Encode()
	body, err := a.getBytes(ctx, endpoint, map[string]string{
		"Accept":     "application/atom+xml, application/xml;q=0.9, */*;q=0.1",
		"User-Agent": redditUserAgent,
	})
	if err != nil {
		return nil, err
	}
	return parseRedditFeed(req.Query, body, req.Limit)
}

func (a *app) searchX(ctx context.Context, req searchRequest) ([]item, error) {
	if _, err := exec.LookPath("x-cli"); err != nil {
		return nil, errors.New("x-cli unavailable")
	}
	out, err := runCommand(ctx, "x-cli", "search", req.Query, "--type", "top", "--count", strconv.Itoa(req.Limit), "--json")
	if err != nil {
		return nil, err
	}
	return parseGenericSocialJSON("x", req.Query, out), nil
}

func redditWindow(days int) string {
	switch {
	case days <= 1:
		return "day"
	case days <= 7:
		return "week"
	case days <= 31:
		return "month"
	default:
		return "year"
	}
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s failed: %s", name, msg)
	}
	return out, nil
}

func (a *app) getJSON(ctx context.Context, endpoint string, headers map[string]string, dest any) error {
	body, err := a.getBytes(ctx, endpoint, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("parse %s: %w", endpoint, err)
	}
	return nil
}

func (a *app) getBytes(ctx context.Context, endpoint string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 300)])))
	}
	return body, nil
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	ID      string     `xml:"id"`
	Updated string     `xml:"updated"`
	Author  atomAuthor `xml:"author"`
	Content string     `xml:"content"`
	Links   []atomLink `xml:"link"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func parseRedditFeed(query string, body []byte, limit int) ([]item, error) {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse reddit rss: %w", err)
	}
	var items []item
	for _, entry := range feed.Entries {
		link := entry.ID
		for _, candidate := range entry.Links {
			if candidate.Href != "" && (candidate.Rel == "alternate" || link == "") {
				link = candidate.Href
			}
		}
		items = append(items, item{Source: "reddit", Title: cleanText(entry.Title), URL: link, Author: entry.Author.Name, Published: dateOnly(entry.Updated), Summary: cleanText(entry.Content), Query: query})
	}
	for _, entry := range feed.Channel.Items {
		items = append(items, item{Source: "reddit", Title: cleanText(entry.Title), URL: entry.Link, Published: dateOnly(entry.PubDate), Summary: cleanText(entry.Description), Query: query})
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func parseGenericSocialJSON(source string, query string, body []byte) []item {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil
	}
	var maps []map[string]any
	collectMaps(value, &maps)
	var out []item
	seen := map[string]bool{}
	for _, m := range maps {
		link := firstString(m, "url", "link", "permalink", "tweet_url", "html_url")
		text := firstString(m, "text", "content", "body", "selftext", "title", "full_text")
		if link == "" && text == "" {
			continue
		}
		key := link
		if key == "" {
			key = hashText(text)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item{
			Source: source, Title: fallbackTitle(cleanText(firstString(m, "title", "name")), text), URL: link,
			Author: firstString(m, "author", "username", "screen_name", "handle"), Container: firstString(m, "subreddit", "community", "channel"),
			Published: dateOnly(firstString(m, "created_at", "date", "timestamp", "published")), Score: firstInt(m, "score", "upvotes", "likes", "favorite_count", "retweet_count"),
			Comments: firstInt(m, "comments", "num_comments", "reply_count"), Summary: cleanText(text), Query: query,
		})
	}
	return out
}

func collectMaps(value any, out *[]map[string]any) {
	switch v := value.(type) {
	case map[string]any:
		*out = append(*out, v)
		for _, child := range v {
			collectMaps(child, out)
		}
	case []any:
		for _, child := range v {
			collectMaps(child, out)
		}
	}
}

func rankAndDedupe(items []item, topic string, limit int) []item {
	prepared := prepareTopic(topic)
	merged := map[string]item{}
	for _, it := range items {
		it.Title = cleanText(it.Title)
		it.Summary = trimRunes(cleanText(it.Summary), 400)
		it.URL = strings.TrimSpace(it.URL)
		if it.Title == "" && it.Summary == "" {
			continue
		}
		it.Relevance = scoreItem(it, prepared)
		key := dedupeKey(it)
		if existing, ok := merged[key]; ok {
			it = mergeItems(existing, it)
		}
		merged[key] = it
	}
	out := make([]item, 0, len(merged))
	for _, it := range merged {
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Relevance != out[j].Relevance {
			return out[i].Relevance > out[j].Relevance
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Published > out[j].Published
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func mergeItems(a, b item) item {
	if b.Relevance > a.Relevance || (b.Relevance == a.Relevance && b.Score+b.Comments > a.Score+a.Comments) {
		b.Deduped = a.Deduped + 1
		if b.Summary == "" {
			b.Summary = a.Summary
		}
		return b
	}
	a.Deduped++
	if a.Summary == "" {
		a.Summary = b.Summary
	}
	return a
}

func scoreItem(it item, terms []string) float64 {
	text := strings.ToLower(it.Title + " " + it.Summary + " " + it.Repo + " " + it.Container)
	matches := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			matches++
		}
	}
	var score float64
	if len(terms) > 0 {
		score = float64(matches) / float64(len(terms))
	}
	score += minFloat(0.35, float64(it.Score)/1000.0)
	score += minFloat(0.20, float64(it.Comments)/500.0)
	if it.Published != "" {
		score += 0.05
	}
	if it.URL != "" {
		score += 0.05
	}
	return score
}

func prepareTopic(topic string) []string {
	words := strings.Fields(strings.ToLower(stripIntentModifiers(topic)))
	stop := map[string]bool{"the": true, "and": true, "or": true, "vs": true, "versus": true, "for": true, "with": true, "about": true, "review": true, "discussion": true}
	var out []string
	seen := map[string]bool{}
	for _, word := range words {
		word = strings.Trim(word, "\"'()[]{}.,:;!?/")
		if len(word) < 3 || stop[word] || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
	}
	return out
}

func dedupeKey(it item) string {
	if it.URL != "" {
		return canonicalURL(it.URL)
	}
	return "text:" + hashText(strings.ToLower(it.Title+"\n"+it.Summary))
}

func canonicalURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	u.Scheme = "https"
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	q := u.Query()
	for key := range q {
		lk := strings.ToLower(key)
		if strings.HasPrefix(lk, "utm_") || lk == "ref" || lk == "ref_src" || lk == "source" || lk == "feature" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	u.Path = path.Clean("/" + strings.Trim(u.Path, "/"))
	if u.Path == "/." {
		u.Path = "/"
	}
	if u.Host == "reddit.com" && strings.Contains(u.Path, "/comments/") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, part := range parts {
			if part == "comments" && i+1 < len(parts) {
				u.Path = "/" + strings.Join(parts[:i+2], "/")
				break
			}
		}
	}
	return u.String()
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func renderMarkdown(r report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s - Tech Pulse\n\n", r.Topic)
	fmt.Fprintf(&b, "Generated: %s UTC · Lookback: %d days\n\n", dateOnly(r.GeneratedAt), r.LookbackDays)
	b.WriteString("## Source status\n\n")
	for _, source := range sortedSourceNames(r.Sources) {
		st := r.Sources[source]
		status := "ok"
		if !st.Available {
			status = "skipped"
		} else if st.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "- **%s**: %s, %d kept", source, status, st.Count)
		if st.Detail != "" {
			fmt.Fprintf(&b, " - %s", st.Detail)
		}
		if st.Error != "" {
			fmt.Fprintf(&b, " - %s", st.Error)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n## Top items\n\n")
	for i, it := range r.Items {
		label := it.Title
		if label == "" {
			label = trimRunes(it.Summary, 80)
		}
		fmt.Fprintf(&b, "%d. **%s**", i+1, label)
		if it.URL != "" {
			fmt.Fprintf(&b, " - %s", it.URL)
		}
		fmt.Fprintf(&b, "\n   - source: %s", it.Source)
		if it.Container != "" {
			fmt.Fprintf(&b, " / %s", it.Container)
		}
		if it.Score != 0 || it.Comments != 0 {
			fmt.Fprintf(&b, "; score: %d; comments: %d", it.Score, it.Comments)
		}
		if it.Published != "" {
			fmt.Fprintf(&b, "; date: %s", it.Published)
		}
		if it.Deduped > 0 {
			fmt.Fprintf(&b, "; deduped: %d", it.Deduped)
		}
		b.WriteByte('\n')
		if it.Summary != "" {
			fmt.Fprintf(&b, "   - %s\n", trimRunes(it.Summary, 220))
		}
	}
	if len(r.Items) == 0 {
		b.WriteString("No usable items.\n")
	}
	return b.String()
}

func sortedSourceNames(statuses map[string]sourceStatus) []string {
	keys := make([]string, 0, len(statuses))
	for key := range statuses {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cleanText(value string) string {
	value = html.UnescapeString(value)
	value = htmlTagRegexp.ReplaceAllString(value, " ")
	return compactSpaces(value)
}

func compactSpaces(value string) string { return strings.Join(strings.Fields(value), " ") }

func dateOnly(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	layouts := []string{time.RFC3339, time.RFC1123Z, time.RFC1123, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	if len(value) >= 10 && dateRegexp.MatchString(value[:10]) {
		return value[:10]
	}
	return value
}

func trimRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func fallbackTitle(title string, text string) string {
	if title != "" {
		return title
	}
	return trimRunes(cleanText(text), 90)
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			switch v := value.(type) {
			case float64:
				return int(v)
			case int:
				return v
			case string:
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func repoFromAPIURL(value string) string {
	u, err := url.Parse(value)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "repos" {
		return parts[1] + "/" + parts[2]
	}
	return ""
}

func maybeRepo(value string) bool {
	return repoRegexp.MatchString(strings.TrimSpace(value))
}

func hashText(value string) string {
	sum := sha1.Sum([]byte(value))
	return fmt.Sprintf("%x", sum[:8])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
