package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type memsearchOptions struct {
	VaultDir string
}

func parseMemsearchFlags(args []string) (memsearchOptions, error) {
	fs := flag.NewFlagSet("memsearch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts memsearchOptions
	fs.StringVar(&opts.VaultDir, "vault", "~/Workspace/knowledge", "Root directory for the knowledge vault")

	if err := fs.Parse(args); err != nil {
		return memsearchOptions{}, err
	}
	return opts, nil
}

func runMemsearch(args []string) error {
	if len(args) == 0 {
		printMemsearchUsage()
		return fmt.Errorf("missing memsearch subcommand")
	}

	switch args[0] {
	case "setup":
		opts, err := parseMemsearchFlags(args[1:])
		if err != nil {
			return err
		}
		return runMemsearchSetup(opts)
	case "status":
		return runMemsearchStatus()
	case "-h", "--help", "help":
		printMemsearchUsage()
		return nil
	default:
		printMemsearchUsage()
		return fmt.Errorf("unknown memsearch subcommand %q", args[0])
	}
}

func runMemsearchSetup(opts memsearchOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	vaultDir := expandPath(opts.VaultDir, home)
	aiDir := filepath.Join(vaultDir, "ai")
	notesDir := filepath.Join(vaultDir, "notes")
	profileDir := filepath.Join(vaultDir, "profile")
	stateDir := filepath.Join(home, ".memsearch", "state")
	confPath := filepath.Join(home, ".agents", "memsearch.conf")

	fmt.Println("dotagents memsearch setup")
	fmt.Printf("vault: %s\n\n", vaultDir)

	// 1. Check memsearch is installed
	msPath, err := exec.LookPath("memsearch")
	if err != nil {
		fmt.Println("memsearch: not found on PATH")
		fmt.Println("  install: uv tool install memsearch")
		return fmt.Errorf("memsearch not installed")
	}
	fmt.Printf("memsearch: %s\n", msPath)

	// 2. Create vault directories
	for _, dir := range []string{aiDir, notesDir, profileDir, stateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	fmt.Printf("vault dirs: created\n")

	// 3. Git init vault if not already
	gitDir := filepath.Join(vaultDir, ".git")
	if !hasDir(gitDir) {
		cmd := exec.Command("git", "init", vaultDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git init vault: %w", err)
		}
	} else {
		fmt.Printf("vault git: already initialized\n")
	}

	// 4. Write config
	conf := fmt.Sprintf(`# memsearch configuration (written by dotagents memsearch setup)
# Source this file from memory hooks to get portable paths.
MEMSEARCH_VAULT_DIR="%s"
MEMSEARCH_AI_DIR="%s"
MEMSEARCH_NOTES_DIR="%s"
MEMSEARCH_PROFILE_DIR="%s"
MEMSEARCH_STATE_DIR="%s"
MEMSEARCH_COLLECTION="ai"
`, vaultDir, aiDir, notesDir, profileDir, stateDir)

	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Printf("config: %s\n", confPath)

	// 5. Report hook path
	repoRoot, _, _, _, err := loadContext(runOptions{})
	if err != nil {
		return fmt.Errorf("load context: %w", err)
	}
	hookSrc := filepath.Join(repoRoot, "memory", "hooks", "session-end.sh")
	if !hasFile(hookSrc) {
		fmt.Printf("memory hooks: not found at %s (skipping)\n", hookSrc)
	} else {
		fmt.Printf("memory hooks: %s\n", filepath.Dir(hookSrc))
	}

	fmt.Println("\ndone. Memory hooks will read config from ~/.agents/memsearch.conf")
	return nil
}

func runMemsearchStatus() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	confPath := filepath.Join(home, ".agents", "memsearch.conf")
	fmt.Println("dotagents memsearch status")
	fmt.Println()

	// Check memsearch binary
	msPath, err := exec.LookPath("memsearch")
	if err != nil {
		fmt.Println("memsearch binary: not found")
	} else {
		fmt.Printf("memsearch binary: %s\n", msPath)
	}

	// Check config
	data, err := os.ReadFile(confPath)
	if err != nil {
		fmt.Printf("config: not found (%s)\n", confPath)
		fmt.Println("\nrun 'dotagents memsearch setup' to configure")
		return nil
	}

	fmt.Printf("config: %s\n", confPath)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fmt.Printf("  %s\n", line)
	}

	return nil
}

func printMemsearchUsage() {
	fmt.Println("dotagents memsearch - manage knowledge vault and memsearch integration")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dotagents memsearch setup  [--vault ~/Workspace/knowledge]  Bootstrap vault + config")
	fmt.Println("  dotagents memsearch status                                  Show current configuration")
}
