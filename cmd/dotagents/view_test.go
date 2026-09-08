package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestHKServeArgs(t *testing.T) {
	if got := hkServeArgs(nil); !reflect.DeepEqual(got, []string{"serve"}) {
		t.Fatalf("hkServeArgs(nil) = %v, want [serve]", got)
	}
	got := hkServeArgs([]string{"--port", "8080"})
	want := []string{"serve", "--port", "8080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hkServeArgs passthrough = %v, want %v", got, want)
	}
}

func TestRunViewMissingBinary(t *testing.T) {
	orig := hkLookPath
	t.Cleanup(func() { hkLookPath = orig })
	hkLookPath = func(string) (string, error) { return "", errors.New("not found") }

	err := runView(nil)
	if err == nil {
		t.Fatal("expected error when hk binary is missing")
	}
	if !strings.Contains(err.Error(), "HarnessKit") || !strings.Contains(err.Error(), "github.com/RealZST/HarnessKit") {
		t.Fatalf("error should guide install, got: %v", err)
	}
}

func TestParseViewArgs(t *testing.T) {
	opts, passthrough, err := parseViewArgs([]string{"--no-open", "--ssh-host", "kirill@box", "--port", "8080", "--no-token"})
	if err != nil {
		t.Fatalf("parseViewArgs error: %v", err)
	}
	if !opts.NoOpen {
		t.Fatal("--no-open not parsed")
	}
	if opts.SSHHost != "kirill@box" {
		t.Fatalf("SSHHost = %q, want kirill@box", opts.SSHHost)
	}
	if want := []string{"--port", "8080", "--no-token"}; !reflect.DeepEqual(passthrough, want) {
		t.Fatalf("passthrough = %v, want %v", passthrough, want)
	}
}

func TestParseViewArgsSSHHostEquals(t *testing.T) {
	opts, passthrough, err := parseViewArgs([]string{"--ssh-host=user@1.2.3.4"})
	if err != nil {
		t.Fatalf("parseViewArgs error: %v", err)
	}
	if opts.SSHHost != "user@1.2.3.4" {
		t.Fatalf("SSHHost = %q", opts.SSHHost)
	}
	if len(passthrough) != 0 {
		t.Fatalf("passthrough = %v, want empty", passthrough)
	}
}

func TestParseViewArgsSSHHostMissingValue(t *testing.T) {
	if _, _, err := parseViewArgs([]string{"--ssh-host"}); err == nil {
		t.Fatal("expected error for --ssh-host without a value")
	}
}

func TestExtractServeURL(t *testing.T) {
	line := "HarnessKit Web UI [host] running at http://127.0.0.1:7070/?token=abc123"
	url, ok := extractServeURL(line)
	if !ok || url != "http://127.0.0.1:7070/?token=abc123" {
		t.Fatalf("extractServeURL = %q, %v", url, ok)
	}
	if _, ok := extractServeURL("Auth token: abc123"); ok {
		t.Fatal("extractServeURL should not match a non-URL line")
	}
}

func TestPortFromURL(t *testing.T) {
	if got := portFromURL("http://127.0.0.1:7070/?token=x"); got != "7070" {
		t.Fatalf("portFromURL = %q, want 7070", got)
	}
	if got := portFromURL("http://example.com/path"); got != "" {
		t.Fatalf("portFromURL without port = %q, want empty", got)
	}
}

func TestResolveSSHHost(t *testing.T) {
	explicit := func(string) string { return "" }
	if got := resolveSSHHost("user@host", explicit); got != "user@host" {
		t.Fatalf("explicit host = %q", got)
	}

	env := func(k string) string {
		switch k {
		case "SSH_CONNECTION":
			return "10.0.0.5 51234 10.0.0.9 22"
		case "USER":
			return "kirill"
		}
		return ""
	}
	if got := resolveSSHHost("", env); got != "kirill@10.0.0.9" {
		t.Fatalf("derived host = %q, want kirill@10.0.0.9", got)
	}

	none := func(string) string { return "" }
	if got := resolveSSHHost("", none); got != "" {
		t.Fatalf("no SSH session host = %q, want empty", got)
	}
}

func TestTunnelCommand(t *testing.T) {
	cmd, ok := tunnelCommand("http://127.0.0.1:7070/?token=x", "user@host")
	if !ok || cmd != "ssh -L 7070:localhost:7070 user@host" {
		t.Fatalf("tunnelCommand = %q, %v", cmd, ok)
	}
	if _, ok := tunnelCommand("http://127.0.0.1:7070/", ""); ok {
		t.Fatal("tunnelCommand should fail without a host")
	}
}

func TestHKBannerNoise(t *testing.T) {
	if !hkBannerNoise("Access via SSH tunnel: ssh -L 7070:localhost:7070 your-server") {
		t.Fatal("tunnel line should be suppressible")
	}
	if !hkBannerNoise("Auth token: abc123") {
		t.Fatal("token line should be suppressible")
	}
	if hkBannerNoise("some runtime log line") {
		t.Fatal("ordinary lines must not be suppressed")
	}
}

func TestAnnounceViewLocalOpensBrowser(t *testing.T) {
	orig := openInBrowser
	t.Cleanup(func() { openInBrowser = orig })
	var opened string
	openInBrowser = func(url string) error { opened = url; return nil }

	var b strings.Builder
	url := "http://127.0.0.1:7070/?token=abc"
	announceView(&b, url, viewOptions{}, false, "")

	out := b.String()
	if !strings.Contains(out, url) {
		t.Fatalf("URL not printed on its own: %q", out)
	}
	if opened != url {
		t.Fatalf("browser not opened, got %q", opened)
	}
	if !strings.Contains(out, "default browser") {
		t.Fatalf("missing browser notice: %q", out)
	}
}

func TestAnnounceViewNoOpen(t *testing.T) {
	orig := openInBrowser
	t.Cleanup(func() { openInBrowser = orig })
	called := false
	openInBrowser = func(string) error { called = true; return nil }

	var b strings.Builder
	announceView(&b, "http://127.0.0.1:7070/?token=abc", viewOptions{NoOpen: true}, false, "")

	if called {
		t.Fatal("--no-open must not open a browser")
	}
	if !strings.Contains(b.String(), "http://127.0.0.1:7070/?token=abc") {
		t.Fatal("URL should still be printed with --no-open")
	}
}

func TestAnnounceViewRemoteShowsTunnelNotBrowser(t *testing.T) {
	orig := openInBrowser
	t.Cleanup(func() { openInBrowser = orig })
	called := false
	openInBrowser = func(string) error { called = true; return nil }

	var b strings.Builder
	announceView(&b, "http://127.0.0.1:7070/?token=abc", viewOptions{}, true, "kirill@10.0.0.9")

	out := b.String()
	if called {
		t.Fatal("remote host must not open a local browser")
	}
	if !strings.Contains(out, "ssh -L 7070:localhost:7070 kirill@10.0.0.9") {
		t.Fatalf("missing tunnel command: %q", out)
	}
}
