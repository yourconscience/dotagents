// Build: cd ~/Workspace/dotagents/skills/techsearch/tools/repo_triage && go build -o repo_triage .
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxRepoConcurrency = 5
	topIssueLimit      = 7
	ghCommandTimeout   = 45 * time.Second
	maintainerReplyAge = 60
)

var frictionMatchers = []frictionMatcher{
	{Label: "uninstall", Pattern: regexp.MustCompile(`\buninstall\b`)},
	{Label: "install", Pattern: regexp.MustCompile(`\binstall\b`)},
	{Label: "setup", Pattern: regexp.MustCompile(`\bsetup\b`)},
	{Label: "broken", Pattern: regexp.MustCompile(`\bbroken\b`)},
	{Label: "crash", Pattern: regexp.MustCompile(`\bcrash(?:ed|es|ing)?\b`)},
	{Label: "data loss", Pattern: regexp.MustCompile(`\bdata loss\b`)},
	{Label: "abandoned", Pattern: regexp.MustCompile(`\babandoned\b`)},
	{Label: "no response", Pattern: regexp.MustCompile(`\bno response\b`)},
	{Label: "apple silicon", Pattern: regexp.MustCompile(`\bapple silicon\b`)},
	{Label: "m1", Pattern: regexp.MustCompile(`\bm1\b`)},
	{Label: "m2", Pattern: regexp.MustCompile(`\bm2\b`)},
}

var seriousFriction = map[string]bool{
	"abandoned":   true,
	"broken":      true,
	"crash":       true,
	"data loss":   true,
	"no response": true,
}

type frictionMatcher struct {
	Label   string
	Pattern *regexp.Regexp
}

type output struct {
	Mode   string      `json:"mode"`
	Query  string      `json:"query,omitempty"`
	Repos  []repoOut   `json:"repos"`
	Errors []repoError `json:"errors,omitempty"`
}

type repoOut struct {
	Slug                      string       `json:"slug"`
	URL                       string       `json:"url,omitempty"`
	Description               string       `json:"description,omitempty"`
	Stars                     int          `json:"stars,omitempty"`
	Archived                  bool         `json:"archived"`
	Fork                      bool         `json:"fork"`
	License                   string       `json:"license,omitempty"`
	LastPush                  string       `json:"last_push,omitempty"`
	LastCommitDate            string       `json:"last_commit_date,omitempty"`
	LastRelease               *releaseInfo `json:"last_release,omitempty"`
	OpenIssues                int          `json:"open_issues,omitempty"`
	CommitLast30d             bool         `json:"commit_last_30d,omitempty"`
	CommitLast60d             bool         `json:"commit_last_60d,omitempty"`
	CommitLast180d            bool         `json:"commit_last_180d,omitempty"`
	MaintainerRepliedRecently bool         `json:"maintainer_replied_recently,omitempty"`
	TopIssues                 []issueOut   `json:"top_issues,omitempty"`
	FrictionKeywords          []string     `json:"friction_keywords,omitempty"`
	SecurityPolicy            bool         `json:"security_policy,omitempty"`
	Vibe                      string       `json:"vibe,omitempty"`
	VibeReason                string       `json:"vibe_reason,omitempty"`
}

type releaseInfo struct {
	Tag  string `json:"tag"`
	Date string `json:"date"`
}

type issueOut struct {
	Title     string `json:"title"`
	Reactions int    `json:"reactions"`
	AgeDays   int    `json:"age_days"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet,omitempty"`
}

type repoError struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

