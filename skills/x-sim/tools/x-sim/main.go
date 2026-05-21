package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	_ "github.com/mattn/go-sqlite3"
)

const defaultRetention = "6m"

type source struct {
	ID    int64
	Kind  string
	Value string
}

type tweet struct {
	ID           string
	SourceID     int64
	AuthorHandle string
	AuthorName   string
	Text         string
	URL          string
	PostedAt     time.Time
	LikeCount    int64
	RepostCount  int64
	ReplyCount   int64
	QuoteCount   int64
	RawJSON      string
}

type evaluation struct {
	Persona   string
	Scores    map[string]int
	Rationale string
	Risks     string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "x-sim:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	db, err := openDB("")
	if err != nil {
		return err
	}
	defer db.Close()

	switch args[0] {
	case "init":
		return initCommand(db, stdout)
	case "source":
		return sourceCommand(db, args[1:], stdout)
	case "sync":
		return syncCommand(db, args[1:], stdout, stderr)
	case "brief":
		return briefCommand(db, args[1:], stdout)
	case "eval-tweet":
		return evalCommand(db, "tweet", args[1:], stdout)
	case "eval-handle":
		return evalCommand(db, "handle", args[1:], stdout)
	case "report":
		return reportCommand(db, args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `x-sim stores scraped X context and evaluates tweets/handle positioning offline.

Commands:
  init
  source add-account @handle
  source add-search "query"
  sync --since 6m --limit-per-source 200 [--input file.json]
  brief --topic "agents"
  eval-tweet --text "draft tweet" [--audience "AI builders"]
  eval-handle --bio "..." --positioning "..."
  report --session <id> [--out report.md]`)
}

func openDB(path string) (*sql.DB, error) {
	path, err := resolveDBPath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func resolveDBPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if envPath := os.Getenv("X_SIM_DB"); envPath != "" {
		return envPath, nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "dotagents", "x-sim", "x-sim.sqlite"), nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`CREATE TABLE IF NOT EXISTS sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL CHECK(kind IN ('account', 'search')),
			value TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_synced_at TEXT,
			UNIQUE(kind, value)
		);`,
		`CREATE TABLE IF NOT EXISTS tweets (
			id TEXT PRIMARY KEY,
			source_id INTEGER,
			author_handle TEXT,
			author_name TEXT,
			text TEXT NOT NULL,
			url TEXT,
			posted_at TEXT NOT NULL,
			like_count INTEGER NOT NULL DEFAULT 0,
			repost_count INTEGER NOT NULL DEFAULT 0,
			reply_count INTEGER NOT NULL DEFAULT 0,
			quote_count INTEGER NOT NULL DEFAULT 0,
			raw_json TEXT NOT NULL,
			fetched_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS tweets_posted_at_idx ON tweets(posted_at);`,
		`CREATE INDEX IF NOT EXISTS tweets_author_idx ON tweets(author_handle);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL CHECK(kind IN ('tweet', 'handle')),
			input_text TEXT NOT NULL,
			audience TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS evaluations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			persona TEXT NOT NULL,
			clarity INTEGER NOT NULL,
			novelty INTEGER NOT NULL,
			credibility INTEGER NOT NULL,
			audience_fit INTEGER NOT NULL,
			reply_potential INTEGER NOT NULL,
			risk INTEGER NOT NULL,
			rationale TEXT NOT NULL,
			risks TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func initCommand(db *sql.DB, stdout io.Writer) error {
	path, err := resolveDBPath("")
	if err != nil {
		return err
	}
	var sourceCount, tweetCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sources`).Scan(&sourceCount); err != nil {
		return err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tweets`).Scan(&tweetCount); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Initialized x-sim DB (%s): %d sources, %d tweets\n", path, sourceCount, tweetCount)
	return nil
}

func sourceCommand(db *sql.DB, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: x-sim source add-account @handle | source add-search \"query\"")
	}
	var kind, value string
	switch args[0] {
	case "add-account":
		kind = "account"
		value = normalizeHandle(args[1])
	case "add-search":
		kind = "search"
		value = strings.TrimSpace(strings.Join(args[1:], " "))
	default:
		return fmt.Errorf("unknown source action %q", args[0])
	}
	if value == "" {
		return errors.New("source value cannot be empty")
	}
	_, err := db.Exec(`INSERT OR IGNORE INTO sources(kind, value) VALUES(?, ?)`, kind, value)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Tracked %s source: %s\n", kind, value)
	return nil
}

func syncCommand(db *sql.DB, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sinceRaw := fs.String("since", defaultRetention, "retention window such as 6m, 30d, or 1y")
	limit := fs.Int("limit-per-source", 200, "maximum tweets per source")
	input := fs.String("input", "", "read x-cli JSON from a file instead of invoking x-cli")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cutoff, err := parseWindowCutoff(*sinceRaw, time.Now())
	if err != nil {
		return err
	}
	sources, err := listSources(db)
	if err != nil {
		return err
	}
	if len(sources) == 0 && *input == "" {
		return errors.New("no sources tracked; add one with x-sim source add-account or add-search")
	}
	total := 0
	if *input != "" {
		raw, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		src := source{Kind: "search", Value: "input"}
		n, err := ingestJSON(db, src, raw, cutoff)
		if err != nil {
			return err
		}
		total += n
	} else {
		for _, src := range sources {
			raw, err := runXCLI(src, *limit)
			if err != nil {
				fmt.Fprintf(stderr, "warning: source %s:%s failed: %v\n", src.Kind, src.Value, err)
				continue
			}
			n, err := ingestJSON(db, src, raw, cutoff)
			if err != nil {
				fmt.Fprintf(stderr, "warning: source %s:%s parse failed: %v\n", src.Kind, src.Value, err)
				continue
			}
			if _, err := db.Exec(`UPDATE sources SET last_synced_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), src.ID); err != nil {
				return err
			}
			total += n
		}
	}
	if _, err := db.Exec(`DELETE FROM tweets WHERE posted_at < ?`, cutoff.Format(time.RFC3339)); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Synced %d tweets; retained tweets since %s\n", total, cutoff.Format("2006-01-02"))
	return nil
}

