package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const pluginManifestSchemaID = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

var pluginManifestKnownFields = map[string]bool{
	"$schema":     true,
	"name":        true,
	"version":     true,
	"description": true,
	"author":      true,
	"homepage":    true,
	"repository":  true,
	"license":     true,
	"keywords":    true,
	"extensions":  true,
}

type pluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

type pluginManifest struct {
	Schema      string
	Name        string
	Version     string
	Description string
	Author      *pluginAuthor
	Homepage    string
	Repository  string
	License     string
	Keywords    []string
}

func hasPluginManifest(cachePath string) bool {
	return hasFile(filepath.Join(cachePath, "plugin.json"))
}

func parsePluginManifest(pluginRoot string) (pluginManifest, []string, bool, error) {
	path := filepath.Join(pluginRoot, "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return pluginManifest{}, nil, false, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return pluginManifest{}, nil, false, fmt.Errorf("invalid JSON: %w", err)
	}

	var unknown []string
	for field := range raw {
		if !pluginManifestKnownFields[field] {
			unknown = append(unknown, field)
		}
	}

	var schemaStr string
	if rawSchema, ok := raw["$schema"]; ok {
		if err := json.Unmarshal(rawSchema, &schemaStr); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("$schema must be a string")
		}
	}
	if schemaStr != pluginManifestSchemaID {
		if schemaStr == "" {
			return pluginManifest{}, unknown, false, fmt.Errorf("missing required $schema")
		}
		return pluginManifest{}, unknown, false, fmt.Errorf("unsupported $schema %q", schemaStr)
	}

	var nameStr string
	if rawName, ok := raw["name"]; ok {
		if err := json.Unmarshal(rawName, &nameStr); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("name must be a string")
		}
	}
	if nameStr == "" {
		return pluginManifest{}, unknown, false, fmt.Errorf("missing required name")
	}
	if len(nameStr) > 64 {
		return pluginManifest{}, unknown, false, fmt.Errorf("name exceeds 64 characters")
	}
	if !pluginNamePattern.MatchString(nameStr) {
		return pluginManifest{}, unknown, false, fmt.Errorf("name %q does not match required pattern", nameStr)
	}
	if strings.Contains(nameStr, "--") {
		return pluginManifest{}, unknown, false, fmt.Errorf("name %q contains consecutive hyphens", nameStr)
	}
	if strings.Contains(nameStr, "..") {
		return pluginManifest{}, unknown, false, fmt.Errorf("name %q contains consecutive periods", nameStr)
	}

	m := pluginManifest{
		Schema: schemaStr,
		Name:   nameStr,
	}

	if rawVersion, ok := raw["version"]; ok {
		if err := json.Unmarshal(rawVersion, &m.Version); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("version must be a string")
		}
	}
	if rawDesc, ok := raw["description"]; ok {
		if err := json.Unmarshal(rawDesc, &m.Description); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("description must be a string")
		}
	}
	if rawHomepage, ok := raw["homepage"]; ok {
		if err := json.Unmarshal(rawHomepage, &m.Homepage); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("homepage must be a string")
		}
	}
	if rawRepo, ok := raw["repository"]; ok {
		if err := json.Unmarshal(rawRepo, &m.Repository); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("repository must be a string")
		}
	}
	if rawLicense, ok := raw["license"]; ok {
		if err := json.Unmarshal(rawLicense, &m.License); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("license must be a string")
		}
	}
	if rawKeywords, ok := raw["keywords"]; ok {
		if err := json.Unmarshal(rawKeywords, &m.Keywords); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("keywords must be an array of strings")
		}
	}

	if rawAuthor, ok := raw["author"]; ok {
		var authorMap map[string]json.RawMessage
		if err := json.Unmarshal(rawAuthor, &authorMap); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("author must be an object")
		}
		authorKnown := map[string]bool{"name": true, "email": true, "url": true}
		for field := range authorMap {
			if !authorKnown[field] {
				return pluginManifest{}, unknown, false, fmt.Errorf("author contains unknown field %q", field)
			}
		}
		var author pluginAuthor
		if err := json.Unmarshal(rawAuthor, &author); err != nil {
			return pluginManifest{}, unknown, false, fmt.Errorf("author decode error: %w", err)
		}
		m.Author = &author
	}

	if rawExt, ok := raw["extensions"]; ok {
		var extObj map[string]json.RawMessage
		if err := json.Unmarshal(rawExt, &extObj); err != nil {
			unknown = append(unknown, "extensions (non-object, ignored per section 8.1)")
		}
	}

	return m, unknown, true, nil
}
