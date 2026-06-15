package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runSources(args []string) error {
	fs := flag.NewFlagSet("sources", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "output JSON")
	compactFlag := fs.Bool("compact", false, "one-line output for skill consumption")
	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	repoRoot, _, err := findRoots()
	if err != nil {
		return fmt.Errorf("find roots: %w", err)
	}

	cfg, err := loadConfig(repoRoot, home, *configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	statuses := resolveSourceStatus(cfg, home)

	// Single source detail mode
	if fs.NArg() > 0 {
		name := fs.Arg(0)
		for _, ss := range statuses {
			if ss.Name == name {
				if *jsonFlag {
					return printJSON(ss)
				}
				return printSourceDetail(ss, home)
			}
		}
		return fmt.Errorf("unknown source %q", name)
	}

	if *jsonFlag {
		return printJSON(statuses)
	}
	if *compactFlag {
		return printCompact(statuses)
	}
	return printTable(statuses)
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printCompact(statuses []sourceStatus) error {
	var parts []string
	for _, ss := range statuses {
		if !ss.Enabled || ss.Best == "" {
			continue
		}
		parts = append(parts, ss.Name+":"+ss.Best)
	}
	fmt.Println(strings.Join(parts, " "))
	return nil
}

func printTable(statuses []sourceStatus) error {
	for _, ss := range statuses {
		name := padRight(ss.Name, 16)
		if !ss.Enabled {
			fmt.Printf("%s  disabled\n", name)
			continue
		}
		if len(ss.Methods) == 0 {
			fmt.Printf("%s  not integrated\n", name)
			continue
		}

		best := padRight(ss.Best, 16)

		var methodParts []string
		var unavailable []string
		for _, ms := range ss.Methods {
			if ms.Available {
				tag := "ok"
				if ms.Type == methodFallback {
					tag = "fallback"
				}
				methodParts = append(methodParts, fmt.Sprintf("%s [%s]", ms.Name, tag))
			} else {
				reason := ms.Reason
				if reason == "" {
					reason = "unavailable"
				}
				unavailable = append(unavailable, fmt.Sprintf("%s: %s", ms.Name, reason))
			}
		}

		methods := strings.Join(methodParts, "  ")
		line := fmt.Sprintf("%s  %s  %s", name, best, methods)
		if len(unavailable) > 0 {
			line += "  (" + strings.Join(unavailable, "; ") + ")"
		}
		fmt.Println(line)
	}
	return nil
}

func printSourceDetail(ss sourceStatus, home string) error {
	def := findSourceDef(ss.Name)
	if def == nil {
		return fmt.Errorf("no definition for %q", ss.Name)
	}

	fmt.Printf("%s  (%s)\n", ss.Name, def.Desc)
	fmt.Printf("  enabled:   %v\n", ss.Enabled)
	if def.ToSRisk != "" {
		fmt.Printf("  tos-risk:  %s\n", def.ToSRisk)
	}
	if ss.Best != "" {
		fmt.Printf("  best:      %s\n", ss.Best)
	}
	fmt.Println()

	if len(def.Methods) == 0 {
		fmt.Println("  no methods (not integrated)")
		return nil
	}

	fmt.Println("  methods:")
	for i, m := range def.Methods {
		status := "NOT AVAILABLE"
		reason := ""
		if i < len(ss.Methods) {
			if ss.Methods[i].Available {
				status = "OK"
			} else {
				reason = ss.Methods[i].Reason
			}
		}

		marker := " "
		if m.Name == ss.Best {
			marker = "*"
		}
		fmt.Printf("  %s [%d] %-16s %-14s %s\n", marker, m.Priority, m.Name, status, reason)

		if m.Auth != "" {
			fmt.Printf("         auth:  %s\n", expandAuthPath(m.Auth, home))
		}
		if m.Setup != "" {
			fmt.Printf("         setup: %s\n", m.Setup)
		}
	}
	return nil
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
