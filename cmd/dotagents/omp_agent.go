package main

import "strings"

var ompToolMapping = map[string]string{
	"bash":      "bash",
	"edit":      "edit",
	"glob":      "glob",
	"grep":      "grep",
	"read":      "read",
	"webfetch":  "read",
	"websearch": "web_search",
	"write":     "write",
}

func renderOMPAgentRole(role agentRole) string {
	var b strings.Builder
	b.WriteString("---\n")
	writeYAMLScalar(&b, "name", role.Name)
	writeYAMLScalar(&b, "description", role.Description)
	if model := strings.TrimSpace(role.Model); model != "" {
		b.WriteString("model:\n")
		writeYAMLListItem(&b, model)
	}
	if tools := ompToolsFor(role.Tools); len(tools) > 0 {
		b.WriteString("tools:\n")
		for _, tool := range tools {
			writeYAMLListItem(&b, tool)
		}
	}
	b.WriteString("---\n\n")
	b.WriteString("<!-- ")
	b.WriteString(generatedAgentMarker)
	b.WriteString(" from agents/")
	b.WriteString(agentRolesFileName)
	b.WriteString("; do not edit directly. -->\n\n")
	b.WriteString(role.Instructions)
	b.WriteString("\n")
	return b.String()
}

func ompToolsFor(tools []string) []string {
	out := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		mapped := ompToolMapping[strings.ToLower(strings.TrimSpace(tool))]
		if mapped == "" {
			continue
		}
		if _, ok := seen[mapped]; ok {
			continue
		}
		seen[mapped] = struct{}{}
		out = append(out, mapped)
	}
	return out
}