type repoREST struct {
	FullName        string `json:"full_name"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	StargazersCount int    `json:"stargazers_count"`
	Archived        bool   `json:"archived"`
	Fork            bool   `json:"fork"`
	PushedAt        string `json:"pushed_at"`
	CreatedAt       string `json:"created_at"`
	OpenIssuesCount int    `json:"open_issues_count"`
	License         *struct {
		SPDXID string `json:"spdx_id"`
		Key    string `json:"key"`
		Name   string `json:"name"`
	} `json:"license"`
}

type repoDetailsGraphQL struct {
	Data struct {
		Repository *struct {
			DefaultBranchRef *struct {
				Name   string `json:"name"`
				Target *struct {
					CommittedDate string `json:"committedDate"`
				} `json:"target"`
			} `json:"defaultBranchRef"`
			LatestRelease *struct {
				TagName     string `json:"tagName"`
				PublishedAt string `json:"publishedAt"`
			} `json:"latestRelease"`
			SecurityRoot   any `json:"securityRoot"`
			SecurityGitHub any `json:"securityGitHub"`
			SecurityDocs   any `json:"securityDocs"`
		} `json:"repository"`
	} `json:"data"`
}

type issuesGraphQL struct {
	Data struct {
		Search struct {
			Nodes []struct {
				Title     string `json:"title"`
				URL       string `json:"url"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
				BodyText  string `json:"bodyText"`
				Reactions struct {
					TotalCount int `json:"totalCount"`
				} `json:"reactions"`
				Comments struct {
					Nodes []struct {
						CreatedAt         string `json:"createdAt"`
						AuthorAssociation string `json:"authorAssociation"`
						Author            *struct {
							Login string `json:"login"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
}

type discoveryRepo struct {
	FullName        string `json:"fullName"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	StargazersCount int    `json:"stargazersCount"`
	PushedAt        string `json:"pushedAt"`
	IsArchived      bool   `json:"isArchived"`
	URL             string `json:"url"`
	License         *struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"license"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var reposArg string
	var topic string
	var light bool

	flag.StringVar(&reposArg, "repos", "", "Comma-separated owner/repo list for triage mode")
	flag.StringVar(&topic, "topic", "", "Discovery topic for shortlist mode")
	flag.BoolVar(&light, "light", false, "Skip issue fetch in triage mode")
	flag.Parse()

	if strings.TrimSpace(reposArg) == "" && strings.TrimSpace(topic) == "" {
		fmt.Fprintln(os.Stderr, "repo_triage requires either --repos owner/a,owner/b or --topic \"query\"")
		return 1
	}
	if strings.TrimSpace(reposArg) != "" && strings.TrimSpace(topic) != "" {
		fmt.Fprintln(os.Stderr, "repo_triage accepts either --repos or --topic, not both")
		return 1
	}
	if light && strings.TrimSpace(reposArg) == "" {
		fmt.Fprintln(os.Stderr, "--light is only valid with --repos")
		return 1
	}

	if err := ensureGHReady(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	ctx := context.Background()
	var out output
	var err error
	if strings.TrimSpace(topic) != "" {
		out, err = runDiscovery(ctx, strings.TrimSpace(topic))
	} else {
		out, err = runTriage(ctx, splitRepoSlugs(reposArg), light)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
		return 1
	}
	return 0
}

func ensureGHReady() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("gh CLI is required on PATH for repo_triage")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ghCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	var stderr bytes.Buffer
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("gh auth status timed out - check GitHub CLI availability")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("gh CLI is not authenticated: %s", msg)
	}
	return nil
}

func runDiscovery(ctx context.Context, topic string) (output, error) {
	args := []string{
		"search", "repos", topic,
		"--json", "fullName,name,description,stargazersCount,pushedAt,isArchived,license,url",
		"--limit", "15",
	}
	raw, err := runGH(ctx, args...)
	if err != nil {
		return output{}, fmt.Errorf("discovery search failed: %w", err)
	}

	var repos []discoveryRepo
	if err := json.Unmarshal(raw, &repos); err != nil {
		return output{}, fmt.Errorf("failed to parse discovery search response: %w", err)
	}

	out := output{
		Mode:  "shortlist",
		Query: topic,
		Repos: make([]repoOut, 0, len(repos)),
	}
	for _, repo := range repos {
		item := repoOut{
			Slug:        firstNonEmpty(repo.FullName, repo.Name),
			URL:         repo.URL,
			Description: repo.Description,
			Stars:       repo.StargazersCount,
			Archived:    repo.IsArchived,
			License:     formatDiscoveryLicense(repo.License),
			LastPush:    repo.PushedAt,
		}
		item.Vibe, item.VibeReason = assignDiscoveryVibe(item)
		out.Repos = append(out.Repos, item)
	}
	return out, nil
}

func runTriage(ctx context.Context, slugs []string, light bool) (output, error) {
	out := output{
		Mode:  "triage",
		Query: strings.Join(slugs, ","),
		Repos: make([]repoOut, 0, len(slugs)),
	}
	if len(slugs) == 0 {
		return out, nil
	}

	now := time.Now().UTC()
	results := make([]repoOut, len(slugs))
	ok := make([]bool, len(slugs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxRepoConcurrency)

	for idx, slug := range slugs {
		idx := idx
		slug := slug
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repo, errs := triageOneRepo(ctx, slug, light, now)

			mu.Lock()
			defer mu.Unlock()
			if repo != nil {
				results[idx] = *repo
				ok[idx] = true
			}
			out.Errors = append(out.Errors, errs...)
		}()
	}

	wg.Wait()

	for idx := range results {
		if ok[idx] {
			out.Repos = append(out.Repos, results[idx])
		}
	}

	sort.Slice(out.Errors, func(i, j int) bool {
		if out.Errors[i].Slug == out.Errors[j].Slug {
			return out.Errors[i].Error < out.Errors[j].Error
		}
		return out.Errors[i].Slug < out.Errors[j].Slug
	})

	return out, nil
}

func triageOneRepo(ctx context.Context, slug string, light bool, now time.Time) (*repoOut, []repoError) {
	owner, name, err := splitOwnerRepo(slug)
	if err != nil {
		return nil, []repoError{{Slug: slug, Error: err.Error()}}
	}

	var errs []repoError
	base, err := fetchRepoREST(ctx, slug)
	if err != nil {
		return nil, []repoError{{Slug: slug, Error: err.Error()}}
	}

	repo := repoOut{
		Slug:        slug,
		URL:         base.HTMLURL,
		Description: base.Description,
		Stars:       base.StargazersCount,
		Archived:    base.Archived,
		Fork:        base.Fork,
		License:     formatRESTLicense(base.License),
		LastPush:    base.PushedAt,
		OpenIssues:  base.OpenIssuesCount,
	}

	details, err := fetchRepoDetails(ctx, owner, name)
	if err != nil {
		errs = append(errs, repoError{Slug: slug, Error: err.Error()})
	} else {
		if details.Data.Repository != nil {
			if details.Data.Repository.DefaultBranchRef != nil && details.Data.Repository.DefaultBranchRef.Target != nil {
				repo.LastCommitDate = details.Data.Repository.DefaultBranchRef.Target.CommittedDate
			}
			if details.Data.Repository.LatestRelease != nil {
				repo.LastRelease = &releaseInfo{
					Tag:  details.Data.Repository.LatestRelease.TagName,
					Date: details.Data.Repository.LatestRelease.PublishedAt,
				}
			}
			repo.SecurityPolicy = details.Data.Repository.SecurityRoot != nil ||
				details.Data.Repository.SecurityGitHub != nil ||
				details.Data.Repository.SecurityDocs != nil
		}
	}

	commitTime := parseTimestamp(repo.LastCommitDate)
	repo.CommitLast30d = isWithinDays(commitTime, now, 30)
	repo.CommitLast60d = isWithinDays(commitTime, now, 60)
	repo.CommitLast180d = isWithinDays(commitTime, now, 180)

	hasIssueData := false
	if !light {
		issues, issueErr := fetchTopIssues(ctx, slug)
		if issueErr != nil {
			errs = append(errs, repoError{Slug: slug, Error: issueErr.Error()})
		} else {
			hasIssueData = true
			repo.TopIssues = make([]issueOut, 0, len(issues.Data.Search.Nodes))
			keywordSet := make(map[string]bool)
			for _, issue := range issues.Data.Search.Nodes {
				repo.TopIssues = append(repo.TopIssues, issueOut{
					Title:     issue.Title,
					Reactions: issue.Reactions.TotalCount,
					AgeDays:   ageInDays(parseTimestamp(issue.CreatedAt), now),
					URL:       issue.URL,
					Snippet:   snippet(issue.BodyText, 120),
				})
				for _, keyword := range matchFrictionKeywords(issue.Title) {
					if !keywordSet[keyword] {
						repo.FrictionKeywords = append(repo.FrictionKeywords, keyword)
						keywordSet[keyword] = true
					}
				}
				if hasRecentMaintainerReply(issue, now) {
					repo.MaintainerRepliedRecently = true
				}
			}
		}
	}

	repo.Vibe, repo.VibeReason = assignTriageVibe(repo, hasIssueData, light, parseTimestamp(base.CreatedAt), now)
	return &repo, errs
}

func fetchRepoREST(ctx context.Context, slug string) (*repoREST, error) {
	raw, err := runGH(ctx, "api", "repos/"+slug)
	if err != nil {
		return nil, fmt.Errorf("repo metadata fetch failed: %w", err)
	}
	var repo repoREST
	if err := json.Unmarshal(raw, &repo); err != nil {
		return nil, fmt.Errorf("repo metadata parse failed: %w", err)
	}
	return &repo, nil
}

func fetchRepoDetails(ctx context.Context, owner, name string) (*repoDetailsGraphQL, error) {
	query := `
query($owner:String!,$name:String!){
  repository(owner:$owner,name:$name){
    defaultBranchRef{
      name
      target{
        ... on Commit {
          committedDate
        }
      }
    }
    latestRelease{
      tagName
      publishedAt
    }
    securityRoot: object(expression:"HEAD:SECURITY.md"){ __typename }
    securityGitHub: object(expression:"HEAD:.github/SECURITY.md"){ __typename }
    securityDocs: object(expression:"HEAD:docs/SECURITY.md"){ __typename }
  }
}`
	raw, err := runGH(ctx, "api", "graphql", "-f", "query="+query, "-F", "owner="+owner, "-F", "name="+name)
	if err != nil {
		return nil, fmt.Errorf("repo details fetch failed: %w", err)
	}
	var payload repoDetailsGraphQL
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("repo details parse failed: %w", err)
	}
	return &payload, nil
}

func fetchTopIssues(ctx context.Context, slug string) (*issuesGraphQL, error) {
	query := `
query($issueQuery:String!){
  search(type: ISSUE, query:$issueQuery, first:` + fmt.Sprintf("%d", topIssueLimit) + `){
    nodes {
      ... on Issue {
        title
        url
        createdAt
        updatedAt
        bodyText
        reactions {
          totalCount
        }
        comments(last:20){
          nodes {
            createdAt
            authorAssociation
            author {
              login
            }
          }
        }
      }
    }
  }
}`
	searchQuery := fmt.Sprintf("repo:%s is:issue is:open sort:reactions-desc", slug)
	raw, err := runGH(ctx, "api", "graphql", "-f", "query="+query, "-F", "issueQuery="+searchQuery)
	if err != nil {
		return nil, fmt.Errorf("top issue fetch failed: %w", err)
	}
	var payload issuesGraphQL
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("top issue parse failed: %w", err)
	}
	return &payload, nil
}

