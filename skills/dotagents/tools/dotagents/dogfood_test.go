package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardDogfood(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := runDashboardDogfood(); err != nil {
		t.Fatal(err)
	}
}

func TestDogfoodGetContainsReportsMissingText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	err := dogfoodGetContains(server.Client(), server.URL, "missing")
	if err == nil {
		t.Fatal("expected missing text error")
	}
}

func TestDashboardDogfoodURL(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if got := dashboardDogfoodURL(listener); got == "" || got[:7] != "http://" {
		t.Fatalf("dashboardDogfoodURL=%q", got)
	}
}
