package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultDashboardAddr = "127.0.0.1:8765"

var openBrowser = openBrowserURL

type dashboardOptions struct {
	Addr           string
	Access         string
	NoOpen         bool
	EnableSessions bool
}

type dashboardHealth struct {
	OK              bool   `json:"ok"`
	Repo            string `json:"repo"`
	SessionsEnabled bool   `json:"sessions_enabled"`
}

type dashboardSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Command     string `json:"command"`
}

type dashboardAgent struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	Model       string   `json:"model"`
	Effort      string   `json:"effort"`
	Tools       []string `json:"tools,omitempty"`
}

type dashboardMCP struct {
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	Agents  []string `json:"agents,omitempty"`
	Path    string   `json:"path"`
}

type dashboardHook struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type dashboardExample struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Command     string `json:"command,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
}

type dashboardExamplesResponse struct {
	Examples []dashboardExample `json:"examples"`
	Warnings []string           `json:"warnings,omitempty"`
}

type dashboardOverview struct {
	Repo             string                 `json:"repo"`
	Branch           string                 `json:"branch,omitempty"`
	Commit           string                 `json:"commit,omitempty"`
	AheadOriginMain  int                    `json:"ahead_origin_main"`
	BehindOriginMain int                    `json:"behind_origin_main"`
	AgentsTarget     string                 `json:"agents_target"`
	Agents           []dashboardAgentStatus `json:"agents"`
	Sync             dashboardSyncSummary   `json:"sync"`
	MCPCount         int                    `json:"mcp_count"`
	HookCount        int                    `json:"hook_count"`
	SessionsEnabled  bool                   `json:"sessions_enabled"`
	Commands         []string               `json:"commands"`
	Warnings         []string               `json:"warnings,omitempty"`
}

type dashboardAgentStatus struct {
	Name      string `json:"name"`
	Detected  bool   `json:"detected"`
	Synced    bool   `json:"synced"`
	SkillRoot string `json:"skill_root"`
	AgentRoot string `json:"agent_root,omitempty"`
}

type dashboardSyncSummary struct {
	Managed   int `json:"managed"`
	Missing   int `json:"missing"`
	Drifted   int `json:"drifted"`
	Conflicts int `json:"conflicts"`
	MCP       int `json:"mcp"`
}

type dashboardFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func runDashboard(args []string) error {
	opts, err := parseDashboardFlags(args)
	if err != nil {
		return err
	}

	repoRoot, _, err := findRoots()
	if err != nil {
		return err
	}

	handler := newDashboardHandler(repoRoot, opts)
	listener, err := listenDashboard(opts.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	url := "http://" + listener.Addr().String()
	fmt.Printf("dotagents dashboard: %s\n", url)
	if opts.Access == "ssh" {
		host, port := dashboardForwardTarget(listener.Addr())
		fmt.Printf("ssh access: ssh -L 127.0.0.1:%s:%s:%s <ssh-host>\n", port, host, port)
	}

	if !opts.NoOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "open browser: %v\n", err)
		}
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func parseDashboardFlags(args []string) (dashboardOptions, error) {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	opts := dashboardOptions{Addr: defaultDashboardAddr, Access: "local"}
	fs.StringVar(&opts.Addr, "addr", opts.Addr, "Local address to bind, for example 127.0.0.1:0")
	fs.StringVar(&opts.Access, "access", opts.Access, "Access mode: local or ssh")
	fs.BoolVar(&opts.NoOpen, "no-open", false, "Do not open a browser")
	fs.BoolVar(&opts.EnableSessions, "enable-sessions", false, "Enable allowlisted terminal sessions")

	if err := fs.Parse(args); err != nil {
		return dashboardOptions{}, err
	}
	if fs.NArg() != 0 {
		return dashboardOptions{}, errors.New("dashboard does not accept positional arguments")
	}

	opts.Access = strings.ToLower(strings.TrimSpace(opts.Access))
	if opts.Access == "" {
		opts.Access = "local"
	}
	if opts.Access != "local" && opts.Access != "ssh" {
		return dashboardOptions{}, fmt.Errorf("unsupported dashboard access mode %q", opts.Access)
	}

	addr, err := normalizeDashboardAddr(opts.Addr)
	if err != nil {
		return dashboardOptions{}, err
	}
	opts.Addr = addr
	return opts, nil
}

func listenDashboard(addr string) (net.Listener, error) {
	normalized, err := normalizeDashboardAddr(addr)
	if err != nil {
		return nil, err
	}
	return net.Listen("tcp", normalized)
}

func dashboardForwardTarget(addr net.Addr) (string, string) {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return "127.0.0.1", ""
	}
	host := "127.0.0.1"
	if tcpAddr.IP != nil && tcpAddr.IP.To4() == nil {
		host = "[" + tcpAddr.IP.String() + "]"
	} else if tcpAddr.IP != nil && !tcpAddr.IP.IsUnspecified() {
		host = tcpAddr.IP.String()
	}
	return host, strconv.Itoa(tcpAddr.Port)
}

func normalizeDashboardAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultDashboardAddr
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid dashboard addr %q: %w", addr, err)
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("invalid dashboard addr %q: missing port", addr)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid dashboard addr %q: invalid port", addr)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if !isLoopbackHost(host) {
		return "", fmt.Errorf("dashboard addr must bind to localhost, got %q", host)
	}
	return net.JoinHostPort(host, port), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newDashboardHandler(repoRoot string, opts dashboardOptions) http.Handler {
	mux := http.NewServeMux()
	assetsDir := filepath.Join(repoRoot, "experimental", "dotagents-ui", "assets")
	health := dashboardHealth{
		OK:              true,
		Repo:            repoRoot,
		SessionsEnabled: opts.EnableSessions,
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(assetsDir, "index.html"))
	})
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(health); err != nil {
			log.Printf("encode dashboard health: %v", err)
		}
	})
	mux.HandleFunc("/api/overview", getJSON(func() (dashboardOverview, error) {
		return collectDashboardOverview(repoRoot, opts)
	}))
	mux.HandleFunc("/api/skills", getJSON(func() ([]dashboardSkill, error) {
		return collectDashboardSkills(repoRoot)
	}))
	mux.HandleFunc("/api/agents", getJSON(func() ([]dashboardAgent, error) {
		return collectDashboardAgents(repoRoot)
	}))
	mux.HandleFunc("/api/mcps", getJSON(func() ([]dashboardMCP, error) {
		return collectDashboardMCPs(repoRoot)
	}))
	mux.HandleFunc("/api/hooks", getJSON(func() ([]dashboardHook, error) {
		return collectDashboardHooks(repoRoot)
	}))
	mux.HandleFunc("/api/examples", getJSON(func() (dashboardExamplesResponse, error) {
		return collectDashboardExamples(repoRoot)
	}))

	return mux
}

func getJSON[T any](fn func() (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		value, err := fn()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := json.NewEncoder(w).Encode(value); err != nil {
			log.Printf("encode dashboard response: %v", err)
		}
	}
}

func collectDashboardOverview(repoRoot string, opts dashboardOptions) (dashboardOverview, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return dashboardOverview{}, fmt.Errorf("resolve home: %w", err)
	}

	overview := dashboardOverview{
		Repo:            repoRoot,
		AgentsTarget:    filepath.Join(home, ".agents"),
		Agents:          []dashboardAgentStatus{},
		SessionsEnabled: opts.EnableSessions,
		Commands: []string{
			"dotagents status",
			"dotagents sync",
			"dotagents doctor --skip-package-age",
			"dotagents dogfood",
		},
	}
	overview.Branch, _ = gitOutput(repoRoot, "branch", "--show-current")
	overview.Commit, _ = gitOutput(repoRoot, "rev-parse", "--short", "HEAD")
	overview.AheadOriginMain, overview.BehindOriginMain = gitAheadBehindOriginMain(repoRoot)

	cfg, err := loadConfig(filepath.Join(repoRoot, "skills", "dotagents"), home, "")
	if err != nil {
		overview.Warnings = append(overview.Warnings, err.Error())
		return overview, nil
	}
	selected, err := selectAgents(cfg, "")
	if err != nil {
		overview.Warnings = append(overview.Warnings, err.Error())
		return overview, nil
	}
	expected, err := expectedSkills(repoRoot, home)
	if err != nil {
		overview.Warnings = append(overview.Warnings, err.Error())
		return overview, nil
	}
	reports, err := inspectAgents(selected, expected, repoRoot, home, cfg)
	if err != nil {
		overview.Warnings = append(overview.Warnings, err.Error())
		return overview, nil
	}
	for _, report := range reports {
		overview.Agents = append(overview.Agents, dashboardAgentStatus{
			Name:      report.Name,
			Detected:  report.Detected,
			Synced:    report.Synced,
			SkillRoot: report.SkillRoot,
			AgentRoot: report.AgentRoot,
		})
		overview.Sync.Managed += len(report.Managed) + len(report.ManagedAgent)
		overview.Sync.Missing += len(report.Missing) + len(report.MissingAgent)
		overview.Sync.Drifted += len(report.Drifted) + len(report.DriftedAgent)
		overview.Sync.Conflicts += len(report.Conflicts)
		overview.Sync.MCP += len(report.ManagedMCP) + len(report.MissingMCP) + len(report.DriftedMCP)
	}
	overview.MCPCount = len(cfg.MCPServers)
	hooks, err := collectDashboardHooks(repoRoot)
	if err != nil {
		overview.Warnings = append(overview.Warnings, err.Error())
	} else {
		overview.HookCount = len(hooks)
	}
	return overview, nil
}

func collectDashboardSkills(repoRoot string) ([]dashboardSkill, error) {
	skillsDir := filepath.Join(repoRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []dashboardSkill{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", skillsDir, err)
	}

	skills := []dashboardSkill{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if !hasFile(path) {
			continue
		}
		frontmatter, err := readSkillFrontmatter(path)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(frontmatter.Name)
		if name == "" {
			name = entry.Name()
		}
		skills = append(skills, dashboardSkill{
			Name:        name,
			Description: strings.TrimSpace(frontmatter.Description),
			Path:        relPath(repoRoot, path),
			Command:     "$" + name,
		})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

func collectDashboardAgents(repoRoot string) ([]dashboardAgent, error) {
	roles, err := loadAgentRoles(repoRoot)
	if err != nil {
		return nil, err
	}
	agents := []dashboardAgent{}
	for _, role := range roles {
		agents = append(agents, dashboardAgent{
			Name:        role.Name,
			Description: role.Description,
			Path:        relPath(repoRoot, role.Source),
			Model:       role.Model,
			Effort:      role.Effort,
			Tools:       append([]string{}, role.Tools...),
		})
	}
	return agents, nil
}

func collectDashboardMCPs(repoRoot string) ([]dashboardMCP, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home: %w", err)
	}
	cfg, err := loadConfig(filepath.Join(repoRoot, "skills", "dotagents"), home, "")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []dashboardMCP{}, nil
		}
		return nil, err
	}
	mcps := []dashboardMCP{}
	for _, server := range cfg.MCPServers {
		mcps = append(mcps, dashboardMCP{
			Name:    server.Name,
			Enabled: server.Enabled,
			Command: server.Command,
			Args:    append([]string{}, server.Args...),
			Env:     sortedKeys(server.Env),
			Agents:  append([]string{}, server.Agents...),
			Path:    relPath(repoRoot, filepath.Join(repoRoot, "skills", "dotagents", "dotagents.yaml")),
		})
	}
	sort.Slice(mcps, func(i, j int) bool { return mcps[i].Name < mcps[j].Name })
	return mcps, nil
}

func collectDashboardHooks(repoRoot string) ([]dashboardHook, error) {
	hooksDir := filepath.Join(repoRoot, "memory", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []dashboardHook{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", hooksDir, err)
	}
	hooks := []dashboardHook{}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		hooks = append(hooks, dashboardHook{
			Name: entry.Name(),
			Path: relPath(repoRoot, filepath.Join(hooksDir, entry.Name())),
		})
	}
	sort.Slice(hooks, func(i, j int) bool { return hooks[i].Name < hooks[j].Name })
	return hooks, nil
}

func collectDashboardExamples(repoRoot string) (dashboardExamplesResponse, error) {
	examplesDir := filepath.Join(repoRoot, "experimental", "dotagents-ui", "examples")
	entries, err := os.ReadDir(examplesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return dashboardExamplesResponse{Examples: []dashboardExample{}}, nil
	}
	if err != nil {
		return dashboardExamplesResponse{}, fmt.Errorf("read %s: %w", examplesDir, err)
	}

	response := dashboardExamplesResponse{Examples: []dashboardExample{}}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(examplesDir, entry.Name())
		example, err := readDashboardExample(repoRoot, path)
		if err != nil {
			response.Warnings = append(response.Warnings, err.Error())
			continue
		}
		if example.Name != "" {
			response.Examples = append(response.Examples, example)
		}
	}
	sort.Slice(response.Examples, func(i, j int) bool { return response.Examples[i].Name < response.Examples[j].Name })
	sort.Strings(response.Warnings)
	return response, nil
}

func readDashboardExample(repoRoot string, path string) (dashboardExample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dashboardExample{}, fmt.Errorf("read %s: %w", path, err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	example := dashboardExample{Name: name, Title: name, Path: relPath(repoRoot, path)}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		text := strings.TrimSpace(string(data))
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				example.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		example.Description = firstParagraph(text)
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &example); err != nil {
			return dashboardExample{}, fmt.Errorf("parse %s: %w", path, err)
		}
		if strings.TrimSpace(example.Name) == "" {
			example.Name = name
		}
		if strings.TrimSpace(example.Title) == "" {
			example.Title = example.Name
		}
		example.Path = relPath(repoRoot, path)
	default:
		return dashboardExample{}, nil
	}
	return example, nil
}

func readSkillFrontmatter(path string) (dashboardFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dashboardFrontmatter{}, fmt.Errorf("read %s: %w", path, err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return dashboardFrontmatter{}, nil
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return dashboardFrontmatter{}, nil
	}
	var frontmatter dashboardFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &frontmatter); err != nil {
		return dashboardFrontmatter{}, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	return frontmatter, nil
}

func firstParagraph(markdown string) string {
	for _, block := range strings.Split(markdown, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "#") {
			continue
		}
		return block
	}
	return ""
}

func gitOutput(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitAheadBehindOriginMain(repoRoot string) (int, int) {
	out, err := gitOutput(repoRoot, "rev-list", "--left-right", "--count", "HEAD...origin/main")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0
	}
	ahead, _ := strconv.Atoi(fields[0])
	behind, _ := strconv.Atoi(fields[1])
	return ahead, behind
}

func relPath(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func openBrowserURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
