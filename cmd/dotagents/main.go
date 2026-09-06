package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type config struct {
	Version        int                   `yaml:"version"`
	Agents         []agentConfig         `yaml:"agents"`
	MCPServers     []mcpServerConfig     `yaml:"mcp_servers"`
	ExternalSkills []externalSkillSource `yaml:"external_skills"`
	Hooks          []hookConfig          `yaml:"hooks,omitempty"`
	// ContextNoteTokens is the estimated skill-listing token threshold above
	// which `dotagents doctor` prints a soft context advisory note. Absent
	// (nil) uses contextNoteTokensDefault; 0 (or negative) disables the note.
	ContextNoteTokens *int `yaml:"context_note_tokens,omitempty"`
}

type externalSkillSource struct {
	URL         string   `yaml:"url"`
	SkillDir    string   `yaml:"skill_dir,omitempty"`
	SkillDirs   []string `yaml:"skill_dirs,omitempty"`
	Branch      string   `yaml:"branch"`
	Skills      []string `yaml:"skills,omitempty"`
	Materialize bool     `yaml:"materialize,omitempty"`
	MCP         bool     `yaml:"mcp,omitempty"`
	MCPAgents   []string `yaml:"mcp_agents,omitempty"`
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
	Name            string
	SkillRoot       string
	AgentRoot       string
	ExpectedSkills  map[string]string
	Detected        bool
	RootPath        string
	RootExpected    string
	RootActual      string
	RootState       string
	Managed         []string
	ManagedAgent    []string
	ManagedMCP      []string
	ManagedHook     []string
	Drifted         []string
	DriftedAgent    []string
	DriftedMCP      []string
	DriftedHook     []string
	Missing         []string
	MissingAgent    []string
	MissingMCP      []string
	MissingHook     []string
	UnsupportedHook []string
	Conflicts       []string
	StaleManaged    []string
	External        []string
	Adds            []string
	AddsAgent       []string
	AddsMCP         []string
	AddsHook        []string
	Updates         []string
	UpdatesAgent    []string
	UpdatesMCP      []string
	UpdatesHook     []string
	Removes         []string
	RemovesAgent    []string
	Synced          bool
}

func isDetected(agent agentConfig) bool {
	if agent.Detect == "" {
		return true
	}
	executable, err := exec.LookPath(agent.Detect)
	if err != nil {
		return false
	}
	if harness := harnessFor(agent.Name); harness != nil && harness.Detect != nil {
		return harness.Detect(executable)
	}
	return true
}

type runOptions struct {
	ConfigPath     string
	Agents         string
	Pull           bool
	E2E            bool
	SkipPackageAge bool
	MemoryTier     string
	JSONOutput     bool
	DryRun         bool
	AssumeYes      bool
	Stdin          io.Reader
	Stdout         io.Writer
	// ConfirmRemovals makes sync preview per-harness removals and role
	// overwrites and ask before applying them. Set by setup-driven syncs.
	ConfirmRemovals bool
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
	case "setup":
		return runSetupCommand(args[1:])
	case "status":
		return runStatusCommand(args[1:])
	case "sync":
		return runSyncCommand(args[1:])
	case "doctor":
		return runDoctorCommand(args[1:])
	case "view":
		return runView(args[1:])
	case "skill":
		return runSkillCommand(args[1:])
	case "mcp":
		return runMCP(args[1:])
	case "cron":
		opts, err := parseCronFlags(args[1:])
		if err != nil {
			return err
		}
		return runCron(opts)
	case "pull":
		printRenameNotice("pull", "sync --pull")
		opts, err := parseSubcommandFlags("pull", args[1:])
		if err != nil {
			return err
		}
		return runPull(opts)
	case "deps":
		return runDeprecatedDeps(args[1:])
	case "memsearch":
		return runDeprecatedMemsearch(args[1:])
	case "skillify":
		printRenameNotice("skillify", "skill new")
		return runSkillCommand(append([]string{"new"}, args[1:]...))
	case "render":
		printRenameNotice("render", "sync")
		opts, err := parseSubcommandFlags("render", args[1:])
		if err != nil {
			return err
		}
		return runRender(opts)
	case "audit":
		printRenameNotice("audit", "doctor")
		opts, err := parseSubcommandFlags("audit", args[1:])
		if err != nil {
			return err
		}
		return runAudit(opts)
	case "external":
		if len(args) > 1 && args[1] == "list" {
			printRenameNotice("external list", "status")
		} else if len(args) > 1 && args[1] == "update" {
			printRenameNotice("external update", "skill update")
		} else {
			printRenameNotice("external", "status or skill update")
		}
		return runExternal(args[1:])
	case "promote":
		printRenameNotice("promote", "skill promote")
		return runSkillCommand(append([]string{"promote"}, args[1:]...))
	case "dogfood":
		printRenameNotice("dogfood", "doctor --e2e")
		opts, err := parseSubcommandFlags("dogfood", args[1:])
		if err != nil {
			return err
		}
		return runDogfood(opts)
	case "help":
		if len(args) == 1 {
			printUsage()
			return nil
		}
		if len(args) == 2 && args[1] == "--all" {
			printAllUsage()
			return nil
		}
		return errors.New("usage: dotagents help [--all]")
	case "-h", "--help":
		if len(args) != 1 {
			return errors.New("usage: dotagents help [--all]")
		}
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runSetupCommand(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "memsearch":
			return runMemsearch(append([]string{"setup"}, args[1:]...))
		}
	}
	opts, err := parseSetupFlags(args)
	if err != nil {
		return err
	}
	return runSetup(opts)
}

