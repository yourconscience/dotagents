package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config types

type TitleFilter struct {
	Positive []string `yaml:"positive"`
	Negative []string `yaml:"negative"`
}

type Company struct {
	Name       string `yaml:"name"`
	CareersURL string `yaml:"careers_url"`
	Enabled    bool   `yaml:"enabled"`
}

type Config struct {
	TitleFilter      TitleFilter `yaml:"title_filter"`
	TrackedCompanies []Company   `yaml:"tracked_companies"`
}

// Opportunity tracker types

type Opportunity struct {
	Company string `yaml:"company"`
	Role    string `yaml:"role"`
}

// Scanned job result

type Job struct {
	Company  string
	Title    string
	Location string
	URL      string
}

// JSON output type

type JSONJob struct {
	Company  string `json:"company"`
	Title    string `json:"title"`
	Location string `json:"location"`
	URL      string `json:"url"`
}

// Per-company scan result

type scanResult struct {
	company   string
	jobs      []Job
	total     int
	filtered  int
	err       error
}

func main() {
	defaultConfig := findDefault("data/portals.yml")
	defaultTracker := findDefault("data/opportunities.yaml")

	configPath := flag.String("config", defaultConfig, "path to portals.yml")
	trackerPath := flag.String("tracker", defaultTracker, "path to opportunities.yaml")
	companyFilter := flag.String("company", "", "scan only this company (by name)")
	dryRun := flag.Bool("dry-run", false, "show what would be found without outputting")
	asJSON := flag.Bool("json", false, "output as JSON array")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fatalf("failed to load config: %v", err)
	}

	existingKeys := loadExistingKeys(*trackerPath)

	companies := cfg.TrackedCompanies
	if *companyFilter != "" {
		companies = filterByName(companies, *companyFilter)
		if len(companies) == 0 {
			fatalf("no company named %q found in config", *companyFilter)
		}
	}

	// Only scan enabled companies
	var enabled []Company
	for _, c := range companies {
		if c.Enabled {
			enabled = append(enabled, c)
		}
	}

	results := scanAll(enabled, cfg.TitleFilter, 10)

	// Aggregate
	var newJobs []Job
	totalFound := 0
	totalFiltered := 0
	totalDupes := 0
	var errors []string

	for _, r := range results {
		if r.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", r.company, r.err))
			continue
		}
		totalFound += r.total
		totalFiltered += r.filtered
		for _, j := range r.jobs {
			key := dedupKey(j.Company, j.Title)
			if existingKeys[key] {
				totalDupes++
				continue
			}
			newJobs = append(newJobs, j)
		}
	}

	if *dryRun {
		fmt.Printf("Dry run: %d new roles would be shown\n", len(newJobs))
		for _, j := range newJobs {
			fmt.Printf("  %s | %s | %s\n", j.Company, j.Title, j.Location)
		}
		return
	}

	if *asJSON {
		out := make([]JSONJob, len(newJobs))
		for i, j := range newJobs {
			out[i] = JSONJob{Company: j.Company, Title: j.Title, Location: j.Location, URL: j.URL}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return
	}

	// Human-readable output
	fmt.Printf("Portal Scan - %s\n", time.Now().Format("2006-01-02"))
	fmt.Println(strings.Repeat("\u2501", 24))
	fmt.Printf("Companies scanned:     %d\n", len(enabled))
	fmt.Printf("Total jobs found:      %d\n", totalFound)
	fmt.Printf("Filtered by title:     %d removed\n", totalFiltered)
	fmt.Printf("Duplicates:            %d skipped\n", totalDupes)
	fmt.Printf("New roles found:       %d\n", len(newJobs))

	if len(newJobs) > 0 {
		fmt.Println("\nNew roles:")
		for _, j := range newJobs {
			fmt.Printf("  + %s | %s | %s\n", j.Company, j.Title, j.Location)
			fmt.Printf("    %s\n", j.URL)
		}
	}

	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("  ! %s\n", e)
		}
	}
}

func scanAll(companies []Company, filter TitleFilter, maxConcurrent int) []scanResult {
	results := make([]scanResult, len(companies))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, c := range companies {
		wg.Add(1)
		go func(idx int, company Company) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = scanCompany(company, filter)
		}(i, c)
	}

	wg.Wait()
	return results
}

