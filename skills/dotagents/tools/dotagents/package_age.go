package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultPackageAgeMinimum = 7 * 24 * time.Hour

const (
	packageVersionLatest = "latest"
	packageCommandUVX    = "uvx"
)

var timeNow = time.Now

type packageReference struct {
	Ecosystem string
	Package   string
	Version   string
	Source    string
}

type packageRelease struct {
	Version   string
	Released  time.Time
	SourceURL string
}

type packageAgeResolver func(packageReference) (packageRelease, error)

func checkExternalPackageAge(repoRoot string, cfg config, skip bool, now time.Time) checkResult {
	return checkExternalPackageAgeWithResolver(repoRoot, cfg, skip, now, resolvePackageRelease)
}

func checkExternalPackageAgeWithResolver(repoRoot string, cfg config, skip bool, now time.Time, resolver packageAgeResolver) checkResult {
	if skip {
		return checkResult{"package age", checkStatusPass, "skipped by --skip-package-age"}
	}
	refs, err := collectPackageReferences(repoRoot, cfg)
	if err != nil {
		return checkResult{"package age", checkStatusFail, err.Error()}
	}
	if len(refs) == 0 {
		return checkResult{"package age", checkStatusPass, "no external package references found"}
	}
	var fresh []string
	var unresolved []string
	for _, ref := range refs {
		release, err := resolver(ref)
		if err != nil {
			unresolved = append(unresolved, fmt.Sprintf("%s:%s (%s)", ref.Ecosystem, ref.Package, err))
			continue
		}
		age := now.Sub(release.Released)
		if age < defaultPackageAgeMinimum {
			fresh = append(fresh, fmt.Sprintf("%s:%s@%s from %s is %s old", ref.Ecosystem, ref.Package, release.Version, ref.Source, age.Round(time.Hour)))
		}
	}
	if len(unresolved) > 0 {
		return checkResult{"package age", checkStatusFail, "registry lookup failed: " + strings.Join(unresolved, "; ")}
	}
	if len(fresh) > 0 {
		return checkResult{"package age", checkStatusFail, "package newer than 7 days: " + strings.Join(fresh, "; ")}
	}
	return checkResult{"package age", checkStatusPass, fmt.Sprintf("%d references older than 7 days", len(refs))}
}

func collectPackageReferences(repoRoot string, cfg config) ([]packageReference, error) {
	seen := map[string]packageReference{}
	add := func(ref packageReference) {
		ref.Package = strings.TrimSpace(ref.Package)
		ref.Version = strings.TrimSpace(ref.Version)
		if ref.Package == "" || strings.HasPrefix(ref.Package, "-") || strings.ContainsAny(ref.Package, "<>{}") {
			return
		}
		key := ref.Ecosystem + "\x00" + ref.Package + "\x00" + ref.Version + "\x00" + ref.Source
		seen[key] = ref
	}

	for _, server := range cfg.MCPServers {
		if !server.Enabled {
			continue
		}
		for _, ref := range packageReferencesFromCommand(server.Command, server.Args, "skills/dotagents/dotagents.yaml:"+server.Name) {
			add(ref)
		}
	}

	for _, path := range packageReferenceFiles(repoRoot) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			rel = path
		}
		for _, ref := range packageReferencesFromText(string(data), rel) {
			add(ref)
		}
	}

	refs := make([]packageReference, 0, len(seen))
	for _, ref := range seen {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Ecosystem != refs[j].Ecosystem {
			return refs[i].Ecosystem < refs[j].Ecosystem
		}
		return refs[i].Package < refs[j].Package
	})
	return refs, nil
}

func packageReferenceFiles(repoRoot string) []string {
	var paths []string
	addIfFile := func(path string) {
		if hasFile(path) {
			paths = append(paths, path)
		}
	}
	addIfFile(filepath.Join(repoRoot, "scripts", "setup-macbook.sh"))
	addIfFile(filepath.Join(repoRoot, "memory", "README.md"))
	skillsRoot := filepath.Join(repoRoot, "skills")
	_ = filepath.WalkDir(skillsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".md" {
			paths = append(paths, path)
		}
		return nil
	})
	return paths
}

func packageReferencesFromCommand(command string, args []string, source string) []packageReference {
	cmd := filepath.Base(strings.TrimSpace(command))
	switch cmd {
	case packageCommandUVX:
		if len(args) == 0 {
			return nil
		}
		pkg, version := splitPyPISpec(args[0])
		return []packageReference{{Ecosystem: "pypi", Package: pkg, Version: version, Source: source}}
	case "uv":
		if len(args) >= 3 && args[0] == "tool" && args[1] == "install" {
			pkg, version := splitPyPISpec(args[2])
			return []packageReference{{Ecosystem: "pypi", Package: pkg, Version: version, Source: source}}
		}
	case "npx", "pnpm", "npm":
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") || arg == "install" || arg == "add" || arg == "dlx" || arg == "-g" {
				continue
			}
			pkg, version := splitNPMSpec(arg)
			return []packageReference{{Ecosystem: "npm", Package: pkg, Version: version, Source: source}}
		}
	}
	return nil
}

