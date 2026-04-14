// memsearch-backfill scans historical Claude Code and Codex session transcripts,
// filters them, summarizes each survivor via claude -p haiku, writes staged
// markdown entries, and consolidates existing ai/ daily files.
//
// Subcommands:
//
//	scan         Walk session dirs, write manifest.jsonl + print stats.
//	filter       Apply age/turns/slash-command/dedup filters, write filtered.jsonl.
//	summarize    Iterate filtered.jsonl, call claude -p haiku per session, write staging.
//	consolidate  Group per-turn entries by session, call haiku per session, add daily
//	             digest at top of file. Operates on ai/*.md files in place.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type SessionRecord struct {
	Path           string `json:"path"`
	Agent          string `json:"agent"`
	SessionID      string `json:"session_id"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	TurnCount      int    `json:"turn_count"`
	AgeDays        int    `json:"age_days"`
	FirstUserMsg   string `json:"first_user_msg"`
	FirstUserHash  string `json:"first_user_hash"`
	FileSizeBytes  int64  `json:"file_size_bytes"`
}

const (
	BackfillDir = "Documents/backfill"
	MaxAgeDays  = 180
	MinTurns    = 4 // skip sessions with <4 turns (user wants >3)
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: memsearch-backfill <scan|filter|summarize|consolidate>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "scan":
		err = runScan()
	case "filter":
		err = runFilter()
	case "summarize":
		err = runSummarize()
	case "consolidate":
		err = runConsolidate(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand:", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, os.Args[1]+" failed:", err)
		os.Exit(1)
	}
}

func runScan() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	outDir := filepath.Join(home, BackfillDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	manifestPath := filepath.Join(outDir, "manifest.jsonl")
	mf, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer mf.Close()

	var records []SessionRecord
	now := time.Now()

	// --- Claude Code ---
	claudeRoot := filepath.Join(home, ".claude", "projects")
	claudeFiles, _ := collectFiles(claudeRoot, ".jsonl")
	for _, p := range claudeFiles {
		rec, err := parseClaudeSession(p, now)
		if err != nil || rec == nil {
			continue
		}
		records = append(records, *rec)
	}

	// --- Codex (new JSONL format) ---
	codexRoot := filepath.Join(home, ".codex", "sessions")
	codexFiles, _ := collectFiles(codexRoot, ".jsonl")
	for _, p := range codexFiles {
		rec, err := parseCodexSession(p, now)
		if err != nil || rec == nil {
			continue
		}
		records = append(records, *rec)
	}

	// --- Write manifest (all records, pre-filter) ---
	enc := json.NewEncoder(mf)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}

	// --- Stats ---
	printStats(records)
	fmt.Printf("\nmanifest: %s (%d records, pre-filter)\n", manifestPath, len(records))
	return nil
}

// collectFiles walks `root` and returns absolute paths with the given suffix.
func collectFiles(root, suffix string) ([]string, error) {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, suffix) {
			out = append(out, p)
		}
		return nil
	})
	return out, nil
}

// parseClaudeSession reads a Claude Code JSONL transcript and returns a SessionRecord.
// Returns (nil, nil) if the file has no real user turns (skipped, not an error).
func parseClaudeSession(p string, now time.Time) (*SessionRecord, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

	var firstMsg, sessionID, startTS, endTS string
	turnCount := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		if sid, ok := obj["sessionId"].(string); ok && sessionID == "" {
			sessionID = sid
		}
		if ts, ok := obj["timestamp"].(string); ok {
			if startTS == "" {
				startTS = ts
			}
			endTS = ts
		}
		if t, _ := obj["type"].(string); t == "user" {
			if obj["isMeta"] == true {
				continue
			}
			msg, _ := obj["message"].(map[string]any)
			if msg == nil {
				continue
			}
			content, ok := msg["content"].(string)
			if !ok || strings.TrimSpace(content) == "" {
				continue // skip tool_result or non-string content
			}
			turnCount++
			if firstMsg == "" {
				firstMsg = content
			}
		}
	}
	if turnCount == 0 || firstMsg == "" {
		return nil, nil
	}
	if sessionID == "" {
		// Fallback: use filename without .jsonl as session id
		sessionID = strings.TrimSuffix(filepath.Base(p), ".jsonl")
	}
	startT := parseTimeFallback(startTS, fi.ModTime())
	endT := parseTimeFallback(endTS, fi.ModTime())
	return &SessionRecord{
		Path:          p,
		Agent:         "claude-code",
		SessionID:     sessionID,
		StartTime:     startT.UTC().Format(time.RFC3339),
		EndTime:       endT.UTC().Format(time.RFC3339),
		TurnCount:     turnCount,
		AgeDays:       int(now.Sub(startT).Hours() / 24),
		FirstUserMsg:  truncate(firstMsg, 500),
		FirstUserHash: hashString(firstMsg),
		FileSizeBytes: fi.Size(),
	}, nil
}

// parseCodexSession reads a Codex "rollout-*.jsonl" file and returns a SessionRecord.
// Real user turns are events of type=event_msg with payload.type=user_message.
func parseCodexSession(p string, now time.Time) (*SessionRecord, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

	var firstMsg, sessionID, startTS, endTS string
	turnCount := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		if ts, ok := obj["timestamp"].(string); ok {
			if startTS == "" {
				startTS = ts
			}
			endTS = ts
		}
		t, _ := obj["type"].(string)
		payload, _ := obj["payload"].(map[string]any)
		if payload == nil {
			continue
		}
		if t == "session_meta" {
			if id, ok := payload["id"].(string); ok {
				sessionID = id
			}
			continue
		}
		if t == "event_msg" {
			if pt, _ := payload["type"].(string); pt == "user_message" {
				msg, _ := payload["message"].(string)
				msg = strings.TrimSpace(msg)
				if msg == "" {
					continue
				}
				// Skip synthetic environment-context frames that Codex injects at turn start.
				if strings.HasPrefix(msg, "<environment_context>") {
					continue
				}
				turnCount++
				if firstMsg == "" {
					firstMsg = msg
				}
			}
		}
	}
	if turnCount == 0 || firstMsg == "" {
		return nil, nil
	}
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(p), ".jsonl")
	}
	startT := parseTimeFallback(startTS, fi.ModTime())
	endT := parseTimeFallback(endTS, fi.ModTime())
	return &SessionRecord{
		Path:          p,
		Agent:         "codex",
		SessionID:     sessionID,
		StartTime:     startT.UTC().Format(time.RFC3339),
		EndTime:       endT.UTC().Format(time.RFC3339),
		TurnCount:     turnCount,
		AgeDays:       int(now.Sub(startT).Hours() / 24),
		FirstUserMsg:  truncate(firstMsg, 500),
		FirstUserHash: hashString(firstMsg),
		FileSizeBytes: fi.Size(),
	}, nil
}

func parseTimeFallback(ts string, fallback time.Time) time.Time {
	if ts == "" {
		return fallback
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, ts); err == nil {
			return t
		}
	}
	return fallback
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func printStats(records []SessionRecord) {
	total := len(records)
	byAgent := map[string]int{}
	var ageTooOld, tooShort, ageOK int
	for _, r := range records {
		byAgent[r.Agent]++
		if r.AgeDays > MaxAgeDays {
			ageTooOld++
		} else {
			ageOK++
			if r.TurnCount < MinTurns {
				tooShort++
			}
		}
	}

	fmt.Println("=== BACKFILL SCAN SUMMARY ===")
	fmt.Printf("total sessions found: %d\n", total)
	for k, v := range byAgent {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Printf("\nfilters:\n")
	fmt.Printf("  older than %d days: %d (would be skipped)\n", MaxAgeDays, ageTooOld)
	fmt.Printf("  within %d days: %d\n", MaxAgeDays, ageOK)
	fmt.Printf("    of which fewer than %d turns: %d (would be skipped)\n", MinTurns, tooShort)
	fmt.Printf("    eligible after age + turn filter: %d\n", ageOK-tooShort)

	// Cluster by first-user-message hash, among eligible records only.
	clusters := map[string][]SessionRecord{}
	for _, r := range records {
		if r.AgeDays > MaxAgeDays || r.TurnCount < MinTurns {
			continue
		}
		clusters[r.FirstUserHash] = append(clusters[r.FirstUserHash], r)
	}
	type clusterRow struct {
		Hash    string
		Count   int
		Sample  string
		Oldest  int
		Newest  int
	}
	var rows []clusterRow
	for h, rs := range clusters {
		oldest, newest := rs[0].AgeDays, rs[0].AgeDays
		for _, r := range rs {
			if r.AgeDays > oldest {
				oldest = r.AgeDays
			}
			if r.AgeDays < newest {
				newest = r.AgeDays
			}
		}
		rows = append(rows, clusterRow{h, len(rs), rs[0].FirstUserMsg, oldest, newest})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })

	fmt.Printf("\n=== TOP 20 FIRST-USER-MESSAGE CLUSTERS (eligible only) ===\n")
	fmt.Printf("Clusters with >1 member could be template-generated. Review before promoting.\n\n")
	n := 20
	if len(rows) < n {
		n = len(rows)
	}
	for i := 0; i < n; i++ {
		r := rows[i]
		sample := strings.ReplaceAll(r.Sample, "\n", " ")
		if len(sample) > 140 {
			sample = sample[:140] + "..."
		}
		fmt.Printf("  [%d sessions, age %d-%dd] %s\n    %s\n", r.Count, r.Newest, r.Oldest, r.Hash, sample)
	}
	// Also show count of singleton clusters for reference
	singletons := 0
	for _, r := range rows {
		if r.Count == 1 {
			singletons++
		}
	}
	fmt.Printf("\n  (%d clusters total, %d singletons)\n", len(rows), singletons)
}

// ---------------------------------------------------------------------------
// filter subcommand
// ---------------------------------------------------------------------------

func runFilter() error {
	home, _ := os.UserHomeDir()
	inPath := filepath.Join(home, BackfillDir, "manifest.jsonl")
	outPath := filepath.Join(home, BackfillDir, "filtered.jsonl")

	records, err := loadManifest(inPath)
	if err != nil {
		return err
	}

	var passed []SessionRecord
	var skippedAge, skippedShort, skippedCmd int

	// group by hash for dedup
	byHash := map[string][]SessionRecord{}
	for _, r := range records {
		if r.AgeDays > MaxAgeDays {
			skippedAge++
			continue
		}
		if r.TurnCount < MinTurns {
			skippedShort++
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(r.FirstUserMsg), "<command-name>") {
			skippedCmd++
			continue
		}
		byHash[r.FirstUserHash] = append(byHash[r.FirstUserHash], r)
	}

	// keep only the newest representative per cluster (smallest AgeDays).
	var deduped int
	for _, cluster := range byHash {
		sort.Slice(cluster, func(i, j int) bool { return cluster[i].AgeDays < cluster[j].AgeDays })
		passed = append(passed, cluster[0])
		deduped += len(cluster) - 1
	}

	// sort final set by start_time ascending so summarize emits in chronological order
	sort.Slice(passed, func(i, j int) bool { return passed[i].StartTime < passed[j].StartTime })

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	enc := json.NewEncoder(out)
	for _, r := range passed {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}

	fmt.Println("=== FILTER RESULTS ===")
	fmt.Printf("manifest input:       %d\n", len(records))
	fmt.Printf("skipped (>%dd old):   %d\n", MaxAgeDays, skippedAge)
	fmt.Printf("skipped (<%d turns):  %d\n", MinTurns, skippedShort)
	fmt.Printf("skipped (slashcmd):   %d\n", skippedCmd)
	fmt.Printf("cluster duplicates:   %d (newest kept, older dropped)\n", deduped)
	fmt.Printf("passed to summarize:  %d\n", len(passed))
	fmt.Printf("\nwrote: %s\n", outPath)
	return nil
}

func loadManifest(path string) ([]SessionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var records []SessionRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		var r SessionRecord
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, scanner.Err()
}

// ---------------------------------------------------------------------------
// summarize subcommand
// ---------------------------------------------------------------------------

const SystemPromptDigest = `You are a third-person note-taker reviewing a full historical conversation transcript between a human and a coding agent. Produce a concise 4-8 bullet digest of the session as a whole.

