package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDashboardFlags(t *testing.T) {
	opts, err := parseDashboardFlags([]string{"--no-open", "--addr", ":0", "--access", "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.NoOpen {
		t.Fatal("NoOpen=false, want true")
	}
	if opts.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr=%q, want 127.0.0.1:0", opts.Addr)
	}
	if opts.Access != "ssh" {
		t.Fatalf("Access=%q, want ssh", opts.Access)
	}
}

func TestParseDashboardFlagsRejectsNonLoopback(t *testing.T) {
	if _, err := parseDashboardFlags([]string{"--addr", "0.0.0.0:8765"}); err == nil {
		t.Fatal("expected non-loopback addr to be rejected")
	}
}

func TestDashboardHandlerServesStaticShellAndHealth(t *testing.T) {
	handler := newDashboardHandler("/repo", dashboardOptions{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status=%d, want 200", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "dotagents dashboard") || !strings.Contains(got, "GET /api/health") {
		t.Fatalf("static shell missing expected content: %s", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health status=%d, want 200", rec.Code)
	}
	var health dashboardHealth
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if !health.OK || health.Repo != "/repo" || health.SessionsEnabled {
		t.Fatalf("health=%+v", health)
	}
}

func TestDashboardHealthReportsSessionsFlag(t *testing.T) {
	handler := newDashboardHandler("/repo", dashboardOptions{EnableSessions: true})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var health dashboardHealth
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if !health.SessionsEnabled {
		t.Fatalf("SessionsEnabled=false, want true")
	}
}

func TestDashboardForwardTargetUsesListenerHost(t *testing.T) {
	host, port := dashboardForwardTarget(&net.TCPAddr{IP: net.ParseIP("::1"), Port: 12345})
	if host != "[::1]" || port != "12345" {
		t.Fatalf("host=%q port=%q, want [::1] 12345", host, port)
	}

	host, port = dashboardForwardTarget(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 23456})
	if host != "127.0.0.1" || port != "23456" {
		t.Fatalf("host=%q port=%q, want 127.0.0.1 23456", host, port)
	}
}