func runGH(ctx context.Context, args ...string) ([]byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, ghCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, "gh", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if callCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("gh %s timed out", strings.Join(args, " "))
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}

func splitRepoSlugs(input string) []string {
	parts := strings.Split(input, ",")
	seen := make(map[string]bool)
	slugs := make([]string, 0, len(parts))
	for _, part := range parts {
		slug := strings.TrimSpace(part)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		slugs = append(slugs, slug)
	}
	return slugs
}

func splitOwnerRepo(slug string) (string, string, error) {
	parts := strings.Split(slug, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo slug %q - expected owner/repo", slug)
	}
	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("invalid repo slug %q - expected owner/repo", slug)
	}
	return owner, name, nil
}

func formatRESTLicense(license *struct {
	SPDXID string `json:"spdx_id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
}) string {
	if license == nil {
		return ""
	}
	if license.SPDXID != "" && license.SPDXID != "NOASSERTION" {
		return license.SPDXID
	}
	return firstNonEmpty(license.Name, strings.ToUpper(license.Key))
}

func formatDiscoveryLicense(license *struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}) string {
	if license == nil {
		return ""
	}
	return firstNonEmpty(license.Name, strings.ToUpper(license.Key))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseTimestamp(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return ts.UTC()
}

func isWithinDays(ts, now time.Time, days int) bool {
	if ts.IsZero() {
		return false
	}
	return ts.After(now.AddDate(0, 0, -days))
}

func ageInDays(ts, now time.Time) int {
	if ts.IsZero() {
		return 0
	}
	if ts.After(now) {
		return 0
	}
	return int(now.Sub(ts).Hours() / 24)
}

func snippet(body string, limit int) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(body), " ")
	runes := []rune(compact)
	if len(runes) <= limit {
		return compact
	}
	return string(runes[:limit-3]) + "..."
}

func matchFrictionKeywords(title string) []string {
	lower := strings.ToLower(title)
	var matched []string
	for _, matcher := range frictionMatchers {
		if matcher.Pattern.MatchString(lower) {
			matched = append(matched, matcher.Label)
		}
	}
	return matched
}

func hasRecentMaintainerReply(issue struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	BodyText  string `json:"bodyText"`
	Reactions struct {
		TotalCount int `json:"totalCount"`
	} `json:"reactions"`
	Comments struct {
		Nodes []struct {
			CreatedAt         string `json:"createdAt"`
			AuthorAssociation string `json:"authorAssociation"`
			Author            *struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"nodes"`
	} `json:"comments"`
}, now time.Time) bool {
	cutoff := now.AddDate(0, 0, -maintainerReplyAge)
	for _, comment := range issue.Comments.Nodes {
		if !isMaintainerAssociation(comment.AuthorAssociation) {
			continue
		}
		if parseTimestamp(comment.CreatedAt).After(cutoff) {
			return true
		}
	}
	return false
}

