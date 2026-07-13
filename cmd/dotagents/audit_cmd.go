package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Severity levels for `dotagents audit`. CRITICAL is the only level that fails
// the command (non-zero exit).
const (
	auditCritical = "CRITICAL"
	auditWarn     = "WARN"
	auditInfo     = "INFO"
)

// auditFinding is a single reported issue. path is relative to the skill
// directory (or the lock file name for external-skill findings).
type auditFinding struct {
	severity string
	skill    string
	path     string
	desc     string
}

// Heuristics for the standalone audit. These are intentionally string/regex
// based (PoC): precision over recall, and no CRITICAL fires without a strong
// combined signal.
var (
	// Scripts that read a credential path (broader than reCredentialPaths so it
	// also covers ~/.aws/credentials, id_rsa, and .env files).
	reScriptCredential = regexp.MustCompile(`(?i)(\$HOME|~)/\.(ssh|aws|gnupg)\b|/\.aws/credentials\b|\bid_rsa\b|\.netrc\b|(?:^|[\s'"=(])\.env(\.[a-z0-9]+)?\b`)
	// Any outbound network call.
	reNetworkCall = regexp.MustCompile(`(?i)\b(curl|wget|nc|ncat|telnet)\b|https?://|\brequests\.(get|post|put|patch|delete)\b|\burllib\b|\bhttp\.client\b|\bnet/http\b|\bfetch\(|\baxios\b|\bsocket\.(socket|create_connection)\b|\bXMLHttpRequest\b`)
	// SKILL.md instruction verbs that move data outward.
	reExfilVerb = regexp.MustCompile(`(?i)\b(exfiltrat\w*|upload|send|transmit|forward|leak|beacon|post(\s+the)?)\b`)
	// Sensitive-data nouns that make an exfil instruction critical.
	reSensitiveData = regexp.MustCompile(`(?i)\b(secret|secrets|credential|credentials|token|tokens|password|passwords|api[ _-]?keys?|private key|ssh key|\.env|env var|environment variable|\.aws|\.ssh|history file)\b`)
	// Any http(s) URL.
	reAuditURL = regexp.MustCompile(`https?://[^\s)"'<>\]]+`)
	// HTML comments (hidden-instruction vector).
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	// Imperative verbs that make a hidden HTML comment suspicious.
	reImperative = regexp.MustCompile(`(?i)\b(ignore|disregard|send|upload|post|exfiltrat\w*|run|execute|delete|fetch|download|curl|wget|export|read|cat|leak|transmit|do not|always|never)\b`)
)

// auditDocHosts are hosts treated as documentation/reference links, not
// findings. Entries ending in "." match as a prefix (e.g. "docs.").
var auditDocHosts = []string{
	"github.com", "raw.githubusercontent.com", "gist.github.com",
	"docs.", "developer.", "readthedocs.io", "readthedocs.org",
	"pkg.go.dev", "go.dev", "golang.org", "stackoverflow.com",
	"wikipedia.org", "npmjs.com", "pypi.org", "crates.io",
	"anthropic.com", "openai.com", "mozilla.org", "developer.mozilla.org",
	"w3.org", "apache.org", "gnu.org", "json.org", "rfc-editor.org",
	"ietf.org", "example.com", "example.org", "localhost", "127.0.0.1",
}

var auditScriptExtensions = map[string]bool{
	".sh": true, ".bash": true, ".zsh": true,
	".py": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true,
	".rb": true, ".pl": true,
}

func runAudit(opts runOptions) error {
	repoRoot, _, cfg, _, err := loadContext(opts)
	if err != nil {
		return err
	}

	fmt.Println("dotagents audit")
	fmt.Printf("repo: %s\n\n", repoRoot)

	findings := auditRepo(repoRoot, cfg)
	crit, _, _ := printAuditFindings(os.Stdout, findings)
	if crit > 0 {
		return fmt.Errorf("%d critical finding(s)", crit)
	}
	return nil
}

// auditRepo scans local skills and external-skill pinning, returning sorted
// findings. It reads only the filesystem so it is directly testable.
func auditRepo(repoRoot string, cfg config) []auditFinding {
	var findings []auditFinding

	skillsDir := filepath.Join(repoRoot, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			dir := filepath.Join(skillsDir, e.Name())
			if !hasFile(filepath.Join(dir, "SKILL.md")) {
				continue
			}
			findings = append(findings, auditLocalSkill(e.Name(), dir)...)
		}
	}

	lock, _ := readLockFile(repoRoot)
	for _, src := range cfg.ExternalSkills {
		if lockEntryFor(lock, src) == nil {
			findings = append(findings, auditFinding{
				auditWarn, repoName(src.URL), lockFileName,
				"external skill not pinned in " + lockFileName,
			})
		}
	}

	sortAuditFindings(findings)
	return findings
}

// auditLocalSkill scans one skill directory tree.
func auditLocalSkill(name, dir string) []auditFinding {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})

	hasBuildSource := false
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".go" || ext == ".rs" || ext == ".c" || ext == ".cpp" ||
			base == "makefile" || base == "build.sh" || base == "go.mod" {
			hasBuildSource = true
			break
		}
	}

	var findings []auditFinding
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			rel = filepath.Base(f)
		}
		switch {
		case filepath.Base(f) == "SKILL.md":
			findings = append(findings, auditSkillMarkdown(name, rel, f)...)
		case auditScriptExtensions[strings.ToLower(filepath.Ext(f))]:
			findings = append(findings, auditScriptFile(name, rel, f)...)
		default:
			if fb := auditBinaryFile(name, rel, f, hasBuildSource); fb != nil {
				findings = append(findings, *fb)
			}
		}
	}
	return findings
}

