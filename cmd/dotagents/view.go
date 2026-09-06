package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// hkBinary is the HarnessKit CLI that `dotagents view` launches as a read-only
// cross-harness inspector. dotagents stays the only writer of the five managed
// surfaces; HarnessKit is consumed purely as a viewer over the materialized
// native harness directories, so `view` never mutates managed config.
const hkBinary = "hk"

// hkLookPath is indirected so tests can exercise the missing-binary path
// without depending on the host PATH.
var hkLookPath = exec.LookPath

const hkInstallHint = `HarnessKit (hk) not found on PATH.

dotagents view launches HarnessKit as a read-only cross-harness inspector for
skills, MCP servers, hooks, and configs across every detected agent.

Install it from https://github.com/RealZST/HarnessKit, then re-run: dotagents view`

// hkServeArgs builds the argv for the underlying `hk serve` invocation. Extra
// args are forwarded verbatim to hk serve (e.g. --port, --host, --no-token).
func hkServeArgs(passthrough []string) []string {
	return append([]string{"serve"}, passthrough...)
}

// runView launches the HarnessKit web UI over the materialized native harness
// dirs. Read-only by intent: dotagents remains the source of truth, so this
// command inspects but never writes managed surfaces.
func runView(args []string) error {
	path, err := hkLookPath(hkBinary)
	if err != nil {
		return errors.New(hkInstallHint)
	}
	fmt.Fprintln(os.Stdout, "Launching HarnessKit (read-only). dotagents stays the source of truth — avoid HarnessKit's enable/disable/deploy actions on dotagents-managed skills, MCP, and hooks.")
	cmd := exec.Command(path, hkServeArgs(args)...) // nosemgrep: go.lang.security.audit.dangerous-exec-command
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
