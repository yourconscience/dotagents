package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

type sessionHubHarness struct {
	Name   string   `json:"name"`
	Store  string   `json:"store"`
	Resume []string `json:"resume"`
}

func runSessionHub(args []string) error {
	if len(args) != 2 || args[0] != "harnesses" || args[1] != "--json" {
		return errors.New("usage: dotagents session-hub harnesses --json")
	}
	registry := getHarnesses()
	out := make([]sessionHubHarness, 0, len(registry))
	for name, harness := range registry {
		if harness.Sessions == nil {
			continue
		}
		out = append(out, sessionHubHarness{Name: name, Store: harness.Sessions.Store, Resume: harness.Sessions.Resume})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"harnesses": out}); err != nil {
		return fmt.Errorf("encode session-hub contract: %w", err)
	}
	return nil
}
