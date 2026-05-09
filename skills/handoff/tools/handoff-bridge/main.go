package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultAddr       = "127.0.0.1:18777"
	defaultMaxRecent  = 20
	maxQueryLength    = 500
	maxPromptLength   = 4000
	maxResponseLength = 200000
)

type server struct {
	home       string
	cmdTimeout time.Duration
	askToken   string
}

type limitedBuffer struct {
	buf []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := maxResponseLength - len(b.buf)
	if remaining > 0 {
		if len(p) < remaining {
			remaining = len(p)
		}
		b.buf = append(b.buf, p[:remaining]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf
}

type statusResponse struct {
	OK             bool   `json:"ok"`
	Hostname       string `json:"hostname"`
	Time           string `json:"time"`
	DroidAvailable bool   `json:"droid_available"`
	CodexAvailable bool   `json:"codex_available"`
	KnowledgeVault string `json:"knowledge_vault"`
}

type sessionStart struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	SessionTitle string `json:"sessionTitle"`
	CWD          string `json:"cwd"`
}

type recentSession struct {
	ID        string `json:"id"`
	Agent     string `json:"agent"`
	Title     string `json:"title,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updated_at"`
}

type recentResponse struct {
	OK       bool            `json:"ok"`
	Sessions []recentSession `json:"sessions"`
}

type sessionCandidate struct {
	path    string
	modTime time.Time
}

type searchResponse struct {
	OK     bool            `json:"ok"`
	Query  string          `json:"query"`
	Droid  json.RawMessage `json:"droid,omitempty"`
	Output string          `json:"output,omitempty"`
}

type askRequest struct {
	Agent       string `json:"agent"`
	SessionID   string `json:"session_id"`
	Instruction string `json:"instruction"`
	Mode        string `json:"mode"`
}

type askResponse struct {
	OK        bool   `json:"ok"`
	Agent     string `json:"agent"`
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
	Output    string `json:"output"`
}

func main() {
	addr := flag.String("addr", defaultAddr, "listen address")
	timeout := flag.Duration("timeout", 2*time.Minute, "command timeout")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	s := &server{
		home:       home,
		cmdTimeout: *timeout,
		askToken:   strings.TrimSpace(os.Getenv("HANDOFF_BRIDGE_TOKEN")),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.status)
	mux.HandleFunc("/recent", s.recent)
	mux.HandleFunc("/search", s.search)
	mux.HandleFunc("/ask", s.ask)

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           withJSONErrors(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("handoff bridge listening on %s", *addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func withJSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	host, _ := os.Hostname()
	writeJSON(w, statusResponse{
		OK:             true,
		Hostname:       host,
		Time:           time.Now().Format(time.RFC3339),
		DroidAvailable: commandExists("droid"),
		CodexAvailable: commandExists("codex"),
		KnowledgeVault: filepath.Join(s.home, "Workspace", "knowledge"),
	})
}

func (s *server) recent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeSensitive(w, r) {
		return
	}
	sessions, err := s.recentDroidSessions(defaultMaxRecent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, recentResponse{OK: true, Sessions: sessions})
}

func (s *server) search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeSensitive(w, r) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing q")
		return
	}
	if len(q) > maxQueryLength {
		writeError(w, http.StatusBadRequest, "q too long")
		return
	}
	stdout, stderr, err := s.run(r.Context(), "droid", "search", q, "--json", "--limit-sessions", "10", "--limit-hits", "3", "--context-chars", "120")
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error()+"\n"+truncate(stderr))
		return
	}
	if !json.Valid(stdout) {
		writeError(w, http.StatusBadGateway, "droid search returned invalid json\n"+truncate(stderr))
		return
	}
	writeJSON(w, searchResponse{OK: true, Query: q, Droid: json.RawMessage(stdout), Output: truncate(stderr)})
}

func (s *server) ask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeSensitive(w, r) {
		return
	}
	var req askRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Agent = strings.ToLower(strings.TrimSpace(req.Agent))
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Instruction = strings.TrimSpace(req.Instruction)
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode == "" {
		req.Mode = "continue"
	}
	if req.Agent != "droid" {
		writeError(w, http.StatusBadRequest, "only agent=droid is supported in v1")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "missing session_id")
		return
	}
	if req.Instruction == "" {
		writeError(w, http.StatusBadRequest, "missing instruction")
		return
	}
	if len(req.Instruction) > maxPromptLength {
		writeError(w, http.StatusBadRequest, "instruction too long")
		return
	}

	var args []string
	switch req.Mode {
	case "continue":
		args = []string{"exec", "--session-id", req.SessionID, req.Instruction}
	case "fork":
		args = []string{"exec", "--fork", req.SessionID, req.Instruction}
	default:
		writeError(w, http.StatusBadRequest, "mode must be continue or fork")
		return
	}

	stdout, stderr, err := s.run(r.Context(), "droid", args...)
	output := append(stdout, stderr...)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error()+"\n"+truncate(output))
		return
	}
	writeJSON(w, askResponse{
		OK:        true,
		Agent:     req.Agent,
		SessionID: req.SessionID,
		Mode:      req.Mode,
		Output:    truncate(output),
	})
}

func (s *server) authorizeSensitive(w http.ResponseWriter, r *http.Request) bool {
	if s.askToken == "" {
		writeError(w, http.StatusServiceUnavailable, "HANDOFF_BRIDGE_TOKEN must be set to enable sensitive endpoints")
		return false
	}
	token := strings.TrimSpace(r.Header.Get("X-Handoff-Token"))
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.askToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func (s *server) recentDroidSessions(limit int) ([]recentSession, error) {
	root := filepath.Join(s.home, ".factory", "sessions")
	var candidates []sessionCandidate
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		candidates = append(candidates, sessionCandidate{path: path, modTime: info.ModTime()})
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	sessions := make([]recentSession, 0, len(candidates))
	for _, candidate := range candidates {
		path := candidate.path
		start, _ := readSessionStart(path)
		id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		title := start.SessionTitle
		if title == "" {
			title = start.Title
		}
		sessions = append(sessions, recentSession{
			ID:        id,
			Agent:     "droid",
			Title:     title,
			CWD:       start.CWD,
			Path:      path,
			UpdatedAt: candidate.modTime.Format(time.RFC3339),
		})
	}
	return sessions, nil
}

func readSessionStart(path string) (sessionStart, error) {
	f, err := os.Open(path)
	if err != nil {
		return sessionStart{}, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var start sessionStart
		if err := json.Unmarshal(scanner.Bytes(), &start); err == nil && start.Type == "session_start" {
			return start, nil
		}
	}
	return sessionStart{}, scanner.Err()
}

func (s *server) run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("%s timed out after %s", name, s.cmdTimeout)
	}
	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), err
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func writeJSON(w http.ResponseWriter, value any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	writeJSON(w, map[string]any{"ok": false, "error": message})
}

func truncate(b []byte) string {
	if len(b) <= maxResponseLength {
		return string(b)
	}
	return string(b[:maxResponseLength])
}
