package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func printReport(mode string, repoRoot string, repoReport repoLinkReport, reports []agentReport) {
	fmt.Printf("dotagents %s\n", mode)
	fmt.Printf("repo: %s\n", repoRoot)
	fmt.Printf("~/.agents: %s", repoReport.State)
	if repoReport.State == stateSynced {
		fmt.Printf(" -> %s\n", repoReport.ExpectedTarget)
	} else {
		fmt.Printf(" (expected %s", repoReport.ExpectedTarget)
		if repoReport.ActualTarget != "" {
			fmt.Printf(", actual %s", repoReport.ActualTarget)
		}
		fmt.Printf(")\n")
	}
	fmt.Println()

	for _, report := range reports {
		fmt.Printf("%s\n", report.Name)
		if !report.Detected {
			fmt.Println("  not detected (binary not on PATH)")
			fmt.Println()
			continue
		}
		fmt.Printf("  skill root: %s\n", report.SkillRoot)
		if report.AgentRoot != "" {
			fmt.Printf("  agent root: %s\n", report.AgentRoot)
		}
		if report.Name == agentHermes {
			fmt.Println("  integration: config-driven via ~/.hermes/config.yaml -> skills.external_dirs")
		}
		if report.RootPath != "" {
			fmt.Printf("  root instructions: %s", report.RootState)
			if report.RootState == stateSynced {
				fmt.Printf(" -> %s\n", report.RootExpected)
			} else {
				fmt.Printf(" (expected %s", report.RootExpected)
				if report.RootActual != "" {
					fmt.Printf(", actual %s", report.RootActual)
				}
				fmt.Printf(")\n")
			}
		}
		if report.Synced {
			fmt.Println("  sync: synced")
		} else {
			fmt.Println("  sync: drifted")
		}
		fmt.Printf("  managed (%d): %s\n", len(report.Managed), displayList(report.Managed))
		if len(report.ManagedAgent)+len(report.MissingAgent)+len(report.DriftedAgent) > 0 {
			fmt.Printf("  agent managed (%d): %s\n", len(report.ManagedAgent), displayList(report.ManagedAgent))
		}
		fmt.Printf("  external (%d): %s\n", len(report.External), displayList(report.External))
		if len(report.ManagedMCP)+len(report.MissingMCP)+len(report.DriftedMCP) > 0 {
			fmt.Printf("  mcp managed (%d): %s\n", len(report.ManagedMCP), displayList(report.ManagedMCP))
		}
		if len(report.Missing) > 0 {
			fmt.Printf("  missing (%d): %s\n", len(report.Missing), displayList(report.Missing))
		}
		if len(report.MissingAgent) > 0 {
			fmt.Printf("  agent missing (%d): %s\n", len(report.MissingAgent), displayList(report.MissingAgent))
		}
		if len(report.MissingMCP) > 0 {
			fmt.Printf("  mcp missing (%d): %s\n", len(report.MissingMCP), displayList(report.MissingMCP))
		}
		if len(report.Drifted) > 0 {
			fmt.Printf("  drifted (%d): %s\n", len(report.Drifted), displayList(report.Drifted))
		}
		if len(report.DriftedAgent) > 0 {
			fmt.Printf("  agent drifted (%d): %s\n", len(report.DriftedAgent), displayList(report.DriftedAgent))
		}
		if len(report.DriftedMCP) > 0 {
			fmt.Printf("  mcp drifted (%d): %s\n", len(report.DriftedMCP), displayList(report.DriftedMCP))
		}
		if len(report.StaleManaged) > 0 {
			fmt.Printf("  stale managed (%d): %s\n", len(report.StaleManaged), displayList(report.StaleManaged))
		}
		if len(report.Conflicts) > 0 {
			fmt.Printf("  conflicts (%d): %s\n", len(report.Conflicts), displayList(report.Conflicts))
		}
		if mode == "sync" {
			fmt.Printf("  sync actions: add=%d update=%d remove=%d agent-add=%d agent-update=%d mcp-add=%d mcp-update=%d\n", len(report.Adds), len(report.Updates), len(report.Removes), len(report.AddsAgent), len(report.UpdatesAgent), len(report.AddsMCP), len(report.UpdatesMCP))
		}
		fmt.Println()
	}
}

func displayList(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func sortReportLists(report *agentReport) {
	sort.Strings(report.Managed)
	sort.Strings(report.ManagedAgent)
	sort.Strings(report.ManagedMCP)
	sort.Strings(report.Drifted)
	sort.Strings(report.DriftedAgent)
	sort.Strings(report.DriftedMCP)
	sort.Strings(report.Missing)
	sort.Strings(report.MissingAgent)
	sort.Strings(report.MissingMCP)
	sort.Strings(report.Conflicts)
	sort.Strings(report.StaleManaged)
	sort.Strings(report.External)
	sort.Strings(report.Adds)
	sort.Strings(report.AddsAgent)
	sort.Strings(report.AddsMCP)
	sort.Strings(report.Updates)
	sort.Strings(report.UpdatesAgent)
	sort.Strings(report.UpdatesMCP)
	sort.Strings(report.Removes)
}

func cloneReports(reports []agentReport) []agentReport {
	cloned := make([]agentReport, len(reports))
	copy(cloned, reports)
	return cloned
}

func restoreSyncActions(current []agentReport, preflight []agentReport) {
	index := make(map[string]agentReport, len(preflight))
	for _, report := range preflight {
		index[report.Name] = report
	}
	for i := range current {
		if original, ok := index[current[i].Name]; ok {
			current[i].Adds = append([]string{}, original.Adds...)
			current[i].AddsAgent = append([]string{}, original.AddsAgent...)
			current[i].AddsMCP = append([]string{}, original.AddsMCP...)
			current[i].Updates = append([]string{}, original.Updates...)
			current[i].UpdatesAgent = append([]string{}, original.UpdatesAgent...)
			current[i].UpdatesMCP = append([]string{}, original.UpdatesMCP...)
			current[i].Removes = append([]string{}, original.Removes...)
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func hasDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func hasFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
