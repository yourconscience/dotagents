package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type sourceConfig struct {
	Name      string `yaml:"name"`
	Enabled   *bool  `yaml:"enabled,omitempty"`
	Preferred string `yaml:"preferred,omitempty"`
}

type methodType string

const (
	methodCLI      methodType = "cli"
	methodMCP      methodType = "mcp"
	methodAPI      methodType = "api"
	methodFallback methodType = "fallback"
)

type sourceMethodDef struct {
	Name     string
	Type     methodType
	Priority int
	Detect   string // binary name for LookPath
	Check    string // shell one-liner for deeper check (run via sh -c)
	MCP      string // MCP server name to look up in config
	Auth     string // credential location (display)
	Setup    string // one-line setup instruction
}

type sourceDef struct {
	Name      string
	Desc      string
	Methods   []sourceMethodDef
	DefaultOn bool
	ToSRisk   string // "", "high"
}

type methodStatus struct {
	Name      string     `json:"name"`
	Type      methodType `json:"type"`
	Priority  int        `json:"priority"`
	Available bool       `json:"available"`
	Reason    string     `json:"reason,omitempty"`
}

type sourceStatus struct {
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Best    string         `json:"best"`
	Methods []methodStatus `json:"methods"`
}

var sourceRegistry = []sourceDef{
	{
		Name: "x.com", Desc: "X.com / Twitter", DefaultOn: true, ToSRisk: "high",
		Methods: []sourceMethodDef{
			{Name: "x-api-v2", Type: methodAPI, Priority: 0, Check: "test -s ~/.x-api/credentials.json", Auth: "~/.x-api/credentials.json", Setup: "register OAuth consumer app at developer.x.com"},
			{Name: "x-cli", Type: methodCLI, Priority: 1, Detect: "x-cli", Auth: "~/.x-cli/credentials.json", Setup: "x-cli auth login"},
			{Name: "websearch", Type: methodFallback, Priority: 99},
		},
	},
	{
		Name: "reddit", Desc: "Reddit", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "rdt-cli", Type: methodCLI, Priority: 1, Detect: "rdt", Setup: "uv tool install rdt-cli"},
			{Name: "pullpush", Type: methodAPI, Priority: 2},
			{Name: "websearch", Type: methodFallback, Priority: 99},
		},
	},
	{
		Name: "hacker-news", Desc: "Hacker News", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "algolia", Type: methodAPI, Priority: 1},
		},
	},
	{
		Name: "discord", Desc: "Discord", DefaultOn: false, ToSRisk: "high",
		Methods: []sourceMethodDef{
			{Name: "discord-cli", Type: methodCLI, Priority: 1, Detect: "discord", Check: "test -n \"$DISCORD_TOKEN\"", Auth: "$DISCORD_TOKEN env var", Setup: "uv tool install kabi-discord-cli"},
			{Name: "websearch", Type: methodFallback, Priority: 99},
		},
	},
	{
		Name: "github", Desc: "GitHub", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "gh", Type: methodCLI, Priority: 1, Detect: "gh", Setup: "gh auth login"},
		},
	},
	{
		Name: "linkedin", Desc: "LinkedIn", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "linkedin-mcp", Type: methodMCP, Priority: 1, MCP: "linkedin", Setup: "uvx linkedin-scraper-mcp --login"},
			{Name: "websearch", Type: methodFallback, Priority: 99},
		},
	},
	{
		Name: "telegram", Desc: "Telegram", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "tg", Type: methodCLI, Priority: 1, Detect: "tg", Auth: "~/.local/share/dotagents/telegram-readonly/telegram.session", Setup: "cd ~/.agents/mcp/telegram-readonly && uv run python login.py"},
			{Name: "telegram-mcp", Type: methodMCP, Priority: 2, MCP: "telegram-readonly"},
		},
	},
	{
		Name: "google", Desc: "Google Workspace", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "gws", Type: methodCLI, Priority: 1, Detect: "gws", Auth: "~/.config/gws/", Setup: "gws auth login"},
		},
	},
	{
		Name: "web-search", Desc: "Web search (general)", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "tavily-mcp", Type: methodMCP, Priority: 1, MCP: "tavily"},
			{Name: "native", Type: methodFallback, Priority: 99},
		},
	},
	{
		Name: "job-portals", Desc: "ATS portals (Greenhouse, Ashby, Lever)", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "portals-scan", Type: methodCLI, Priority: 1, Detect: "go", Check: "test -d ~/.agents/skills/jobs/tools/portals-scan"},
		},
	},
	{
		Name: "glassdoor", Desc: "Glassdoor / Levels.fyi", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "websearch", Type: methodFallback, Priority: 99},
		},
	},
	{
		Name: "hugging-face", Desc: "Hugging Face Hub", DefaultOn: true,
		Methods: []sourceMethodDef{
			{Name: "hf-api", Type: methodAPI, Priority: 1},
		},
	},
	{
		Name: "ra", Desc: "Resident Advisor", DefaultOn: false,
		Methods: nil,
	},
}