Output only bullet points (each starting with "- "). NOTHING else - no preamble, no closing.

Focus on:
- First bullet: what the user wanted to accomplish (session goal, one sentence)
- Middle bullets: key decisions, tools invoked, files touched, problems hit, solutions chosen - be specific with file names, commands, actual outcomes
- Final bullet: outcome - was the goal achieved, what was left open

Rules:
- Third person: "User asked...", "Agent read file X", "Agent ran command Y"
- Skip environment boilerplate, recoverable tool errors, internal reasoning
- Match the language of the human's messages (English or Russian)
- Do not editorialize, evaluate, or suggest next steps
- Do not include any text before or after the bullets`

const MaxTranscriptBytes = 180000 // cap input to haiku at ~180KB

func runSummarize() error {
	home, _ := os.UserHomeDir()
	inPath := filepath.Join(home, BackfillDir, "filtered.jsonl")
	stagingDir := filepath.Join(home, BackfillDir, "ai")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}

	records, err := loadManifest(inPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "summarizing %d sessions to %s\n", len(records), stagingDir)

	// ensure claude CLI is available
	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude binary not found on PATH: %w", err)
	}

	var done, skipped, failed int
	for i, r := range records {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s %s...", i+1, len(records), r.Agent, r.SessionID[:min(8, len(r.SessionID))])

		day := strings.SplitN(r.StartTime, "T", 2)[0]
		if day == "" {
			day = "unknown-date"
		}
		targetFile := filepath.Join(stagingDir, day+".md")

		if anchorExists(targetFile, r.SessionID) {
			fmt.Fprintln(os.Stderr, " skipped (already captured)")
			skipped++
			continue
		}

		transcript, err := formatTranscript(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, " parse failed: %v\n", err)
			failed++
			continue
		}
		if strings.TrimSpace(transcript) == "" {
			fmt.Fprintln(os.Stderr, " empty transcript")
			failed++
			continue
		}
		if len(transcript) > MaxTranscriptBytes {
			transcript = transcript[:MaxTranscriptBytes] + "\n...(truncated)"
		}

		summary, err := callHaiku(claudeBin, transcript)
		if err != nil || strings.TrimSpace(summary) == "" {
			fmt.Fprintf(os.Stderr, " haiku failed: %v\n", err)
			failed++
			continue
		}

		if err := appendEntry(targetFile, r, summary); err != nil {
			fmt.Fprintf(os.Stderr, " write failed: %v\n", err)
			failed++
			continue
		}

		done++
		fmt.Fprintln(os.Stderr, " ok")
		time.Sleep(300 * time.Millisecond) // light rate limiting
	}

	fmt.Fprintf(os.Stderr, "\n=== SUMMARIZE RESULTS ===\ndone: %d\nskipped: %d\nfailed: %d\nstaging: %s\n",
		done, skipped, failed, stagingDir)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// anchorExists returns true if the file already contains an entry for sessionID.
func anchorExists(path, sessionID string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	needle := "session:" + sessionID
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), needle) {
			return true
		}
	}
	return false
}

// appendEntry writes a new <backfill> section to the daily file.
func appendEntry(path string, r SessionRecord, summary string) error {
	hhmm := "00:00"
	if t, err := time.Parse(time.RFC3339, r.StartTime); err == nil {
		hhmm = t.Format("15:04")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	header := fmt.Sprintf("\n### %s <backfill %s>\n<!-- session:%s transcript:%s agent:%s -->\n",
		hhmm, r.Agent, r.SessionID, r.Path, r.Agent)
	_, err = io.WriteString(f, header+strings.TrimRight(summary, "\n")+"\n")
	return err
}

// formatTranscript reads the raw transcript file and returns a text-formatted
// dialogue suitable for feeding into claude -p haiku. Handles both Claude Code
// and Codex formats.
func formatTranscript(r SessionRecord) (string, error) {
	switch r.Agent {
	case "claude-code":
		return formatClaudeTranscript(r.Path)
	case "codex":
		return formatCodexTranscript(r.Path)
	default:
		return "", fmt.Errorf("unknown agent: %s", r.Agent)
	}
}

func formatClaudeTranscript(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

	var b strings.Builder
	b.WriteString("=== Transcript of a conversation between a human and Claude Code ===\n")

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		t, _ := obj["type"].(string)
		if t != "user" && t != "assistant" {
			continue
		}
		msg, _ := obj["message"].(map[string]any)
		if msg == nil {
			continue
		}
		if t == "user" {
			if obj["isMeta"] == true {
				continue
			}
			if s, ok := msg["content"].(string); ok && strings.TrimSpace(s) != "" {
				b.WriteString("[Human]: ")
				b.WriteString(strings.TrimSpace(s))
				b.WriteString("\n")
				continue
			}
			if list, ok := msg["content"].([]any); ok {
				for _, blk := range list {
					m, ok := blk.(map[string]any)
					if !ok {
						continue
					}
					bt, _ := m["type"].(string)
					if bt == "text" {
						if tt, ok := m["text"].(string); ok && strings.TrimSpace(tt) != "" {
							b.WriteString("[Human]: ")
							b.WriteString(strings.TrimSpace(tt))
							b.WriteString("\n")
						}
					} else if bt == "tool_result" {
						res := toolResultText(m["content"])
						res = truncate(res, 600)
						isError, _ := m["is_error"].(bool)
						if isError {
							b.WriteString("[Tool error]: ")
						} else {
							b.WriteString("[Tool output]: ")
						}
						b.WriteString(res)
						b.WriteString("\n")
					}
				}
			}
			continue
		}
		// assistant
		list, _ := msg["content"].([]any)
		for _, blk := range list {
			m, ok := blk.(map[string]any)
			if !ok {
				continue
			}
			bt, _ := m["type"].(string)
			switch bt {
			case "text":
				if tt, ok := m["text"].(string); ok && strings.TrimSpace(tt) != "" {
					b.WriteString("[Claude Code]: ")
					b.WriteString(strings.TrimSpace(tt))
					b.WriteString("\n")
				}
			case "tool_use":
				name, _ := m["name"].(string)
				input, _ := m["input"].(map[string]any)
				var parts []string
				for k, v := range input {
					vs := truncate(fmt.Sprintf("%v", v), 120)
					parts = append(parts, fmt.Sprintf("%s=%s", k, vs))
				}
				summary := truncate(strings.Join(parts, ", "), 400)
				b.WriteString(fmt.Sprintf("[Claude Code calls tool]: %s(%s)\n", name, summary))
			}
		}
	}
	return b.String(), scanner.Err()
}

func toolResultText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if list, ok := v.([]any); ok {
		var parts []string
		for _, blk := range list {
			if m, ok := blk.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return fmt.Sprintf("%v", v)
}

func formatCodexTranscript(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

	var b strings.Builder
	b.WriteString("=== Transcript of a conversation between a human and Codex ===\n")

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		t, _ := obj["type"].(string)
		payload, _ := obj["payload"].(map[string]any)
		if payload == nil {
			continue
		}
		switch t {
		case "event_msg":
			pt, _ := payload["type"].(string)
			if pt == "user_message" {
				msg, _ := payload["message"].(string)
				msg = strings.TrimSpace(msg)
				if msg == "" || strings.HasPrefix(msg, "<environment_context>") {
					continue
				}
				b.WriteString("[Human]: ")
				b.WriteString(msg)
				b.WriteString("\n")
			} else if pt == "agent_message" {
				if m, ok := payload["message"].(string); ok && strings.TrimSpace(m) != "" {
					b.WriteString("[Codex]: ")
					b.WriteString(strings.TrimSpace(m))
					b.WriteString("\n")
				}
			}
		case "response_item":
			pt, _ := payload["type"].(string)
			if pt != "message" {
				continue
			}
			role, _ := payload["role"].(string)
			if role != "assistant" {
				continue
			}
			content, _ := payload["content"].([]any)
			for _, blk := range content {
				m, ok := blk.(map[string]any)
				if !ok {
					continue
				}
				if tt, ok := m["text"].(string); ok && strings.TrimSpace(tt) != "" {
					b.WriteString("[Codex]: ")
					b.WriteString(strings.TrimSpace(tt))
					b.WriteString("\n")
				}
			}
		}
	}
	return b.String(), scanner.Err()
}

func callHaiku(claudeBin, transcript string) (string, error) {
	return callHaikuWithPrompt(claudeBin, SystemPromptDigest, transcript)
}

func callHaikuWithPrompt(claudeBin, systemPrompt, input string) (string, error) {
	if len(input) > MaxTranscriptBytes {
		input = input[:MaxTranscriptBytes] + "\n...(truncated)"
	}
	cmd := exec.Command(claudeBin,
		"-p",
		"--model", "haiku",
		"--no-session-persistence",
		"--no-chrome",
		"--system-prompt", systemPrompt,
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(
		os.Environ(),
		"MEMSEARCH_NO_WATCH=1",
		"MEMSEARCH_SKIP_CLAUDE_HOOKS=1",
		"CLAUDECODE=",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ---------------------------------------------------------------------------
// consolidate subcommand
// ---------------------------------------------------------------------------

const VaultAIDir = "Documents/knowledge/ai"

// Entry represents one parsed session-or-turn block from an ai/*.md file.
type Entry struct {
	Time           string // "HH:MM"
	Agent          string // e.g. "claude-code" or "codex"
	SessionID      string
	TranscriptPath string
	Body           []string // bullet lines
}

var (
	reH2Turn        = regexp.MustCompile(`^## (\d{2}:\d{2}) ([a-z-]+)\s*$`)
	reH2Session     = regexp.MustCompile(`^## Session (\d{2}:\d{2})\s*$`)
	reH3Turn        = regexp.MustCompile(`^### (\d{2}:\d{2})(?:\s+<([^>]+)>)?\s*$`)
	reH1Date        = regexp.MustCompile(`^# \d{4}-\d{2}-\d{2}\s*$`)
	reH2Digest      = regexp.MustCompile(`^## Daily digest\s*$`)
	reCommentAnchor = regexp.MustCompile(`<!--\s*session:([a-zA-Z0-9-]+).*?transcript:([^\s]+)`)
	reItalicAnchor  = regexp.MustCompile("\\*session `([a-zA-Z0-9-]+)` - \\[raw transcript\\]\\(([^)]+)\\)\\*")
)

