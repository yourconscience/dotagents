package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestSessionHubCanonicalHarnesses(t *testing.T) {
	var got []string
	for name, harness := range getHarnesses() {
		if harness.Sessions == nil {
			continue
		}
		if harness.Sessions.Store == "" || len(harness.Sessions.Resume) == 0 {
			t.Fatalf("%s has an incomplete session contract", name)
		}
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{"claude-code", "codex", "droid", "hermes", "omp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
