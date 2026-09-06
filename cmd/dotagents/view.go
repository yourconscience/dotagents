package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// hkBinary is the HarnessKit CLI that `dotagents view` launches as a
// cross-harness inspection surface over the materialized native harness dirs.
// The `view` command itself never writes. dotagents does NOT enforce read-only:
// the launched HarnessKit UI can enable/disable/deploy, and those writes go
// straight to native dirs, bypassing dotagents. The launch banner warns against
// using them on managed surfaces; reconcile any drift with `dotagents sync`.
const hkBinary = "hk"

// hkLookPath is indirected so tests can exercise the missing-binary path
// without depending on the host PATH.
var hkLookPath = exec.LookPath

const hkInstallHint = `HarnessKit (hk) not found on PATH.

dotagents view launches HarnessKit as a cross-harness inspection surface for
skills, MCP servers, hooks, and configs across every detected agent.

Install it from https://github.com/RealZST/HarnessKit, then re-run: dotagents view`

// hkServeArgs builds the argv for the underlying `hk serve` invocation. Extra
// args are forwarded verbatim to hk serve (e.g. --port, --host, --no-token).
func hkServeArgs(passthrough []string) []string {
	return append([]string{"serve"}, passthrough...)
}

// runView starts `hk serve` (forwarding its stdio, which prints the tokenized
// URL) so the user can inspect every harness in HarnessKit's web UI. The
// launcher itself writes nothing; the banner cautions that HarnessKit's own
// write actions bypass dotagents.
func runView(args []string) error {
	path, err := hkLookPath(hkBinary)
	if err != nil {
		return errors.New(hkInstallHint)
	}
	fmt.Fprintln(os.Stdout, "Launching HarnessKit for inspection. Note: HarnessKit can also enable/disable/deploy, and those writes bypass dotagents — avoid them on dotagents-managed skills, MCP, and hooks (reconcile drift with: dotagents sync).")
	cmd := exec.Command(path, hkServeArgs(args)...) // nosemgrep: go.lang.security.audit.dangerous-exec-command
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