func runStatusCommand(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "memsearch":
			printRenameNotice("status memsearch", "status")
			return runMemsearch(append([]string{"status"}, args[1:]...))
		}
	}
	opts, err := parseSubcommandFlags("status", args)
	if err != nil {
		return err
	}
	return runStatus(opts)
}

func runSyncCommand(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "pull":
			printRenameNotice("sync pull", "sync --pull")
			opts, err := parseSubcommandFlags("sync pull", args[1:])
			if err != nil {
				return err
			}
			return runPull(opts)
		case "deps":
			opts, err := parseDepsFlags("sync deps", args[1:])
			if err != nil {
				return err
			}
			return runDepsUpdate(opts)
		case "render":
			printRenameNotice("sync render", "sync")
			opts, err := parseSubcommandFlags("sync render", args[1:])
			if err != nil {
				return err
			}
			return runRender(opts)
		}
	}
	opts, err := parseSyncFlags(args)
	if err != nil {
		return err
	}
	if opts.Pull {
		opts.Pull = false
		return runPull(opts)
	}
	return runSync(opts)
}

func runDoctorCommand(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "audit":
			opts, err := parseSubcommandFlags("doctor audit", args[1:])
			if err != nil {
				return err
			}
			return runAudit(opts)
		case "deps":
			opts, err := parseDepsFlags("doctor deps", args[1:])
			if err != nil {
				return err
			}
			return runDepsCheck(opts)
		case "dogfood":
			printRenameNotice("doctor dogfood", "doctor --e2e")
			opts, err := parseSubcommandFlags("doctor dogfood", args[1:])
			if err != nil {
				return err
			}
			return runDogfood(opts)
		}
	}
	opts, err := parseDoctorFlags(args)
	if err != nil {
		return err
	}
	if opts.E2E {
		opts.E2E = false
		return runDogfood(opts)
	}
	return runDoctor(opts)
}

func runSkillCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("skill requires subcommand: new, update, promote")
	}
	switch args[0] {
	case "new":
		return runSkillify(args[1:])
	case "update":
		return runExternalUpdate(args[1:])
	case "promote":
		return runPromote(args[1:])
	case "external":
		if len(args) > 1 && args[1] == "list" {
			printRenameNotice("skill external list", "status")
		} else if len(args) > 1 && args[1] == "update" {
			printRenameNotice("skill external update", "skill update")
		} else {
			printRenameNotice("skill external", "status or skill update")
		}
		return runExternal(args[1:])
	default:
		return fmt.Errorf("unknown skill subcommand %q", args[0])
	}
}

func runDeprecatedDeps(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "check":
			printRenameNotice("deps check", "doctor deps")
			return runDoctorCommand(append([]string{"deps"}, args[1:]...))
		case "update":
			printRenameNotice("deps update", "sync deps")
			return runSyncCommand(append([]string{"deps"}, args[1:]...))
		}
	}
	printRenameNotice("deps", "doctor deps or sync deps")
	return runDeps(args)
}

func runDeprecatedMemsearch(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "setup":
			printRenameNotice("memsearch setup", "setup memsearch")
			return runSetupCommand(append([]string{"memsearch"}, args[1:]...))
		case "status":
			printRenameNotice("memsearch status", "status memsearch")
			return runStatusCommand(append([]string{"memsearch"}, args[1:]...))
		}
	}
	printRenameNotice("memsearch", "setup memsearch or status memsearch")
	return runMemsearch(args)
}

func printRenameNotice(oldCommand string, newCommand string) {
	fmt.Fprintf(os.Stderr, "dotagents: %q was renamed to %q\n", oldCommand, newCommand)
}

