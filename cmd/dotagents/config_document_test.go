package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCanonicalTestConfig(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "dotagents.yaml")
	data := []byte("# canonical comment\nversion: 1\nfuture_key: preserve\nagents:\n  - name: Codex\n    enabled: false\n    skill_root: ~/.codex/skills\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigDocumentPreservesUnknownFieldsAndMode(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := json.Marshal(true)
	saved, err := doc.applyOperations(configLayerShared, doc.revision(configLayerShared), []configOperation{{Path: "/agents/codex/enabled", Op: "set", Value: value}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved.After), "future_key: preserve") || !strings.Contains(string(saved.After), "canonical comment") {
		t.Fatalf("node edit dropped unknown field or comment:\n%s", saved.After)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	if saved.Diff == "" || !strings.Contains(saved.Diff, "enabled: true") {
		t.Fatalf("diff does not show changed YAML:\n%s", saved.Diff)
	}
}

func TestConfigDocumentRejectsInvalidAndStaleWrites(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err := doc.saveRaw(configLayerShared, doc.revision(configLayerShared), []byte("version: [")); err == nil {
		t.Fatal("invalid YAML was accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid write changed canonical bytes")
	}
	if err := os.WriteFile(path, append(before, []byte("# external\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.saveRaw(configLayerShared, doc.revision(configLayerShared), before); !errors.Is(err, errStaleRevision) {
		t.Fatalf("stale write error = %v, want stale revision", err)
	}
}

func TestConfigDocumentLocalOverlayIsolatedAndUIWholeEntry(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	localPath := filepath.Join(root, "dotagents.local.yaml")
	local := []byte("ui:\n  links:\n    - name: Usage\n      url: /usage\n")
	if err := os.WriteFile(localPath, local, 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	if doc.effective.UI == nil || doc.effective.UI.Links[0].URL != "/usage" {
		t.Fatalf("effective UI overlay missing: %#v", doc.effective.UI)
	}
	value, _ := json.Marshal("/dashboard")
	if _, err := doc.applyOperations(configLayerLocal, doc.revision(configLayerLocal), []configOperation{{Path: "/ui/links/0/url", Op: "set", Value: value}}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("local edit changed shared YAML")
	}
}

func TestConfigWebRequiresSessionOriginAndCSRF(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	server := &configWebServer{doc: doc, origin: "http://127.0.0.1:8765", token: "session", csrf: "csrf"}
	handler := server.handler()
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/api/state", nil)
	request.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	request.Header.Set("Origin", server.origin)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "future_key") {
		t.Fatalf("authenticated state response = %d %s", response.Code, response.Body.String())
	}
	mutation := httptest.NewRequest(http.MethodPatch, "http://127.0.0.1:8765/api/config", strings.NewReader(`{"layer":"shared","expected_revision":"x","operations":[]}`))
	mutation.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	mutation.AddCookie(&http.Cookie{Name: "dotagents_csrf", Value: "csrf"})
	mutation.Header.Set("Origin", server.origin)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, mutation)
	if response.Code != http.StatusForbidden {
		t.Fatalf("CSRF-less mutation status = %d, want 403", response.Code)
	}
	value, _ := json.Marshal(true)
	body := `{"layer":"shared","expected_revision":"` + doc.revision(configLayerShared) + `","operations":[{"op":"set","path":"/agents/codex/enabled","value":` + string(value) + `}]}`
	mutation = httptest.NewRequest(http.MethodPatch, "http://127.0.0.1:8765/api/config", strings.NewReader(body))
	mutation.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	mutation.AddCookie(&http.Cookie{Name: "dotagents_csrf", Value: "csrf"})
	mutation.Header.Set("Origin", server.origin)
	mutation.Header.Set("X-Dotagents-CSRF", "csrf")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, mutation)
	if response.Code != http.StatusOK {
		t.Fatalf("authorized mutation status = %d: %s", response.Code, response.Body.String())
	}
	updated, _ := os.ReadFile(path)
	if !strings.Contains(string(updated), "enabled: true") {
		t.Fatalf("authorized structured mutation did not persist: %s", updated)
	}
}

func TestConfigWebAcceptsMatchingHTTPSProxyOrigin(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	server := &configWebServer{doc: doc, secureCookie: true, token: "session", csrf: "csrf"}
	handler := server.handler()

	request := httptest.NewRequest(http.MethodGet, "https://macbook.example.ts.net/api/state", nil)
	request.Host = "macbook.example.ts.net"
	request.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	request.Header.Set("Origin", "https://macbook.example.ts.net")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("matching proxy origin status = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "https://macbook.example.ts.net/api/state", nil)
	request.Host = "macbook.example.ts.net"
	request.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	request.Header.Set("Referer", "https://macbook.example.ts.net/dotagents/")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("same-origin referer status = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "https://macbook.example.ts.net/api/state", nil)
	request.Host = "macbook.example.ts.net"
	request.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("headerless same-origin GET status = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "https://macbook.example.ts.net/api/state", nil)
	request.Host = "macbook.example.ts.net"
	request.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	request.Header.Set("Origin", "https://other.example.ts.net")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("mismatched proxy origin status = %d, want 403", response.Code)
	}
}

func TestConfigWebRawYAMLPreservesSecrets(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("mcp_servers:\n  - name: secret\n    enabled: false\n    command: secret-tool\n    env:\n      TOKEN: secret-value\n    agents: [Codex]\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	server := &configWebServer{doc: doc, origin: "http://127.0.0.1:8765", token: "session", csrf: "csrf"}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/api/state", nil)
	request.AddCookie(&http.Cookie{Name: "dotagents_session", Value: "session"})
	request.Header.Set("Origin", server.origin)
	response := httptest.NewRecorder()
	server.handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "secret-value") {
		t.Fatalf("raw YAML lost secret: %d %s", response.Code, response.Body.String())
	}
}

func TestConfigValidationDoesNotExpandEditableLayers(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := writeCanonicalTestConfig(t, root)
	doc, err := newConfigDocument(path, home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.validateRaw(configLayerLocal, []byte("ui:\n  links:\n    - name: Usage\n      url: /usage\n")); err != nil {
		t.Fatal(err)
	}
	if got := doc.shared.Agents[0].SkillRoot; got != "~/.codex/skills" {
		t.Fatalf("shared skill root mutated to %q", got)
	}
	candidate, err := doc.validateRaw(configLayerShared, doc.sharedBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := candidate.Agents[0].SkillRoot; got != "~/.codex/skills" {
		t.Fatalf("shared candidate skill root expanded to %q", got)
	}
}

func TestConfigServeRejectsWildcardAddresses(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8765", ":8765", "192.0.2.1:8765", "[::]:8765"} {
		if err := validateLoopbackAddr(addr); err == nil {
			t.Fatalf("validateLoopbackAddr(%q) accepted non-loopback bind", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8765", "[::1]:8765", "localhost:8765"} {
		if err := validateLoopbackAddr(addr); err != nil {
			t.Fatalf("validateLoopbackAddr(%q) = %v", addr, err)
		}
	}
}