var (
	uvToolInstallPattern = regexp.MustCompile(`\buv\s+tool\s+install\s+([A-Za-z0-9_.-]+(?:(?:==|@)[A-Za-z0-9_.!-]+)?)`)
	uvxPattern           = regexp.MustCompile(`\buvx\s+([A-Za-z0-9_.-]+(?:(?:==|@)[A-Za-z0-9_.!-]+)?)`)
	npmInstallPattern    = regexp.MustCompile(`\bnpm\s+install\s+(?:-g\s+)?((?:@[A-Za-z0-9_.-]+/)?[A-Za-z0-9_.-]+(?:@[A-Za-z0-9_.-]+)?)`)
	pnpmPattern          = regexp.MustCompile(`\bpnpm\s+(?:add|dlx)\s+((?:@[A-Za-z0-9_.-]+/)?[A-Za-z0-9_.-]+(?:@[A-Za-z0-9_.-]+)?)`)
)

func packageReferencesFromText(text string, source string) []packageReference {
	var refs []packageReference
	for _, match := range uvToolInstallPattern.FindAllStringSubmatch(text, -1) {
		pkg, version := splitPyPISpec(match[1])
		refs = append(refs, packageReference{Ecosystem: "pypi", Package: pkg, Version: version, Source: source})
	}
	for _, match := range uvxPattern.FindAllStringSubmatch(text, -1) {
		pkg, version := splitPyPISpec(match[1])
		refs = append(refs, packageReference{Ecosystem: "pypi", Package: pkg, Version: version, Source: source})
	}
	for _, match := range npmInstallPattern.FindAllStringSubmatch(text, -1) {
		pkg, version := splitNPMSpec(match[1])
		refs = append(refs, packageReference{Ecosystem: "npm", Package: pkg, Version: version, Source: source})
	}
	for _, match := range pnpmPattern.FindAllStringSubmatch(text, -1) {
		pkg, version := splitNPMSpec(match[1])
		refs = append(refs, packageReference{Ecosystem: "npm", Package: pkg, Version: version, Source: source})
	}
	return refs
}

func splitPyPISpec(spec string) (string, string) {
	spec = strings.TrimSpace(spec)
	if idx := strings.LastIndex(spec, "=="); idx > 0 {
		return spec[:idx], spec[idx+2:]
	}
	if idx := strings.LastIndex(spec, "@"); idx > 0 {
		return spec[:idx], spec[idx+1:]
	}
	return spec, packageVersionLatest
}

func splitNPMSpec(spec string) (string, string) {
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "@") {
		slash := strings.Index(spec, "/")
		if slash == -1 {
			return spec, packageVersionLatest
		}
		if idx := strings.LastIndex(spec[slash+1:], "@"); idx >= 0 {
			cut := slash + 1 + idx
			return spec[:cut], spec[cut+1:]
		}
		return spec, packageVersionLatest
	}
	if idx := strings.LastIndex(spec, "@"); idx > 0 {
		return spec[:idx], spec[idx+1:]
	}
	return spec, packageVersionLatest
}

func resolvePackageRelease(ref packageReference) (packageRelease, error) {
	switch ref.Ecosystem {
	case "pypi":
		return resolvePyPIRelease(ref)
	case "npm":
		return resolveNPMRelease(ref)
	default:
		return packageRelease{}, fmt.Errorf("unsupported ecosystem %q", ref.Ecosystem)
	}
}

func resolvePyPIRelease(ref packageReference) (packageRelease, error) {
	endpoint := "https://pypi.org/pypi/" + url.PathEscape(ref.Package) + "/json"
	var doc struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
		Releases map[string][]struct {
			UploadTime string `json:"upload_time_iso_8601"`
		} `json:"releases"`
	}
	if err := getJSON(endpoint, &doc); err != nil {
		return packageRelease{}, err
	}
	version := ref.Version
	if version == "" || version == packageVersionLatest {
		version = doc.Info.Version
	}
	files := doc.Releases[version]
	if len(files) == 0 {
		return packageRelease{}, fmt.Errorf("version %q not found", version)
	}
	released, err := time.Parse(time.RFC3339, strings.TrimSuffix(files[0].UploadTime, "Z")+"Z")
	if err != nil {
		return packageRelease{}, fmt.Errorf("parse upload time: %w", err)
	}
	return packageRelease{Version: version, Released: released, SourceURL: endpoint}, nil
}

func resolveNPMRelease(ref packageReference) (packageRelease, error) {
	endpoint := "https://registry.npmjs.org/" + url.PathEscape(ref.Package)
	endpoint = strings.ReplaceAll(endpoint, "%2F", "/")
	var doc struct {
		DistTags map[string]string `json:"dist-tags"`
		Time     map[string]string `json:"time"`
	}
	if err := getJSON(endpoint, &doc); err != nil {
		return packageRelease{}, err
	}
	version := ref.Version
	if version == "" || version == packageVersionLatest {
		version = doc.DistTags[packageVersionLatest]
	}
	raw := doc.Time[version]
	if raw == "" {
		return packageRelease{}, fmt.Errorf("version %q not found", version)
	}
	released, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return packageRelease{}, fmt.Errorf("parse release time: %w", err)
	}
	return packageRelease{Version: version, Released: released, SourceURL: endpoint}, nil
}

func getJSON(endpoint string, target interface{}) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