func checkMethodAvailability(m sourceMethodDef, cfg config, home string) (bool, string) {
	switch m.Type {
	case methodFallback:
		return true, ""
	case methodAPI:
		if m.Check == "" {
			return true, ""
		}
		return runShellCheck(m.Check, home)
	case methodCLI:
		if m.Detect != "" {
			if _, err := exec.LookPath(m.Detect); err != nil {
				return false, fmt.Sprintf("%s not on PATH", m.Detect)
			}
		}
		if m.Check != "" {
			return runShellCheck(m.Check, home)
		}
		return true, ""
	case methodMCP:
		if m.MCP == "" {
			return false, "no MCP server name"
		}
		for _, srv := range cfg.MCPServers {
			if srv.Name == m.MCP && srv.Enabled {
				return true, ""
			}
		}
		return false, fmt.Sprintf("MCP server %q not configured or disabled", m.MCP)
	}
	return false, "unknown method type"
}

func runShellCheck(cmd string, home string) (bool, string) {
	cmd = strings.ReplaceAll(cmd, "~", home)
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		reason := strings.TrimSpace(string(out))
		if reason == "" {
			if cmd != "" {
				reason = "check failed"
			}
		}
		return false, reason
	}
	return true, ""
}

func resolveSourceStatus(cfg config, home string) []sourceStatus {
	overrides := make(map[string]sourceConfig, len(cfg.Sources))
	for _, s := range cfg.Sources {
		overrides[s.Name] = s
	}

	var results []sourceStatus
	for _, def := range sourceRegistry {
		enabled := def.DefaultOn
		preferred := ""
		if ov, ok := overrides[def.Name]; ok {
			if ov.Enabled != nil {
				enabled = *ov.Enabled
			}
			preferred = ov.Preferred
		}

		ss := sourceStatus{
			Name:    def.Name,
			Enabled: enabled,
		}

		if !enabled || len(def.Methods) == 0 {
			results = append(results, ss)
			continue
		}

		bestPriority := 999
		for _, m := range def.Methods {
			avail, reason := checkMethodAvailability(m, cfg, home)
			ms := methodStatus{
				Name:      m.Name,
				Type:      m.Type,
				Priority:  m.Priority,
				Available: avail,
				Reason:    reason,
			}
			ss.Methods = append(ss.Methods, ms)

			if avail && m.Name == preferred {
				ss.Best = m.Name
				bestPriority = -1
			} else if avail && m.Priority < bestPriority && ss.Best == "" {
				ss.Best = m.Name
				bestPriority = m.Priority
			}
		}

		// If preferred was set but not available, still pick best available
		if ss.Best == "" {
			for _, ms := range ss.Methods {
				if ms.Available && ms.Priority < bestPriority {
					ss.Best = ms.Name
					bestPriority = ms.Priority
				}
			}
		}

		results = append(results, ss)
	}
	return results
}

func findSourceDef(name string) *sourceDef {
	for i := range sourceRegistry {
		if sourceRegistry[i].Name == name {
			return &sourceRegistry[i]
		}
	}
	return nil
}

func expandAuthPath(auth string, home string) string {
	if strings.HasPrefix(auth, "~/") {
		return filepath.Join(home, auth[2:])
	}
	return auth
}