func runConsolidate(args []string) error {
	home, _ := os.UserHomeDir()
	aiDir := filepath.Join(home, VaultAIDir)

	var targets []string
	scope := "today"
	ifNeeded := false
	for _, a := range args {
		switch a {
		case "--all":
			scope = "all"
		case "--old":
			scope = "old"
		case "--if-needed":
			ifNeeded = true
		default:
			targets = append(targets, a)
			scope = "file"
		}
	}

	if scope != "file" {
		files, _ := filepath.Glob(filepath.Join(aiDir, "*.md"))
		today := time.Now().Format("2006-01-02")
		for _, f := range files {
			base := strings.TrimSuffix(filepath.Base(f), ".md")
			if !isDateFormat(base) {
				continue
			}
			switch scope {
			case "today":
				if base == today {
					targets = append(targets, f)
				}
			case "old":
				if base < today {
					targets = append(targets, f)
				}
			case "all":
				targets = append(targets, f)
			}
		}
	}

	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "no files to consolidate")
		return nil
	}

	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found: %w", err)
	}

	var succeeded, failed, skipped int
	for _, f := range targets {
		if ifNeeded && alreadyConsolidated(f) {
			skipped++
			continue
		}
		if err := consolidateFile(claudeBin, f); err != nil {
			fmt.Fprintf(os.Stderr, "  failed %s: %v\n", filepath.Base(f), err)
			failed++
			continue
		}
		succeeded++
	}
	fmt.Fprintf(os.Stderr, "\nconsolidated: %d ok, %d failed, %d skipped (--if-needed)\n",
		succeeded, failed, skipped)
	return nil
}