func parseSubcommandFlags(name string, args []string) (runOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts runOptions
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names to use for this run")
	fs.BoolVar(&opts.SkipPackageAge, "skip-package-age", false, "Skip external package publish-age checks")

	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	if fs.NArg() != 0 {
		return runOptions{}, fmt.Errorf("%s does not accept positional arguments", name)
	}

	return opts, nil
}

func parseSetupFlags(args []string) (runOptions, error) {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts runOptions
	opts.MemoryTier = memoryTierBasic
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names to use for this run")
	fs.StringVar(&opts.MemoryTier, "memory", memoryTierBasic, "Memory tier: off, basic, or memsearch")
	fs.BoolVar(&opts.JSONOutput, "json", false, "Emit detection result as JSON and exit")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "Show detected import candidates and exit without changes (overrides --yes)")
	fs.BoolVar(&opts.AssumeYes, "yes", false, "Import all detected items without prompting")
	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	if fs.NArg() != 0 {
		return runOptions{}, errors.New("setup does not accept positional arguments")
	}
	return opts, nil
}

func parseSyncFlags(args []string) (runOptions, error) {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts runOptions
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names to use for this run")
	fs.BoolVar(&opts.Pull, "pull", false, "Pull the repo before syncing")
	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	if fs.NArg() != 0 {
		return runOptions{}, errors.New("sync does not accept positional arguments")
	}
	return opts, nil
}

func parseDoctorFlags(args []string) (runOptions, error) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts runOptions
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to dotagents YAML config")
	fs.StringVar(&opts.Agents, "agents", "", "Comma-separated agent names to use for this run")
	fs.BoolVar(&opts.E2E, "e2e", false, "Run sync, status, and doctor end to end")
	fs.BoolVar(&opts.SkipPackageAge, "skip-package-age", false, "Skip external package publish-age checks")
	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}
	if fs.NArg() != 0 {
		return runOptions{}, errors.New("doctor does not accept positional arguments")
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
	fs.BoolVar(&opts.Deps, "deps", false, "Install dependency maintenance cron instead of auto-pull")
	fs.StringVar(&opts.Interval, "interval", cronIntervalDefault, "Pull interval: 5m, 15m, 30m, 1h, 6h, 12h, daily, weekly")

	if err := fs.Parse(args); err != nil {
		return cronOptions{}, err
	}
	return opts, nil
}

func printUsage() {
	fmt.Println("dotagents - manage shared skills and MCP config across coding agents")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup    Set up this machine and sync configured harnesses")
	fmt.Println("  status   Show harness, external lock, and memsearch state")
	fmt.Println("  sync     Regenerate committed artifacts and reconcile harnesses")
	fmt.Println("  doctor   Check pins, dependencies, and local health")
	fmt.Println()
	fmt.Println("Command groups:")
	fmt.Println("  skill    Create, update, and promote skills")
	fmt.Println("  mcp      Manage MCP servers")
	fmt.Println()
	fmt.Println("Run \"dotagents help --all\" for flags, maintenance commands, and compatibility aliases.")
}

func printAllUsage() {
	printUsage()
	fmt.Println()
	fmt.Println("Canonical forms:")
	fmt.Println("  dotagents setup [--memory off|basic|memsearch] [--agents ...] [--yes] [--dry-run] [--json]")
	fmt.Println("  dotagents status [--agents ...]")
	fmt.Println("  dotagents sync [--pull] [--agents ...]")
	fmt.Println("  dotagents doctor [--e2e] [--agents ...]")
	fmt.Println("  dotagents view [hk serve flags: --port N, --host ADDR, --no-token]")
	fmt.Println("  dotagents skill new <name> [--description ...]")
	fmt.Println("  dotagents skill update [name ...]")
	fmt.Println("  dotagents skill promote <name-or-path> [--dry-run]")
	fmt.Println("  dotagents mcp <list|add|import|remove> [options]")
	fmt.Println()
	fmt.Println("Maintenance and compatibility aliases:")
	fmt.Println("  dotagents cron [--interval 30m|--deps|--remove]")
	fmt.Println("  dotagents deps <check|update> [options]")
	fmt.Println("  dotagents memsearch <setup|status> [options]")
	fmt.Println("  dotagents pull [options]")
	fmt.Println("  dotagents render [options]")
	fmt.Println("  dotagents audit [options]")
	fmt.Println("  dotagents external <list|update> [name ...]")
	fmt.Println("  dotagents skillify <name> [options]")
	fmt.Println("  dotagents promote <name-or-path> [--dry-run]")
	fmt.Println("  dotagents dogfood [options]")
}
