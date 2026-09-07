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