// alreadyConsolidated returns true if the file already has a Daily digest header
// AND no duplicate session IDs (i.e. one entry per session, not per turn).
func alreadyConsolidated(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(content)
	if !strings.Contains(s, "## Daily digest") {
		return false
	}
	entries := parseEntries(s)
	seen := map[string]int{}
	for _, e := range entries {
		if e.SessionID != "" {
			seen[e.SessionID]++
			if seen[e.SessionID] > 1 {
				return false
			}
		}
	}
	return true
}

func isDateFormat(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func consolidateFile(claudeBin, path string) error {
	fmt.Fprintf(os.Stderr, "\n%s:\n", filepath.Base(path))

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	entries := parseEntries(string(content))
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "  no entries, skipping")
		return nil
	}

	// Group by session ID; entries without ID become their own group.
	groups := make(map[string][]Entry)
	order := []string{}
	for _, e := range entries {
		key := e.SessionID
		if key == "" {
			key = "anon-" + e.Time
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], e)
	}

	// Consolidate multi-entry groups.
	var sessions []Entry
	for _, key := range order {
		group := groups[key]
		if len(group) == 1 {
			sessions = append(sessions, group[0])
			continue
		}
		fmt.Fprintf(os.Stderr, "  session %s: %d turns -> 1 digest\n",
			truncate(key, 12), len(group))
		consolidated, err := consolidateGroup(claudeBin, group)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    haiku failed: %v, falling back to merge\n", err)
			merged := group[0]
			for _, g := range group[1:] {
				merged.Body = append(merged.Body, g.Body...)
			}
			sessions = append(sessions, merged)
			continue
		}
		sessions = append(sessions, consolidated)
		time.Sleep(300 * time.Millisecond)
	}

	// Daily digest.
	var dailyDigest []string
	if len(sessions) > 0 {
		fmt.Fprintf(os.Stderr, "  generating daily digest from %d sessions\n", len(sessions))
		d, err := generateDailyDigest(claudeBin, sessions)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    daily digest failed: %v\n", err)
		} else {
			dailyDigest = d
		}
	}

	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Time < sessions[j].Time })

	date := strings.TrimSuffix(filepath.Base(path), ".md")
	newContent := formatDailyFile(date, dailyDigest, sessions)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(newContent), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  wrote %d sessions + daily digest\n", len(sessions))
	return nil
}

