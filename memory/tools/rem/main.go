package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// candidateLine is a parsed capture line from ai/*.md.
type candidateLine struct {
	Text string // fact text, provenance suffix stripped
	Src  string // provenance, e.g. "claude", "codex"; empty if none
	File string // path of the ai/ file it came from
	Day  string // YYYY-MM-DD derived from filename
}

var candidateRe = regexp.MustCompile(`^- candidate: (.*?)(?: \(via ([^)]+)\))?$`)

// normalize lowercases, strips punctuation, and collapses whitespace.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9+]+`).ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func knowledgeDir() string {
	if d := os.Getenv("KNOWLEDGE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fatal("user home", err, "")
	}
	return filepath.Join(home, "Workspace", "knowledge")
}

func today() string { return time.Now().Format("2006-01-02") }

// cmdAdd appends `- candidate: <fact>` to ai/YYYY-MM-DD.md, skipping facts already
// captured as candidates in any ai/ file.
func cmdAdd(args []string) error {
	src := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-src" && i+1 < len(args) {
			src = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	fact := strings.TrimSpace(strings.Join(rest, " "))
	if fact == "" {
		return fmt.Errorf("usage: rem add [-src harness] \"fact\"")
	}

	dir := filepath.Join(knowledgeDir(), "ai")
	existing, err := loadCandidates(dir)
	if err != nil {
		return err
	}
	fp := normalize(fact)
	for _, c := range existing {
		if normalize(c.Text) == fp {
			fmt.Printf("rem add: already captured on %s: %s\n", c.Day, c.Text)
			return nil
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, today()+".md")
	fresh := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fresh = true
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if fresh {
		if _, err := fmt.Fprintf(f, "# %s\n\n", today()); err != nil {
			return err
		}
	}
	line := "- candidate: " + fact
	if src != "" {
		line += " (via " + src + ")"
	}
	if _, err := fmt.Fprintln(f, line); err != nil {
		return err
	}
	fmt.Printf("rem add: captured to %s\n", path)
	return nil
}

// loadCandidates parses every `- candidate:` line across ai/*.md.
func loadCandidates(dir string) ([]candidateLine, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []candidateLine
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, " \t\r")
			m := candidateRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			out = append(out, candidateLine{
				Text: strings.TrimSpace(m[1]),
				Src:  m[2],
				File: filepath.Join(dir, name),
				Day:  strings.TrimSuffix(name, ".md"),
			})
		}
	}
	return out, nil
}

// cmdSearch passes a query through to memsearch.
func cmdSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: rem search \"query\"")
	}
	bin, err := exec.LookPath("memsearch")
	if err != nil {
		return fmt.Errorf("memsearch not found: %w", err)
	}
	cmd := exec.Command(bin, append([]string{"search"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// cmdSync wraps the guarded knowledge-sync binary.
func cmdSync() error {
	bin := os.Getenv("REM_SYNC_BIN")
	if bin == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		bin = filepath.Join(home, ".local", "bin", "knowledge-sync")
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("knowledge-sync not found at %s: %w", bin, err)
	}
	cmd := exec.Command(bin)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
