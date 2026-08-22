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

// ---- shared text helpers -------------------------------------------------

var syncHeadingRe = regexp.MustCompile(`(?m)^## Sync [^\n]+\n\n`)

type syncEntry struct {
	Section string // full "## Sync ..." section text, trimmed
	Body    string
}

func splitSyncEntries(text string) (preamble string, entries []syncEntry) {
	matches := syncHeadingRe.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return strings.TrimRight(text, "\n"), nil
	}
	preamble = strings.TrimRight(text[:matches[0][0]], "\n")
	for i, m := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := strings.TrimSpace(text[m[0]:end])
		entries = append(entries, syncEntry{Section: section, Body: strings.TrimSpace(text[m[1]:end])})
	}
	return preamble, entries
}

// collapseExactDuplicates keeps the first occurrence of each normalized body.
func collapseExactDuplicates(text string) (out string, dropped int) {
	preamble, entries := splitSyncEntries(text)
	seen := map[string]bool{}
	var kept []string
	for _, e := range entries {
		fp := normalize(e.Body)
		if seen[fp] {
			dropped++
			continue
		}
		seen[fp] = true
		kept = append(kept, e.Section)
	}
	out = preamble
	if len(kept) > 0 {
		out += "\n\n" + strings.Join(kept, "\n\n")
	}
	return out + "\n", dropped
}

// ---- candidate clustering -------------------------------------------------

var negationWords = map[string]bool{
	"not": true, "never": true, "dont": true, "avoid": true, "stop": true, "no": true,
}

type cluster struct {
	Members     []candidateLine
	Distinct    int // distinct source files
	Negative    int
	Positive    int
	Fingerprint string
}

func tokens(s string) map[string]bool {
	t := map[string]bool{}
	for _, w := range strings.Fields(normalize(s)) {
		t[w] = true
	}
	return t
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}

const similarityThreshold = 0.6

// clusterCandidates groups candidates by lexical similarity; single-pass greedy,
// deterministic (candidates sorted by day then text).
func clusterCandidates(cands []candidateLine) []cluster {
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].Day != cands[j].Day {
			return cands[i].Day < cands[j].Day
		}
		return cands[i].Text < cands[j].Text
	})
	var clusters []cluster
	for _, c := range cands {
		tk := tokens(c.Text)
		neg := hasNegation(c.Text)
		placed := false
		for ci := range clusters {
			rep := clusters[ci].Members[0]
			if jaccard(tk, tokens(rep.Text)) >= similarityThreshold {
				clusters[ci].Members = append(clusters[ci].Members, c)
				if neg {
					clusters[ci].Negative++
				} else {
					clusters[ci].Positive++
				}
				placed = true
				break
			}
		}
		if !placed {
			nc := cluster{Members: []candidateLine{c}, Fingerprint: normalize(c.Text)}
			if neg {
				nc.Negative = 1
			} else {
				nc.Positive = 1
			}
			clusters = append(clusters, nc)
		}
	}
	for ci := range clusters {
		files := map[string]bool{}
		for _, m := range clusters[ci].Members {
			files[m.File] = true
		}
		clusters[ci].Distinct = len(files)
	}
	return clusters
}

func hasNegation(s string) bool {
	for w := range tokens(s) {
		if negationWords[w] {
			return true
		}
	}
	return false
}

// suggestTarget routes a cluster by simple cue words. Ambiguous -> needs review.
func suggestTarget(cl cluster) string {
	text := ""
	for _, m := range cl.Members {
		text += " " + m.Text
	}
	t := tokens(text)
	personal := false
	operational := false
	for w := range t {
		switch w {
		case "prefer", "prefers", "favorite", "my", "i":
			personal = true
		case "always", "repo", "repos", "commit", "commits", "agents", "harness", "skill", "skills":
			operational = true
		}
	}
	switch {
	case personal && !operational:
		return "profile/USER.md"
	case operational && !personal:
		return "~/.agents/AGENTS.md"
	default:
		return "needs review"
	}
}

// ---- dream command --------------------------------------------------------

func cmdDream(args []string) error {
	apply := false
	var rest []string
	for _, a := range args {
		if strings.TrimLeft(a, "-") == "apply" {
			apply = true
			continue
		}
		rest = append(rest, a)
	}
	if apply {
		return dreamApply(rest)
	}
	return dreamReport(rest)
}