func parseEntries(content string) []Entry {
	lines := strings.Split(content, "\n")
	var entries []Entry
	var current *Entry
	inDailyDigest := false

	flush := func() {
		if current != nil {
			entries = append(entries, *current)
			current = nil
		}
	}

	for _, line := range lines {
		if reH1Date.MatchString(line) {
			continue
		}
		if reH2Digest.MatchString(line) {
			flush()
			inDailyDigest = true
			continue
		}

		// New entry heading?
		if m := reH2Turn.FindStringSubmatch(line); m != nil {
			flush()
			inDailyDigest = false
			current = &Entry{Time: m[1], Agent: m[2]}
			continue
		}
		if m := reH2Session.FindStringSubmatch(line); m != nil {
			// Old-format empty heading marker. Flush any current entry and skip.
			flush()
			inDailyDigest = false
			continue
		}
		if m := reH3Turn.FindStringSubmatch(line); m != nil {
			flush()
			inDailyDigest = false
			agent := "claude-code"
			if m[2] != "" {
				parts := strings.Fields(m[2])
				if len(parts) > 0 {
					agent = parts[len(parts)-1]
				}
			}
			current = &Entry{Time: m[1], Agent: agent}
			continue
		}

		// Break on any other H1/H2 that isn't a known marker while we're outside the digest block.
		if strings.HasPrefix(line, "## ") && !inDailyDigest {
			flush()
			continue
		}
		if strings.HasPrefix(line, "---") {
			if inDailyDigest {
				inDailyDigest = false
			}
			continue
		}

		if current == nil || inDailyDigest {
			continue
		}

		if m := reCommentAnchor.FindStringSubmatch(line); m != nil {
			current.SessionID = m[1]
			current.TranscriptPath = m[2]
			continue
		}
		if m := reItalicAnchor.FindStringSubmatch(line); m != nil {
			current.SessionID = m[1]
			current.TranscriptPath = m[2]
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		current.Body = append(current.Body, trimmed)
	}
	flush()

	// Drop entries with no body (empty session markers).
	var nonEmpty []Entry
	for _, e := range entries {
		if len(e.Body) > 0 {
			nonEmpty = append(nonEmpty, e)
		}
	}
	return nonEmpty
}

const SystemPromptSessionConsolidate = `You are a third-person note-taker reviewing multiple per-turn notes from ONE conversation session between a human and a coding agent. Consolidate them into ONE coherent 4-8 bullet session-level digest.

Output only bullet points (each starting with "- "). NOTHING else - no preamble, no closing.

Rules:
- First bullet: what the user wanted (session goal, one sentence)
- Middle bullets: key decisions, tools invoked, files touched, problems hit, solutions chosen - be specific with file names, commands, actual outcomes
- Final bullet: outcome - was the goal achieved, what was left open
- Third person ("User asked...", "Agent did...")
- Skip environment boilerplate and recoverable errors
- Match the language of the content`

func consolidateGroup(claudeBin string, group []Entry) (Entry, error) {
	var input strings.Builder
	input.WriteString("=== Per-turn notes from a single session ===\n\n")
	for i, e := range group {
		fmt.Fprintf(&input, "Turn %d (%s):\n", i+1, e.Time)
		for _, b := range e.Body {
			input.WriteString(b)
			input.WriteString("\n")
		}
		input.WriteString("\n")
	}

	summary, err := callHaikuWithPrompt(claudeBin, SystemPromptSessionConsolidate, input.String())
	if err != nil {
		return Entry{}, err
	}

	result := group[0]
	result.Body = splitBullets(summary)
	return result, nil
}

const SystemPromptDailyDigest = `You are a third-person note-taker reviewing session digests from ONE day. Produce a concise 4-8 bullet narrative digest covering the whole day.

Output only bullet points (each starting with "- "). NOTHING else.

Rules:
- Tell the day's main thread of work - what the user set out to do, what got done, what is left open
- Reference specific projects, files, or tools where relevant
- Third person throughout
- Combine related sessions into one bullet where possible; do not list each session individually
- Match the language of the content
- No preamble, no closing`

func generateDailyDigest(claudeBin string, sessions []Entry) ([]string, error) {
	var input strings.Builder
	input.WriteString("=== Session digests from one day ===\n\n")
	for i, s := range sessions {
		fmt.Fprintf(&input, "Session %d at %s (%s):\n", i+1, s.Time, s.Agent)
		for _, b := range s.Body {
			input.WriteString(b)
			input.WriteString("\n")
		}
		input.WriteString("\n")
	}

	summary, err := callHaikuWithPrompt(claudeBin, SystemPromptDailyDigest, input.String())
	if err != nil {
		return nil, err
	}
	return splitBullets(summary), nil
}

func splitBullets(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func formatDailyFile(date string, dailyDigest []string, sessions []Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", date)

	if len(dailyDigest) > 0 {
		b.WriteString("## Daily digest\n\n")
		fmt.Fprintf(&b, "*Last updated: %s*\n\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))
		for _, bullet := range dailyDigest {
			b.WriteString(bullet)
			b.WriteString("\n")
		}
		b.WriteString("\n---\n\n")
	}

	for _, s := range sessions {
		fmt.Fprintf(&b, "## %s %s\n", s.Time, s.Agent)
		if s.SessionID != "" {
			if s.TranscriptPath != "" {
				fmt.Fprintf(&b, "*session `%s` - [raw transcript](%s)*\n\n", s.SessionID, s.TranscriptPath)
			} else {
				fmt.Fprintf(&b, "*session `%s`*\n\n", s.SessionID)
			}
		}
		for _, bullet := range s.Body {
			b.WriteString(bullet)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