func isMaintainerAssociation(assoc string) bool {
	switch assoc {
	case "COLLABORATOR", "MEMBER", "OWNER":
		return true
	default:
		return false
	}
}

func assignDiscoveryVibe(repo repoOut) (string, string) {
	lastPush := parseTimestamp(repo.LastPush)
	switch {
	case repo.Archived:
		return "avoid-for-now", "archived repo in discovery view"
	case isWithinDays(lastPush, time.Now().UTC(), 60) && repo.License != "":
		return "maybe", "active shortlist candidate - run full triage before picking"
	case isWithinDays(lastPush, time.Now().UTC(), 180):
		return "mixed-signals", "recent enough to shortlist, but metadata-only"
	default:
		return "mixed-signals", "metadata-only shortlist with soft maintenance signal"
	}
}

func assignTriageVibe(repo repoOut, hasIssueData, light bool, createdAt, now time.Time) (string, string) {
	hasLicense := strings.TrimSpace(repo.License) != ""
	releaseTime := time.Time{}
	if repo.LastRelease != nil {
		releaseTime = parseTimestamp(repo.LastRelease.Date)
	}
	recentRelease := isWithinDays(releaseTime, now, 180)
	frictionCount := len(repo.FrictionKeywords)
	severe := hasSeriousFriction(repo.FrictionKeywords)
	youngRepo := !createdAt.IsZero() && createdAt.After(now.AddDate(-1, 0, 0))

	switch {
	case repo.Archived:
		return "avoid-for-now", "archived repository"
	case !repo.CommitLast180d && !recentRelease && youngRepo:
		return "avoid-for-now", "young repo with little recent activity and no recent release"
	case !repo.CommitLast180d && !recentRelease && frictionCount >= 2:
		return "avoid-for-now", "stale maintenance picture plus visible user friction"
	case !hasIssueData:
		if light {
			if repo.CommitLast60d && !repo.Archived {
				return "mixed-signals", "metadata looks active - run full triage for issue surface"
			}
			if !repo.CommitLast180d && !recentRelease {
				return "maybe", "metadata-only pass and maintenance looks soft"
			}
			return "mixed-signals", "metadata-only triage"
		}
		if repo.CommitLast60d && hasLicense {
			return "mixed-signals", "active repo, but issue surface could not be fetched"
		}
		return "mixed-signals", "thin public signals - issue surface missing"
	case frictionCount >= 3 && (!repo.MaintainerRepliedRecently || severe):
		return "avoid-for-now", "popular but too many unresolved high-vote complaints"
	case frictionCount >= 2:
		if repo.MaintainerRepliedRecently {
			return "maybe", "active, but repeated friction reports still show up in top issues"
		}
		return "maybe", "active but repeated install or breakage complaints"
	case severe && !repo.MaintainerRepliedRecently:
		return "maybe", "a few serious issue smells without recent maintainer follow-up"
	case repo.CommitLast30d && hasLicense && !repo.Archived && frictionCount == 0:
		return "worth-trying", "active repo with license and a clean top-issue surface"
	case repo.CommitLast180d && (hasLicense || recentRelease || repo.SecurityPolicy):
		return "maybe", "maintenance signals look decent, but not clean enough to fast-track"
	case len(repo.TopIssues) == 0:
		return "mixed-signals", "thin public issue data"
	default:
		return "mixed-signals", "mixed maintenance and issue signals"
	}
}

func hasSeriousFriction(keywords []string) bool {
	for _, keyword := range keywords {
		if seriousFriction[keyword] {
			return true
		}
	}
	return false
}