func runXCLI(src source, limit int) ([]byte, error) {
	var args []string
	switch src.Kind {
	case "account":
		args = []string{"timeline", "user", src.Value, "--count", strconv.Itoa(limit), "--json"}
	case "search":
		args = []string{"search", src.Value, "--type", "latest", "--count", strconv.Itoa(limit), "--json"}
	default:
		return nil, fmt.Errorf("unsupported source kind %q", src.Kind)
	}
	cmd := exec.Command("x-cli", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func listSources(db *sql.DB) ([]source, error) {
	rows, err := db.Query(`SELECT id, kind, value FROM sources ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []source
	for rows.Next() {
		var src source
		if err := rows.Scan(&src.ID, &src.Kind, &src.Value); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func ingestJSON(db *sql.DB, src source, raw []byte, cutoff time.Time) (int, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, err
	}
	tweets := extractTweets(payload, src, time.Now().UTC())
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	count := 0
	for _, tw := range tweets {
		if tw.ID == "" || tw.Text == "" || tw.PostedAt.Before(cutoff) {
			continue
		}
		if tw.PostedAt.IsZero() {
			tw.PostedAt = time.Now().UTC()
		}
		_, err := tx.Exec(`
			INSERT INTO tweets(id, source_id, author_handle, author_name, text, url, posted_at, like_count, repost_count, reply_count, quote_count, raw_json, fetched_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				source_id=excluded.source_id,
				author_handle=excluded.author_handle,
				author_name=excluded.author_name,
				text=excluded.text,
				url=excluded.url,
				posted_at=excluded.posted_at,
				like_count=excluded.like_count,
				repost_count=excluded.repost_count,
				reply_count=excluded.reply_count,
				quote_count=excluded.quote_count,
				raw_json=excluded.raw_json,
				fetched_at=excluded.fetched_at`,
			tw.ID, nullableSourceID(src.ID), tw.AuthorHandle, tw.AuthorName, tw.Text, tw.URL,
			tw.PostedAt.UTC().Format(time.RFC3339), tw.LikeCount, tw.RepostCount, tw.ReplyCount,
			tw.QuoteCount, tw.RawJSON, time.Now().UTC().Format(time.RFC3339),
		)
		if err != nil {
			return count, err
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

func nullableSourceID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func extractTweets(payload any, src source, fetchedAt time.Time) []tweet {
	seen := map[string]bool{}
	var out []tweet
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			if tw, ok := tweetFromMap(x, src, fetchedAt); ok && !seen[tw.ID] {
				seen[tw.ID] = true
				out = append(out, tw)
			}
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(payload)
	return out
}

func tweetFromMap(m map[string]any, src source, fetchedAt time.Time) (tweet, bool) {
	id := firstString(m, "id", "id_str", "rest_id", "tweet_id", "tweetId")
	text := firstString(m, "text", "full_text", "tweetText", "content")
	if id == "" || text == "" {
		return tweet{}, false
	}
	raw, _ := json.Marshal(m)
	authorHandle := firstString(m, "author_handle", "authorHandle", "screen_name", "username", "handle")
	authorName := firstString(m, "author_name", "authorName", "name")
	if authorHandle == "" {
		authorHandle = nestedString(m, []string{"author", "user", "core", "legacy"}, "screen_name", "username", "handle")
	}
	if authorName == "" {
		authorName = nestedString(m, []string{"author", "user", "core", "legacy"}, "name")
	}
	postedAt := parseAnyTime(firstString(m, "created_at", "createdAt", "posted_at", "postedAt", "time", "timestamp"))
	if postedAt.IsZero() {
		postedAt = fetchedAt
	}
	url := firstString(m, "url", "tweet_url", "tweetUrl")
	if url == "" && authorHandle != "" {
		url = fmt.Sprintf("https://x.com/%s/status/%s", normalizeHandle(authorHandle), id)
	}
	return tweet{
		ID:           id,
		SourceID:     src.ID,
		AuthorHandle: normalizeHandle(authorHandle),
		AuthorName:   authorName,
		Text:         strings.TrimSpace(text),
		URL:          url,
		PostedAt:     postedAt.UTC(),
		LikeCount:    firstInt(m, "like_count", "likes", "favorite_count", "favoriteCount"),
		RepostCount:  firstInt(m, "repost_count", "retweet_count", "retweets", "reposts"),
		ReplyCount:   firstInt(m, "reply_count", "replies"),
		QuoteCount:   firstInt(m, "quote_count", "quotes"),
		RawJSON:      string(raw),
	}, true
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch x := v.(type) {
			case string:
				return strings.TrimSpace(x)
			case json.Number:
				return x.String()
			case float64:
				if x == float64(int64(x)) {
					return strconv.FormatInt(int64(x), 10)
				}
			}
		}
	}
	return ""
}

func nestedString(m map[string]any, containers []string, keys ...string) string {
	for _, c := range containers {
		if child, ok := m[c].(map[string]any); ok {
			if value := firstString(child, keys...); value != "" {
				return value
			}
			if value := nestedString(child, containers, keys...); value != "" {
				return value
			}
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int64:
				return x
			case string:
				n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
				return n
			}
		}
	}
	return 0
}

func parseAnyTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"Mon Jan 02 15:04:05 -0700 2006",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if unix > 1_000_000_000_000 {
			return time.UnixMilli(unix)
		}
		return time.Unix(unix, 0)
	}
	return time.Time{}
}

func briefCommand(db *sql.DB, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("brief", flag.ContinueOnError)
	topic := fs.String("topic", "", "topic filter")
	sinceRaw := fs.String("since", defaultRetention, "lookback window")
	limit := fs.Int("limit", 200, "maximum tweets to inspect")
	out := fs.String("out", "", "optional markdown output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cutoff, err := parseWindowCutoff(*sinceRaw, time.Now())
	if err != nil {
		return err
	}
	tweets, err := queryTweets(db, *topic, cutoff, *limit)
	if err != nil {
		return err
	}
	report := renderBrief(*topic, tweets, cutoff)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote %s\n", *out)
		return nil
	}
	fmt.Fprint(stdout, report)
	return nil
}

func queryTweets(db *sql.DB, topic string, cutoff time.Time, limit int) ([]tweet, error) {
	pattern := "%" + strings.TrimSpace(topic) + "%"
	query := `SELECT id, COALESCE(author_handle, ''), COALESCE(author_name, ''), text, COALESCE(url, ''), posted_at, like_count, repost_count, reply_count, quote_count, raw_json
		FROM tweets WHERE posted_at >= ?`
	args := []any{cutoff.Format(time.RFC3339)}
	if strings.TrimSpace(topic) != "" {
		query += ` AND text LIKE ?`
		args = append(args, pattern)
	}
	query += ` ORDER BY (like_count + repost_count * 2 + reply_count * 3 + quote_count * 2) DESC, posted_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tweets []tweet
	for rows.Next() {
		var tw tweet
		var posted string
		if err := rows.Scan(&tw.ID, &tw.AuthorHandle, &tw.AuthorName, &tw.Text, &tw.URL, &posted, &tw.LikeCount, &tw.RepostCount, &tw.ReplyCount, &tw.QuoteCount, &tw.RawJSON); err != nil {
			return nil, err
		}
		tw.PostedAt = parseAnyTime(posted)
		tweets = append(tweets, tw)
	}
	return tweets, rows.Err()
}

func renderBrief(topic string, tweets []tweet, cutoff time.Time) string {
	var b strings.Builder
	title := "X Audience Brief"
	if topic != "" {
		title += ": " + topic
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "Lookback: since %s. Tweets inspected: %d.\n\n", cutoff.Format("2006-01-02"), len(tweets))
	if len(tweets) == 0 {
		b.WriteString("No local tweet context matched. Run `x-sim sync` or broaden the topic.\n")
		return b.String()
	}
	b.WriteString("## Signals\n\n")
	for _, term := range topTerms(tweets, 12) {
		fmt.Fprintf(&b, "- %s\n", term)
	}
	b.WriteString("\n## Source Handles\n\n")
	for _, item := range topHandles(tweets, 8) {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	b.WriteString("\n## Evidence Tweets\n\n")
	for i, tw := range tweets {
		if i >= 8 {
			break
		}
		fmt.Fprintf(&b, "- @%s: %s", tw.AuthorHandle, oneLine(tw.Text, 220))
		if tw.URL != "" {
			fmt.Fprintf(&b, " (%s)", tw.URL)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func evalCommand(db *sql.DB, kind string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("eval-"+kind, flag.ContinueOnError)
	text := fs.String("text", "", "tweet text to evaluate")
	bio := fs.String("bio", "", "handle bio to evaluate")
	positioning := fs.String("positioning", "", "handle positioning or promotion angle")
	promotion := fs.String("promotion", "", "handle promotion angle")
	audience := fs.String("audience", "", "target audience")
	topic := fs.String("topic", "", "local tweet context topic")
	out := fs.String("out", "", "optional markdown output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	input := strings.TrimSpace(*text)
	if kind == "handle" {
		parts := []string{}
		if strings.TrimSpace(*bio) != "" {
			parts = append(parts, "Bio: "+strings.TrimSpace(*bio))
		}
		if strings.TrimSpace(*positioning) != "" {
			parts = append(parts, "Positioning: "+strings.TrimSpace(*positioning))
		}
		if strings.TrimSpace(*promotion) != "" {
			parts = append(parts, "Promotion: "+strings.TrimSpace(*promotion))
		}
		input = strings.Join(parts, "\n")
	}
	if input == "" {
		return fmt.Errorf("eval-%s requires input text", kind)
	}
	cutoff, _ := parseWindowCutoff(defaultRetention, time.Now())
	contextTweets, err := queryTweets(db, *topic, cutoff, 60)
	if err != nil {
		return err
	}
	sessionID := newSessionID(kind)
	if _, err := db.Exec(`INSERT INTO sessions(id, kind, input_text, audience) VALUES(?, ?, ?, ?)`, sessionID, kind, input, *audience); err != nil {
		return err
	}
	evals := simulate(kind, input, *audience, contextTweets)
	for _, ev := range evals {
		if _, err := db.Exec(`INSERT INTO evaluations(session_id, persona, clarity, novelty, credibility, audience_fit, reply_potential, risk, rationale, risks)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sessionID, ev.Persona, ev.Scores["clarity"], ev.Scores["novelty"], ev.Scores["credibility"],
			ev.Scores["audience_fit"], ev.Scores["reply_potential"], ev.Scores["risk"], ev.Rationale, ev.Risks,
		); err != nil {
			return err
		}
	}
	report := renderSessionReport(sessionID, kind, input, *audience, evals, contextTweets)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote %s for session %s\n", *out, sessionID)
		return nil
	}
	fmt.Fprint(stdout, report)
	return nil
}

func reportCommand(db *sql.DB, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	sessionID := fs.String("session", "", "session id")
	out := fs.String("out", "", "optional markdown output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" {
		return errors.New("report requires --session")
	}
	var kind, input, audience string
	if err := db.QueryRow(`SELECT kind, input_text, COALESCE(audience, '') FROM sessions WHERE id = ?`, *sessionID).Scan(&kind, &input, &audience); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT persona, clarity, novelty, credibility, audience_fit, reply_potential, risk, rationale, risks FROM evaluations WHERE session_id = ? ORDER BY id`, *sessionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var evals []evaluation
	for rows.Next() {
		ev := evaluation{Scores: map[string]int{}}
		if err := rows.Scan(&ev.Persona, scoreDest(ev.Scores, "clarity"), scoreDest(ev.Scores, "novelty"), scoreDest(ev.Scores, "credibility"), scoreDest(ev.Scores, "audience_fit"), scoreDest(ev.Scores, "reply_potential"), scoreDest(ev.Scores, "risk"), &ev.Rationale, &ev.Risks); err != nil {
			return err
		}
		evals = append(evals, ev)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	report := renderSessionReport(*sessionID, kind, input, audience, evals, nil)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote %s\n", *out)
		return nil
	}
	fmt.Fprint(stdout, report)
	return nil
}

func scoreDest(scores map[string]int, key string) any {
	return scoreScanner{scores: scores, key: key}
}

type scoreScanner struct {
	scores map[string]int
	key    string
}

func (s scoreScanner) Scan(src any) error {
	var n int
	var err error
	switch x := src.(type) {
	case int64:
		n = int(x)
	case int:
		n = x
	case []byte:
		n, err = strconv.Atoi(string(x))
	default:
		n, err = strconv.Atoi(fmt.Sprint(x))
	}
	if err != nil {
		return err
	}
	s.scores[s.key] = n
	return nil
}

func simulate(kind, input, audience string, contextTweets []tweet) []evaluation {
	personas := []string{
		"technical builder",
		"skeptical founder/operator",
		"AI-agent power user",
		"casual X reader",
		"contrarian critic",
	}
	var out []evaluation
	for _, persona := range personas {
		out = append(out, scoreForPersona(kind, input, audience, persona, contextTweets))
	}
	return out
}

func scoreForPersona(kind, input, audience, persona string, contextTweets []tweet) evaluation {
	words := strings.Fields(input)
	lower := strings.ToLower(input)
	length := len([]rune(input))
	hasSpecifics := regexp.MustCompile(`\d|/|github|repo|benchmark|trace|sqlite|go|codex|claude|agent`).MatchString(lower)
	hasQuestion := strings.Contains(input, "?")
	hasHype := regexp.MustCompile(`(?i)\b(revolutionary|game changer|insane|crazy|massive|excited|thrilled|unlock|leverage)\b`).MatchString(input)
	hasConcreteTension := regexp.MustCompile(`(?i)\b(but|except|tradeoff|failed|slower|faster|because|measured|cost|risk)\b`).MatchString(input)

	clarity := clamp(7-boolInt(length > 280)*2-boolInt(len(words) > 45)+boolInt(length >= 70 && length <= 220), 1, 10)
	novelty := clamp(4+boolInt(hasConcreteTension)*2+boolInt(hasSpecifics)*2-boolInt(hasHype), 1, 10)
	credibility := clamp(4+boolInt(hasSpecifics)*3+boolInt(hasConcreteTension)-boolInt(hasHype)*2, 1, 10)
	audienceFit := clamp(5+boolInt(strings.Contains(strings.ToLower(persona+" "+audience), "builder") && hasSpecifics)*2+boolInt(len(contextTweets) > 0), 1, 10)
	replyPotential := clamp(4+boolInt(hasQuestion)*2+boolInt(hasConcreteTension)*2+boolInt(persona == "contrarian critic"), 1, 10)
	risk := clamp(3+boolInt(hasHype)*3+boolInt(length > 280)*3+boolInt(!hasSpecifics && kind == "handle")+boolInt(strings.Count(input, "\n") > 4), 1, 10)

	rationale := "Likely response depends on whether the claim feels earned by concrete evidence."
	if hasSpecifics {
		rationale = "Concrete details make this easier to trust and discuss."
	}
	if hasConcreteTension {
		rationale += " The visible tradeoff gives readers something to react to."
	}
	risks := "May read as generic if the surrounding thread lacks evidence."
	if hasHype {
		risks = "Hype language raises AI-tone and credibility risk."
	}
	if length > 280 {
		risks = "Too long for a single tweet without compression."
	}
	return evaluation{
		Persona: persona,
		Scores: map[string]int{
			"clarity":         clarity,
			"novelty":         novelty,
			"credibility":     credibility,
			"audience_fit":    audienceFit,
			"reply_potential": replyPotential,
			"risk":            risk,
		},
		Rationale: rationale,
		Risks:     risks,
	}
}

func renderSessionReport(sessionID, kind, input, audience string, evals []evaluation, contextTweets []tweet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# x-sim %s evaluation\n\n", kind)
	fmt.Fprintf(&b, "Session: `%s`\n\n", sessionID)
	if audience != "" {
		fmt.Fprintf(&b, "Audience: %s\n\n", audience)
	}
	fmt.Fprintf(&b, "## Input\n\n%s\n\n", input)
	if len(evals) > 0 {
		fmt.Fprintf(&b, "## Scores\n\n| Persona | Clarity | Novelty | Credibility | Fit | Reply | Risk |\n|---|---:|---:|---:|---:|---:|---:|\n")
		for _, ev := range evals {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d |\n",
				ev.Persona, ev.Scores["clarity"], ev.Scores["novelty"], ev.Scores["credibility"],
				ev.Scores["audience_fit"], ev.Scores["reply_potential"], ev.Scores["risk"])
		}
		avg := averageScore(evals)
		fmt.Fprintf(&b, "\nAverage useful score: %.1f/10. Average risk: %.1f/10.\n\n", avg["useful"], avg["risk"])
		b.WriteString("## Persona Notes\n\n")
		for _, ev := range evals {
			fmt.Fprintf(&b, "- **%s**: %s Risk: %s\n", ev.Persona, ev.Rationale, ev.Risks)
		}
	}
	if len(contextTweets) > 0 {
		b.WriteString("\n## Local Evidence\n\n")
		for i, tw := range contextTweets {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- @%s: %s", tw.AuthorHandle, oneLine(tw.Text, 180))
			if tw.URL != "" {
				fmt.Fprintf(&b, " (%s)", tw.URL)
			}
			fmt.Fprintln(&b)
		}
	}
	b.WriteString("\n## Offline Boundary\n\nThis report is a local simulation from scraped context. It does not post or mutate X state.\n")
	return b.String()
}

func averageScore(evals []evaluation) map[string]float64 {
	out := map[string]float64{"useful": 0, "risk": 0}
	for _, ev := range evals {
		out["useful"] += float64(ev.Scores["clarity"]+ev.Scores["novelty"]+ev.Scores["credibility"]+ev.Scores["audience_fit"]+ev.Scores["reply_potential"]) / 5
		out["risk"] += float64(ev.Scores["risk"])
	}
	if len(evals) > 0 {
		out["useful"] /= float64(len(evals))
		out["risk"] /= float64(len(evals))
	}
	return out
}

func topHandles(tweets []tweet, n int) []string {
	counts := map[string]int{}
	for _, tw := range tweets {
		if tw.AuthorHandle != "" {
			counts["@"+tw.AuthorHandle]++
		}
	}
	return topMap(counts, n)
}

func topTerms(tweets []tweet, n int) []string {
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "that": true, "with": true, "you": true,
		"this": true, "from": true, "are": true, "but": true, "not": true, "have": true,
		"has": true, "was": true, "just": true, "your": true, "about": true, "into": true,
		"https": true, "com": true, "x": true, "twitter": true,
	}
	counts := map[string]int{}
	for _, tw := range tweets {
		for _, term := range splitTerms(tw.Text) {
			if len(term) >= 3 && !stop[term] {
				counts[term]++
			}
		}
	}
	return topMap(counts, n)
}

func splitTerms(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
}

func topMap(m map[string]int, n int) []string {
	type item struct {
		key   string
		count int
	}
	items := make([]item, 0, len(m))
	for k, v := range m {
		items = append(items, item{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].key < items[j].key
		}
		return items[i].count > items[j].count
	})
	var out []string
	for i, item := range items {
		if i >= n {
			break
		}
		out = append(out, fmt.Sprintf("%s (%d)", item.key, item.count))
	}
	return out
}

func parseWindowCutoff(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		raw = defaultRetention
	}
	now = now.UTC()
	if len(raw) < 2 {
		return time.Time{}, fmt.Errorf("invalid window %q", raw)
	}
	unit := raw[len(raw)-1]
	n, err := strconv.Atoi(raw[:len(raw)-1])
	if err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid window %q", raw)
	}
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -7*n), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	case 'y':
		return now.AddDate(-n, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid window %q; use d, w, m, or y", raw)
	}
}

func normalizeHandle(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), "@")
}

func oneLine(raw string, limit int) string {
	s := strings.Join(strings.Fields(raw), " ")
	if len([]rune(s)) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit-1]) + "..."
}

func clamp(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func newSessionID(kind string) string {
	return fmt.Sprintf("%s-%d", kind, time.Now().UnixNano())
}
