package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultDashboardAddr = "127.0.0.1:8765"

var openBrowser = openBrowserURL

type dashboardOptions struct {
	Addr           string
	Access         string
	NoOpen         bool
	EnableSessions bool
}

type dashboardHealth struct {
	OK              bool   `json:"ok"`
	Repo            string `json:"repo"`
	SessionsEnabled bool   `json:"sessions_enabled"`
}

func runDashboard(args []string) error {
	opts, err := parseDashboardFlags(args)
	if err != nil {
		return err
	}

	repoRoot, _, err := findRoots()
	if err != nil {
		return err
	}

	handler := newDashboardHandler(repoRoot, opts)
	listener, err := listenDashboard(opts.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	url := "http://" + listener.Addr().String()
	fmt.Printf("dotagents dashboard: %s\n", url)
	if opts.Access == "ssh" {
		host, port := dashboardForwardTarget(listener.Addr())
		fmt.Printf("ssh access: ssh -L 127.0.0.1:%s:%s:%s <ssh-host>\n", port, host, port)
	}

	if !opts.NoOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "open browser: %v\n", err)
		}
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func parseDashboardFlags(args []string) (dashboardOptions, error) {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	opts := dashboardOptions{Addr: defaultDashboardAddr, Access: "local"}
	fs.StringVar(&opts.Addr, "addr", opts.Addr, "Local address to bind, for example 127.0.0.1:0")
	fs.StringVar(&opts.Access, "access", opts.Access, "Access mode: local or ssh")
	fs.BoolVar(&opts.NoOpen, "no-open", false, "Do not open a browser")
	fs.BoolVar(&opts.EnableSessions, "enable-sessions", false, "Enable allowlisted terminal sessions")

	if err := fs.Parse(args); err != nil {
		return dashboardOptions{}, err
	}
	if fs.NArg() != 0 {
		return dashboardOptions{}, errors.New("dashboard does not accept positional arguments")
	}

	opts.Access = strings.ToLower(strings.TrimSpace(opts.Access))
	if opts.Access == "" {
		opts.Access = "local"
	}
	if opts.Access != "local" && opts.Access != "ssh" {
		return dashboardOptions{}, fmt.Errorf("unsupported dashboard access mode %q", opts.Access)
	}

	addr, err := normalizeDashboardAddr(opts.Addr)
	if err != nil {
		return dashboardOptions{}, err
	}
	opts.Addr = addr
	return opts, nil
}

func listenDashboard(addr string) (net.Listener, error) {
	normalized, err := normalizeDashboardAddr(addr)
	if err != nil {
		return nil, err
	}
	return net.Listen("tcp", normalized)
}

func dashboardForwardTarget(addr net.Addr) (string, string) {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return "127.0.0.1", ""
	}
	host := "127.0.0.1"
	if tcpAddr.IP != nil && tcpAddr.IP.To4() == nil {
		host = "[" + tcpAddr.IP.String() + "]"
	} else if tcpAddr.IP != nil && !tcpAddr.IP.IsUnspecified() {
		host = tcpAddr.IP.String()
	}
	return host, strconv.Itoa(tcpAddr.Port)
}

func normalizeDashboardAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultDashboardAddr
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid dashboard addr %q: %w", addr, err)
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("invalid dashboard addr %q: missing port", addr)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid dashboard addr %q: invalid port", addr)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if !isLoopbackHost(host) {
		return "", fmt.Errorf("dashboard addr must bind to localhost, got %q", host)
	}
	return net.JoinHostPort(host, port), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newDashboardHandler(repoRoot string, opts dashboardOptions) http.Handler {
	mux := http.NewServeMux()
	health := dashboardHealth{
		OK:              true,
		Repo:            repoRoot,
		SessionsEnabled: opts.EnableSessions,
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(dashboardHTML))
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(health); err != nil {
			log.Printf("encode dashboard health: %v", err)
		}
	})

	return mux
}

func openBrowserURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>dotagents dashboard</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f6f7f8;
      --panel: #ffffff;
      --ink: #192026;
      --muted: #5c6770;
      --line: #d8dde3;
      --accent: #0f766e;
      --warn: #8a5a00;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #101418;
        --panel: #171d22;
        --ink: #eef2f5;
        --muted: #a6b0ba;
        --line: #2d363f;
        --accent: #2dd4bf;
        --warn: #f4bf50;
      }
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--ink);
    }
    header {
      border-bottom: 1px solid var(--line);
      background: var(--panel);
      padding: 18px 24px;
    }
    main {
      display: grid;
      gap: 16px;
      max-width: 1120px;
      margin: 0 auto;
      padding: 20px;
    }
    h1, h2, p { margin: 0; }
    h1 { font-size: 22px; line-height: 1.2; }
    h2 { font-size: 16px; }
    p { color: var(--muted); line-height: 1.45; }
    .topline {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      max-width: 1120px;
      margin: 0 auto;
    }
    .badge {
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 4px 10px;
      color: var(--accent);
      font-size: 13px;
      white-space: nowrap;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 12px;
    }
    section, .card {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      padding: 16px;
    }
    section {
      display: grid;
      gap: 14px;
    }
    .card {
      min-height: 118px;
      display: grid;
      align-content: start;
      gap: 8px;
    }
    code {
      display: block;
      overflow-wrap: anywhere;
      border: 1px solid var(--line);
      border-radius: 6px;
      padding: 10px;
      background: color-mix(in srgb, var(--panel), var(--ink) 4%);
      color: var(--ink);
    }
    .warn { color: var(--warn); }
  </style>
</head>
<body>
  <header>
    <div class="topline">
      <h1>dotagents dashboard</h1>
      <span class="badge">local-first MVP shell</span>
    </div>
  </header>
  <main>
    <section>
      <h2>Overview</h2>
      <p>This first slice proves the dashboard command, localhost server, static app shell, and health endpoint before live dotagents data is added.</p>
      <code>GET /api/health</code>
    </section>
    <section>
      <h2>Upcoming Views</h2>
      <div class="grid">
        <article class="card">
          <h2>Sync Health</h2>
          <p>Status, doctor output, MCP counts, and memory hook checks.</p>
        </article>
        <article class="card">
          <h2>Catalog</h2>
          <p>Searchable skills, native agents, MCP entries, hooks, and docs.</p>
        </article>
        <article class="card">
          <h2>Examples</h2>
          <p>Concrete daily workflow cards with copyable commands and prompts.</p>
        </article>
        <article class="card">
          <h2>Sessions</h2>
          <p><span class="warn">Disabled by default.</span> Future slices will require explicit allowlisted presets.</p>
        </article>
      </div>
    </section>
  </main>
</body>
</html>
`
