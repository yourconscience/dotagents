package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func printReport(mode string, repoRoot string, repoReport repoLinkReport, reports []agentReport, home string, cfg config) {
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
	if len(cfg.ExternalSkills) > 0 {
		cacheRoot := externalCacheDir(home)
		fmt.Println()
		fmt.Println("external sources:")
		for _, src := range cfg.ExternalSkills {
			name := repoName(src.URL)
			cachePath := filepath.Join(cacheRoot, name)
			state := "not cloned"
			if hasDir(filepath.Join(cachePath, ".git")) {
				commit := externalSkillCommit(cachePath)
				state = fmt.Sprintf("synced (%s)", commit)
			}
			fmt.Printf("  %s  %s  %s\n", name, src.URL, state)
		}
	}
	if len(cfg.Plugins) > 0 {
		fmt.Println()
		fmt.Println("plugins:")
		for _, plugin := range cfg.Plugins {
			state := "example"
			if plugin.Enabled {
				state = "enabled"
			}
			fmt.Printf("  %s  %s  %s  %s\n", plugin.Name, state, plugin.Format, strings.Join(plugin.Surfaces, ","))
			fmt.Printf("    compatibility: %s\n", pluginCompatibilitySummary(plugin))
			if plugin.Review != "" {
				fmt.Printf("    review: %s\n", plugin.Review)
			}
		}
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
		if report.Delivery != "" && report.Delivery != deliverySync {
			fmt.Printf("  delivery: %s\n", report.Delivery)
		}
		if h := harnessFor(report.Name); h != nil && h.IntegrationNote != "" {
			fmt.Printf("  integration: %s\n", h.IntegrationNote)
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
		if mode == "status" {
			listingBytes := skillListingBytes(report.ExpectedSkills)
			fmt.Printf("  skill listing context: %d skills, %d bytes name+desc, %s\n", len(report.ExpectedSkills), listingBytes, formatTokenEstimate(estimateTokens(listingBytes)))
		}
		fmt.Printf("  managed (%d): %s\n", len(report.Managed), displayList(report.Managed))
		if len(report.ManagedAgent)+len(report.MissingAgent)+len(report.DriftedAgent) > 0 {
			fmt.Printf("  agent managed (%d): %s\n", len(report.ManagedAgent), displayList(report.ManagedAgent))
		}
		fmt.Printf("  external (%d): %s\n", len(report.External), displayList(report.External))
		if len(report.ManagedMCP)+len(report.MissingMCP)+len(report.DriftedMCP) > 0 {
			fmt.Printf("  mcp managed (%d): %s\n", len(report.ManagedMCP), displayList(report.ManagedMCP))
		}
		if len(report.ManagedHook)+len(report.MissingHook)+len(report.DriftedHook)+len(report.UnsupportedHook) > 0 {
			fmt.Printf("  hook managed (%d): %s\n", len(report.ManagedHook), displayList(report.ManagedHook))
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
		if len(report.MissingHook) > 0 {
			fmt.Printf("  hook missing (%d): %s\n", len(report.MissingHook), displayList(report.MissingHook))
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
		if len(report.DriftedHook) > 0 {
			fmt.Printf("  hook drifted (%d): %s\n", len(report.DriftedHook), displayList(report.DriftedHook))
		}
		if len(report.UnsupportedHook) > 0 {
			fmt.Printf("  hook unsupported (%d): %s\n", len(report.UnsupportedHook), displayList(report.UnsupportedHook))
		}
		if len(report.StaleManaged) > 0 {
			fmt.Printf("  stale managed (%d): %s\n", len(report.StaleManaged), displayList(report.StaleManaged))
		}
		if len(report.Conflicts) > 0 {
			fmt.Printf("  conflicts (%d): %s\n", len(report.Conflicts), displayList(report.Conflicts))
		}
		if mode == "sync" {
			fmt.Printf("  sync actions: add=%d update=%d remove=%d agent-add=%d agent-update=%d agent-remove=%d mcp-add=%d mcp-update=%d hook-add=%d hook-update=%d\n", len(report.Adds), len(report.Updates), len(report.Removes), len(report.AddsAgent), len(report.UpdatesAgent), len(report.RemovesAgent), len(report.AddsMCP), len(report.UpdatesMCP), len(report.AddsHook), len(report.UpdatesHook))
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
	sort.Strings(report.ManagedHook)
	sort.Strings(report.Drifted)
	sort.Strings(report.DriftedAgent)
	sort.Strings(report.DriftedMCP)
	sort.Strings(report.DriftedHook)
	sort.Strings(report.Missing)
	sort.Strings(report.MissingAgent)
	sort.Strings(report.MissingMCP)
	sort.Strings(report.MissingHook)
	sort.Strings(report.UnsupportedHook)
	sort.Strings(report.Conflicts)
	sort.Strings(report.StaleManaged)
	sort.Strings(report.External)
	sort.Strings(report.Adds)
	sort.Strings(report.AddsAgent)
	sort.Strings(report.AddsMCP)
	sort.Strings(report.AddsHook)
	sort.Strings(report.Updates)
	sort.Strings(report.UpdatesAgent)
	sort.Strings(report.UpdatesMCP)
	sort.Strings(report.UpdatesHook)
	sort.Strings(report.Removes)
	sort.Strings(report.RemovesAgent)
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
			current[i].AddsHook = append([]string{}, original.AddsHook...)
			current[i].Updates = append([]string{}, original.Updates...)
			current[i].UpdatesAgent = append([]string{}, original.UpdatesAgent...)
			current[i].RemovesAgent = append([]string{}, original.RemovesAgent...)
			current[i].UpdatesMCP = append([]string{}, original.UpdatesMCP...)
			current[i].UpdatesHook = append([]string{}, original.UpdatesHook...)
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