func dreamReport([]string) error {
	root := knowledgeDir()
	cands, err := loadCandidates(filepath.Join(root, "ai"))
	if err != nil {
		return err
	}
	knowledgePath := filepath.Join(root, "sessions", "knowledge.md")
	var dupCount, totalSections, distinctSections int
	if data, err := os.ReadFile(knowledgePath); err == nil {
		_, entries := splitSyncEntries(string(data))
		totalSections = len(entries)
		seen := map[string]bool{}
		for _, e := range entries {
			if seen[normalize(e.Body)] {
				continue
			}
			seen[normalize(e.Body)] = true
		}
		distinctSections = len(seen)
		dupCount = totalSections - distinctSections
	}

	clusters := clusterCandidates(cands)
	repeaters := []cluster{}
	for _, cl := range clusters {
		if cl.Distinct >= 2 {
			repeaters = append(repeaters, cl)
		}
	}
	conflicts := []cluster{}
	for _, cl := range repeaters {
		if cl.Positive > 0 && cl.Negative > 0 {
			conflicts = append(conflicts, cl)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# rem dream report %s\n\n", today())
	fmt.Fprintf(&b, "- candidates scanned: %d across %d ai/ files\n", len(cands), countAIDirs(root))
	fmt.Fprintf(&b, "- knowledge.md sync sections: %d (%d exact duplicates collapsible)\n", totalSections, dupCount)
	fmt.Fprintf(&b, "- repeated clusters (>=2 distinct days/files): %d\n", len(repeaters))
	fmt.Fprintf(&b, "- conflicts (mixed polarity, not promotable): %d\n\n", len(conflicts))

	if len(repeaters) == 0 {
		b.WriteString("No promotion candidates this pass.\n")
	}
	for _, cl := range repeaters {
		status := "propose"
		target := suggestTarget(cl)
		if cl.Positive > 0 && cl.Negative > 0 {
			status = "conflict"
			target = "needs review"
		}
		fmt.Fprintf(&b, "## Candidate: %s\n\n", truncate(cl.Members[0].Text, 80))
		fmt.Fprintf(&b, "- target: %s\n- status: %s\n- distinct sources: %d (positive %d / negative %d)\n- proposed bullet: %s\n- evidence:\n",
			target, status, cl.Distinct, cl.Positive, cl.Negative, cl.Members[len(cl.Members)-1].Text)
		seenFile := map[string]bool{}
		for _, m := range cl.Members {
			if seenFile[m.File] {
				continue
			}
			seenFile[m.File] = true
			fmt.Fprintf(&b, "  - %s (%s): %s\n", m.Day, srcOr(m.Src), m.Text)
		}
		b.WriteString("\n")
	}
	if dupCount > 0 {
		fmt.Fprintf(&b, "Run `rem dream --apply` to collapse the %d exact duplicate sync sections.\n", dupCount)
	}

	outPath := filepath.Join(root, "reviews", "rem-dream-"+today()+".md")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("rem dream: %d candidates, %d repeaters, %d conflicts, %d collapsible dups\nreport: %s\n",
		len(cands), len(repeaters), len(conflicts), dupCount, outPath)
	return nil
}

func srcOr(s string) string {
	if s == "" {
		return "manual"
	}
	return s
}

func countAIDirs(root string) int {
	entries, err := os.ReadDir(filepath.Join(root, "ai"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			n++
		}
	}
	return n
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// dreamApply performs the only unattended-safe write: collapsing byte-normalized
// duplicate sync sections in sessions/knowledge.md, with backup + git commit.
func dreamApply([]string) error {
	root := knowledgeDir()
	path := filepath.Join(root, "sessions", "knowledge.md")

	// Refuse on dirty tree or non-main checkout.
	if out, err := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput(); err != nil {
		return fmt.Errorf("git status: %w (%s)", err, out)
	} else if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("refusing: knowledge repo has uncommitted changes")
	}
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("git branch: %w (%s)", err, out)
	} else if strings.TrimSpace(string(out)) != "main" {
		return fmt.Errorf("refusing: worktree is on %s, expected main", strings.TrimSpace(string(out)))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	collapsed, dropped := collapseExactDuplicates(string(data))
	if dropped == 0 {
		fmt.Println("rem dream --apply: nothing to collapse")
		return nil
	}

	// Keep backups invisible to the dirty-tree guard without touching tracked files.
	if excl, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude")); err == nil &&
		!strings.Contains(string(excl), "knowledge.md.bak-") {
		f, ferr := os.OpenFile(filepath.Join(root, ".git", "info", "exclude"), os.O_APPEND|os.O_WRONLY, 0o644)
		if ferr == nil {
			fmt.Fprintln(f, "sessions/knowledge.md.bak-*")
			f.Close()
		}
	}
	backup := path + ".bak-" + time.Now().UTC().Format("20060102T150405Z")
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(collapsed), 0o644); err != nil {
		return err
	}
	git := func(args ...string) error {
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w (%s)", args[0], err, out)
		}
		return nil
	}
	if err := git("add", "sessions/knowledge.md"); err != nil {
		return err
	}
	msg := fmt.Sprintf("rem dream: collapse %d duplicate sync sections (backup %s)",
		dropped, filepath.Base(backup))
	if out, err := exec.Command("git", "-C", root, "commit", "-m", msg).CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "nothing to commit") {
			return fmt.Errorf("git commit: %w (%s)", err, out)
		}
	}
	fmt.Printf("rem dream --apply: collapsed %d duplicate sections; backup %s\n", dropped, backup)
	return nil
}
