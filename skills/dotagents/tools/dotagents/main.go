package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

type config struct {
	Version int           `yaml:"version"`
	Agents  []agentConfig `yaml:"agents"`
}

type agentConfig struct {
	Name      string `yaml:"name"`
	Enabled   bool   `yaml:"enabled"`
	SkillRoot string `yaml:"skill_root"`
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
	Managed      []string
	Drifted      []string
	Missing      []string
	Conflicts    []string
	StaleManaged []string
	External     []string
	Adds         []string
	Updates      []string
	Removes      []string
	Synced       bool
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

func printUsage() {
	fmt.Println("dotagents")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./skills/dotagents/tools/dotagents status [--config path] [--agents codex,claude-code]")
	fmt.Println("  go run ./skills/dotagents/tools/dotagents sync   [--config path] [--agents codex,claude-code]")
}
