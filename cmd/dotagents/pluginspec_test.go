package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writePluginJSON(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParsePluginManifestMinimal(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "minimal-plugin"
	}`)

	m, unknown, ok, err := parsePluginManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(unknown) != 0 {
		t.Errorf("expected no unknown fields, got %v", unknown)
	}
	if m.Name != "minimal-plugin" {
		t.Errorf("name = %q, want %q", m.Name, "minimal-plugin")
	}
	if m.Schema != pluginManifestSchemaID {
		t.Errorf("schema = %q, want %q", m.Schema, pluginManifestSchemaID)
	}
}

func TestParsePluginManifestFull(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "full-plugin",
		"version": "1.2.0",
		"description": "A full plugin",
		"author": {"name": "Test Author", "email": "test@example.com", "url": "https://example.com"},
		"homepage": "https://docs.example.com",
		"repository": "https://github.com/example/full-plugin",
		"license": "MIT",
		"keywords": ["test", "full"]
	}`)

	m, _, ok, err := parsePluginManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if m.Name != "full-plugin" {
		t.Errorf("name = %q", m.Name)
	}
	if m.Version != "1.2.0" {
		t.Errorf("version = %q", m.Version)
	}
	if m.Description != "A full plugin" {
		t.Errorf("description = %q", m.Description)
	}
	if m.Author == nil || m.Author.Name != "Test Author" {
		t.Errorf("author = %+v", m.Author)
	}
	if m.License != "MIT" {
		t.Errorf("license = %q", m.License)
	}
	if len(m.Keywords) != 2 {
		t.Errorf("keywords = %v", m.Keywords)
	}
}

func TestParsePluginManifestMissingSchema(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{"name": "no-schema"}`)

	_, _, ok, _ := parsePluginManifest(dir)
	if ok {
		t.Fatal("expected ok=false for missing $schema")
	}
}

func TestParsePluginManifestWrongSchema(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://example.com/wrong.json",
		"name": "wrong-schema"
	}`)

	_, _, ok, _ := parsePluginManifest(dir)
	if ok {
		t.Fatal("expected ok=false for wrong $schema")
	}
}

func TestParsePluginManifestMissingName(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	}`)

	_, _, ok, _ := parsePluginManifest(dir)
	if ok {
		t.Fatal("expected ok=false for missing name")
	}
}

func TestParsePluginManifestInvalidNames(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"uppercase", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"My-Plugin"}`},
		{"leading hyphen", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"-start"}`},
		{"consecutive hyphens", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"has--double"}`},
		{"consecutive periods", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"too..dots"}`},
		{"empty", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writePluginJSON(t, dir, tt.json)
			_, _, ok, _ := parsePluginManifest(dir)
			if ok {
				t.Fatalf("expected ok=false for %s", tt.name)
			}
		})
	}
}

func TestParsePluginManifestPeriodsAllowed(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "acme.tools"
	}`)

	m, _, ok, err := parsePluginManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true for name with periods")
	}
	if m.Name != "acme.tools" {
		t.Errorf("name = %q", m.Name)
	}
}

func TestParsePluginManifestUnknownFieldNonFatal(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"engines": {}
	}`)

	_, unknown, ok, err := parsePluginManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true for unknown top-level field")
	}
	found := false
	for _, u := range unknown {
		if u == "engines" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'engines' in unknown fields, got %v", unknown)
	}
}

func TestParsePluginManifestExtensionsNonObject(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"extensions": "not-an-object"
	}`)

	_, unknown, ok, err := parsePluginManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true for non-object extensions")
	}
	found := false
	for _, u := range unknown {
		if u == "extensions (non-object, ignored per section 8.1)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected extensions warning in unknown, got %v", unknown)
	}
}

func TestParsePluginManifestAuthorUnknownField(t *testing.T) {
	dir := t.TempDir()
	writePluginJSON(t, dir, `{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"author": {"name": "Test", "twitter": "@test"}
	}`)

	_, _, ok, _ := parsePluginManifest(dir)
	if ok {
		t.Fatal("expected ok=false for author with unknown field")
	}
}

func TestHasPluginManifest(t *testing.T) {
	dir := t.TempDir()
	if hasPluginManifest(dir) {
		t.Fatal("expected false for empty dir")
	}
	writePluginJSON(t, dir, `{}`)
	if !hasPluginManifest(dir) {
		t.Fatal("expected true after writing plugin.json")
	}
}
