package main

import (
	"reflect"
	"testing"
)

func TestSearchArgsInjectsCollection(t *testing.T) {
	got := searchArgs([]string{"cafe preferences"}, "ai")
	want := []string{"search", "--collection", "ai", "cafe preferences"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSearchArgsRespectsExplicitCollection(t *testing.T) {
	for _, flag := range []string{"-c", "--collection"} {
		got := searchArgs([]string{"query", flag, "other"}, "ai")
		want := []string{"search", "query", flag, "other"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("flag %s: got %v, want %v", flag, got, want)
		}
	}
}
