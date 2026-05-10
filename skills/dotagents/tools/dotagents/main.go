package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

type config struct {
	Version    int               `yaml:"version"`
	Agents     []agentConfig     `yaml:"agents"`
	MCPServers []mcpServerConfig `yaml:"mcp_servers"`
}

type agentConfig struct {
	Name      string `yaml:"name"`
	Enabled   bool   `yaml:"enabled"`
	SkillRoot string `yaml:"skill_root"`
	AgentRoot string `yaml:"agent_root,omitempty"`
	Detect    string `yaml:"detect,omitempty"`
}

type repoLinkReport struct {
	Path           string
	ExpectedTarget string
	ActualTarget   string
	State          string
}

type agentReport struct {
	Name         string
	SkillRoot    string
	AgentRoot    string
	Detected     bool
	RootPath     string
	RootExpected string
	RootActual   string
	RootState    string
	Managed      []string
	ManagedAgent []string
	ManagedMCP   []string
	Drifted      []string
	DriftedAgent []string
	DriftedMCP   []string
	Missing      []string
	MissingAgent []string
	MissingMCP   []string
	Conflicts    []string
	StaleManaged []string
	External     []string
	Adds         []string
	AddsAgent    []string
	AddsMCP      []string
	Updates      []string
	UpdatesAgent []string
	UpdatesMCP   []string
	Removes      []string
	Synced       bool
}

func isDetected(agent agentConfig) bool {
	if agent.Detect == "" {
		return true
	}
	_, err := exec.LookPath(agent.Detect)
	return err == nil
}

type runOptions struct {
	ConfigPath string
	Agents     string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("missing subcommand")
	}

	switch args[0] {
	case "status":
		opts, err := parseSubcommandFlags("status", args[1:])
		if err != nil {
			return err
		}
		return runStatus(opts)
	case "sync":
		opts, err := parseSubcommandFlags("sync", args[1:])
		if err != nil {
			return err
		}
		return runSync(opts)
	case "setup":
		opts, err := parseSubcommandFlags("setup", args[1:])
		if err != nil {
			return err
		}
		return runSetup(opts)
	case "pull":
		opts, err := parseSubcommandFlags("pull", args[1:])
		if err != nil {
			return err
		}
		return runPull(opts)
	case "cron":
		opts, err := parseCronFlags(args[1:])
		if err != nil {
			return err
		}
		return runCron(opts)
	case "memsearch":
		return runMemsearch(args[1:])
	case "mcp":
		return runMCP(args[1:])
	case "skillify":
		return runSkillify(args[1:])
	case "doctor":
		opts, err := parseSubcommandFlags("doctor", args[1:])
		if err != nil {
			return err
		}
		return runDoctor(opts)
	case "promote":
		return runPromote(args[1:])
	case "dogfood":
		opts, err := parseSubcommandFlags("dogfood", args[1:])
		if err != nil {
			return err
		}
		return runDogfood(opts)
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func parseSubcommandFlags(name string, args []string) (runOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts runOptions
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names to use for this run")

	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	if fs.NArg() != 0 {
		return runOptions{}, fmt.Errorf("%s does not accept positional arguments", name)
	}

	return opts, nil
}

func parseCronFlags(args []string) (cronOptions, error) {
	fs := flag.NewFlagSet("cron", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts cronOptions
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names")
	fs.BoolVar(&opts.Remove, "remove", false, "Remove the cron entry instead of installing")
	fs.StringVar(&opts.Interval, "interval", "30m", "Pull interval: 5m, 15m, 30m, 1h, 6h, 12h, daily")

	if err := fs.Parse(args); err != nil {
		return cronOptions{}, err
	}
	return opts, nil
}

func printUsage() {
	fmt.Println("dotagents - manage shared skills and MCP config across coding agents")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dotagents setup     [--agents ...]           First-time setup: symlink, patch configs, sync")
	fmt.Println("  dotagents status    [--agents ...]           Show skill/MCP sync state for detected agents")
	fmt.Println("  dotagents sync      [--agents ...]           Sync managed skills and MCP config to detected agents")
	fmt.Println("  dotagents pull      [--agents ...]           Git pull + sync (for cron use)")
	fmt.Println("  dotagents cron      [--interval 30m]         Install a crontab entry for auto-pull")
	fmt.Println("  dotagents cron      --remove                 Remove the crontab entry")
	fmt.Println("  dotagents memsearch setup [--vault ...]      Bootstrap knowledge vault + memsearch config")
	fmt.Println("  dotagents memsearch status                   Show memsearch configuration")
	fmt.Println("  dotagents mcp list                           List canonical managed MCP servers")
	fmt.Println("  dotagents mcp add <name> --command <cmd>     Add/update canonical managed MCP")
	fmt.Println("  dotagents mcp import <agent> <name>          Import native MCP into canonical config")
	fmt.Println("  dotagents mcp remove <name>                  Remove canonical managed MCP")
	fmt.Println("  dotagents skillify <name> [--description \"...\"]  Scaffold a new skill from template")
	fmt.Println("  dotagents promote <name-or-path> [--dry-run]   Promote a Hermes skill to dotagents + PR")
	fmt.Println("  dotagents doctor        [--agents ...]           Health audit: frontmatter, collisions, sizes")
	fmt.Println("  dotagents dogfood       [--agents ...]           End-to-end self-test: sync + status + doctor")
}