// auditSkillMarkdown inspects SKILL.md instruction text (frontmatter stripped).
func auditSkillMarkdown(skill, rel, path string) []auditFinding {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	body := stripFrontmatter(string(data))
	var findings []auditFinding

	for _, comment := range reHTMLComment.FindAllString(body, -1) {
		if reImperative.MatchString(comment) {
			findings = append(findings, auditFinding{
				auditWarn, skill, rel,
				"HTML comment in SKILL.md contains imperative instructions",
			})
			break
		}
	}

	seenHost := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		urls := reAuditURL.FindAllString(line, -1)
		if len(urls) > 0 && reExfilVerb.MatchString(line) && reSensitiveData.MatchString(line) {
			findings = append(findings, auditFinding{
				auditCritical, skill, rel,
				"SKILL.md instructs exfiltrating sensitive data to an external URL",
			})
			continue
		}
		for _, u := range urls {
			host := urlHost(u)
			if host == "" || isDocHost(host) || seenHost[host] {
				continue
			}
			seenHost[host] = true
			findings = append(findings, auditFinding{
				auditWarn, skill, rel,
				"SKILL.md links to non-documentation host " + host,
			})
		}
	}
	return findings
}

// auditScriptFile inspects a bundled script for shell/exfil patterns.
func auditScriptFile(skill, rel, path string) []auditFinding {
	info, err := os.Stat(path)
	if err != nil || info.Size() > skillAuditMaxFileSize {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(data)
	var findings []auditFinding

	if rePipeToShell.MatchString(text) {
		findings = append(findings, auditFinding{
			auditCritical, skill, rel, "curl/wget output piped directly into a shell",
		})
	}
	if reBase64ToShell.MatchString(text) {
		findings = append(findings, auditFinding{
			auditCritical, skill, rel, "base64-decoded payload piped into a shell",
		})
	}

	cred := reScriptCredential.MatchString(text)
	net := reNetworkCall.MatchString(text)
	switch {
	case cred && net:
		findings = append(findings, auditFinding{
			auditCritical, skill, rel,
			"reads a credential path and makes a network call in the same script",
		})
	case net:
		findings = append(findings, auditFinding{
			auditInfo, skill, rel, "script makes a network call",
		})
	}
	return findings
}

// auditBinaryFile flags committed executable binaries that lack accompanying
// build scripts or source in the skill tree.
func auditBinaryFile(skill, rel, path string, hasBuildSource bool) *auditFinding {
	if hasBuildSource || !isExecutableBinary(path) {
		return nil
	}
	return &auditFinding{
		auditWarn, skill, rel,
		"bundled executable binary without build script or source",
	}
}

func isExecutableBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	buf = buf[:n]
	if hasExecutableMagic(buf) {
		return true
	}
	execBit := info.Mode()&0o111 != 0
	return execBit && bytes.IndexByte(buf, 0) >= 0
}

func hasExecutableMagic(b []byte) bool {
	if len(b) >= 4 {
		if b[0] == 0x7f && b[1] == 'E' && b[2] == 'L' && b[3] == 'F' {
			return true // ELF
		}
		switch binary.BigEndian.Uint32(b[:4]) {
		case 0xFEEDFACE, 0xFEEDFACF, 0xCEFAEDFE, 0xCFFAEDFE, 0xCAFEBABE, 0xBEBAFECA:
			return true // Mach-O (thin + fat, both byte orders)
		}
	}
	if len(b) >= 2 && b[0] == 'M' && b[1] == 'Z' {
		return true // PE/COFF
	}
	return false
}

func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return content
	}
	nl := strings.IndexByte(content, '\n')
	rest := content[nl+1:]
	loc := regexp.MustCompile(`(?m)^---\s*$`).FindStringIndex(rest)
	if loc == nil {
		return content
	}
	return rest[loc[1]:]
}

func urlHost(raw string) string {
	raw = strings.TrimRight(raw, ".,);]\"'>")
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func isDocHost(host string) bool {
	for _, d := range auditDocHosts {
		if strings.HasSuffix(d, ".") {
			if strings.HasPrefix(host, d) {
				return true
			}
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func auditSeverityRank(sev string) int {
	switch sev {
	case auditCritical:
		return 0
	case auditWarn:
		return 1
	default:
		return 2
	}
}

func sortAuditFindings(findings []auditFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if ra, rb := auditSeverityRank(a.severity), auditSeverityRank(b.severity); ra != rb {
			return ra < rb
		}
		if a.skill != b.skill {
			return a.skill < b.skill
		}
		if a.path != b.path {
			return a.path < b.path
		}
		return a.desc < b.desc
	})
}

func printAuditFindings(w io.Writer, findings []auditFinding) (crit, warn, info int) {
	for _, f := range findings {
		fmt.Fprintf(w, "%s %s %s: %s\n", f.severity, f.skill, f.path, f.desc)
		switch f.severity {
		case auditCritical:
			crit++
		case auditWarn:
			warn++
		case auditInfo:
			info++
		}
	}
	fmt.Fprintf(w, "\n%d finding(s): %d critical, %d warn, %d info\n", len(findings), crit, warn, info)
	return crit, warn, info
}