func scanCompany(c Company, filter TitleFilter) scanResult {
	platform, slug, err := detectPlatform(c.CareersURL)
	if err != nil {
		return scanResult{company: c.Name, err: err}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var jobs []Job

	switch platform {
	case "ashby":
		jobs, err = fetchAshby(client, slug, c.Name)
	case "greenhouse":
		jobs, err = fetchGreenhouse(client, slug, c.Name)
	case "lever":
		jobs, err = fetchLever(client, slug, c.Name)
	default:
		err = fmt.Errorf("unknown platform for URL %s", c.CareersURL)
	}

	if err != nil {
		return scanResult{company: c.Name, err: err}
	}

	total := len(jobs)
	var passed []Job
	filtered := 0
	for _, j := range jobs {
		if matchesFilter(j.Title, filter) {
			passed = append(passed, j)
		} else {
			filtered++
		}
	}

	return scanResult{
		company:  c.Name,
		jobs:     passed,
		total:    total,
		filtered: filtered,
	}
}

// Platform detection

var (
	reAshby       = regexp.MustCompile(`(?i)^https?://jobs\.ashbyhq\.com/([^/?#]+)`)
	reGreenhouse  = regexp.MustCompile(`(?i)^https?://job-boards(?:\.eu)?\.greenhouse\.io/([^/?#]+)`)
	reLever       = regexp.MustCompile(`(?i)^https?://jobs\.lever\.co/([^/?#]+)`)
)

func detectPlatform(careersURL string) (platform, slug string, err error) {
	if m := reAshby.FindStringSubmatch(careersURL); m != nil {
		return "ashby", m[1], nil
	}
	if m := reGreenhouse.FindStringSubmatch(careersURL); m != nil {
		return "greenhouse", m[1], nil
	}
	if m := reLever.FindStringSubmatch(careersURL); m != nil {
		return "lever", m[1], nil
	}
	return "", "", fmt.Errorf("unrecognized careers URL: %s", careersURL)
}

// Greenhouse

type greenhouseResponse struct {
	Jobs []struct {
		Title    string `json:"title"`
		URL      string `json:"absolute_url"`
		Location struct {
			Name string `json:"name"`
		} `json:"location"`
	} `json:"jobs"`
}

func fetchGreenhouse(client *http.Client, slug, company string) ([]Job, error) {
	url := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", slug)
	body, err := getJSON(client, url)
	if err != nil {
		return nil, err
	}
	var resp greenhouseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}
	jobs := make([]Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		jobs = append(jobs, Job{
			Company:  company,
			Title:    j.Title,
			Location: j.Location.Name,
			URL:      j.URL,
		})
	}
	return jobs, nil
}

// Ashby

type ashbyResponse struct {
	Jobs []struct {
		Title    string `json:"title"`
		JobURL   string `json:"jobUrl"`
		Location string `json:"location"`
	} `json:"jobs"`
}

func fetchAshby(client *http.Client, slug, company string) ([]Job, error) {
	url := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s?includeCompensation=true", slug)
	body, err := getJSON(client, url)
	if err != nil {
		return nil, err
	}
	var resp ashbyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}
	jobs := make([]Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		jobs = append(jobs, Job{
			Company:  company,
			Title:    j.Title,
			Location: j.Location,
			URL:      j.JobURL,
		})
	}
	return jobs, nil
}

// Lever

type leverPosting struct {
	Text      string `json:"text"`
	HostedURL string `json:"hostedUrl"`
	Categories struct {
		Location string `json:"location"`
	} `json:"categories"`
}

func fetchLever(client *http.Client, slug, company string) ([]Job, error) {
	url := fmt.Sprintf("https://api.lever.co/v0/postings/%s", slug)
	body, err := getJSON(client, url)
	if err != nil {
		return nil, err
	}
	var postings []leverPosting
	if err := json.Unmarshal(body, &postings); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}
	jobs := make([]Job, 0, len(postings))
	for _, p := range postings {
		jobs = append(jobs, Job{
			Company:  company,
			Title:    p.Text,
			Location: p.Categories.Location,
			URL:      p.HostedURL,
		})
	}
	return jobs, nil
}

// HTTP helper

func getJSON(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "portals-scan/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// Title filtering

func matchesFilter(title string, filter TitleFilter) bool {
	lower := strings.ToLower(title)

	// Must match at least one positive keyword
	hasPositive := false
	for _, kw := range filter.Positive {
		if strings.Contains(lower, strings.ToLower(kw)) {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		return false
	}

	// Must not match any negative keyword
	for _, kw := range filter.Negative {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return false
		}
	}

	return true
}

// Dedup

func dedupKey(company, role string) string {
	return strings.ToLower(company) + "::" + strings.ToLower(role)
}

func loadExistingKeys(path string) map[string]bool {
	keys := make(map[string]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		// File may not exist yet; that's fine
		return keys
	}

	// opportunities.yaml is a list of maps; we only need company and role
	var entries []map[string]interface{}
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return keys
	}

	for _, e := range entries {
		company, _ := e["company"].(string)
		role, _ := e["role"].(string)
		if company != "" && role != "" {
			keys[dedupKey(company, role)] = true
		}
	}
	return keys
}

// Config loading

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Helpers

func findDefault(relPath string) string {
	exe, err := os.Executable()
	if err == nil {
		// Resolve relative to skill root (two levels up from tools/portals-scan/)
		skillRoot := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
		candidate := filepath.Join(skillRoot, relPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Fallback: relative to cwd
	return relPath
}

func filterByName(companies []Company, name string) []Company {
	lower := strings.ToLower(name)
	var out []Company
	for _, c := range companies {
		if strings.ToLower(c.Name) == lower {
			out = append(out, c)
		}
	}
	return out
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
