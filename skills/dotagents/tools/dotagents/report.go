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
		if report.Name == agentHermes {
			fmt.Println("  integration: config-driven via ~/.hermes/config.yaml -> skills.external_dirs")
		}
		if report.Synced {
			fmt.Println("  sync: synced")
		} else {
			fmt.Println("  sync: drifted")
		}
		fmt.Printf("  managed (%d): %s\n", len(report.Managed), displayList(report.Managed))
		fmt.Printf("  external (%d): %s\n", len(report.External), displayList(report.External))
		if len(report.ManagedMCP)+len(report.MissingMCP)+len(report.DriftedMCP) > 0 {
			fmt.Printf("  mcp managed (%d): %s\n", len(report.ManagedMCP), displayList(report.ManagedMCP))
		}
		if len(report.Missing) > 0 {
			fmt.Printf("  missing (%d): %s\n", len(report.Missing), displayList(report.Missing))
		}
		if len(report.MissingMCP) > 0 {
			fmt.Printf("  mcp missing (%d): %s\n", len(report.MissingMCP), displayList(report.MissingMCP))
		}
		if len(report.Drifted) > 0 {
			fmt.Printf("  drifted (%d): %s\n", len(report.Drifted), displayList(report.Drifted))
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
			fmt.Printf("  sync actions: add=%d update=%d remove=%d mcp-add=%d mcp-update=%d\n", len(report.Adds), len(report.Updates), len(report.Removes), len(report.AddsMCP), len(report.UpdatesMCP))
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
	sort.Strings(report.ManagedMCP)
	sort.Strings(report.Drifted)
	sort.Strings(report.DriftedMCP)
	sort.Strings(report.Missing)
	sort.Strings(report.MissingMCP)
	sort.Strings(report.Conflicts)
	sort.Strings(report.StaleManaged)
	sort.Strings(report.External)
	sort.Strings(report.Adds)
	sort.Strings(report.AddsMCP)
	sort.Strings(report.Updates)
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
			current[i].AddsMCP = append([]string{}, original.AddsMCP...)
			current[i].Updates = append([]string{}, original.Updates...)
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
