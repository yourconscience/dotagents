# Agent Plugins Spec (v1.0.0) support in dotagents

Status: v1 implemented (skill + MCP extraction). Native plugin projection (Codex) planned.

Sources read: `/Users/conscience/Public/agent-plugins-spec/spec/1.0.0.md`,
`schemas/1.0.0/plugin.schema.json`, `schemas/1.0.0/mcp.schema.json`; dotagents
`cmd/dotagents/{external.go,config.go,main.go,lock.go,skillspec.go,mcp.go,
harness.go,setup_scaffold.go,sync.go,skill_discovery.go}` and
`starter_assets.go`.

## 1. Scope

**In:**

1. When a `dotagents.yaml` `external_skills` source's cloned repo contains a
   `plugin.json` at its root, parse and validate it for richer metadata
   (name, version, author, license) and to unlock optional `mcp.json`
   discovery.
2. When that source also opts in (`mcp: true`) and has a valid `mcp.json`,
   discover its `stdio` MCP servers and feed them into the existing MCP
   surface (`cfg.MCPServers` → harness sync), the same surface that already
   consumes `dotagents.yaml`'s `mcp_servers:` list.
3. Ship a skills-only `plugin.json` at the dotagents config root so any
   Agent Plugins v1 client can consume `~/.agents/skills/*` directly,
   independent of dotagents.

**Out of v1 (planned for follow-up):**

- **Codex native plugin projection**: translating agent-plugins-spec
  `plugin.json` into Codex's `.codex-plugin/plugin.json` format and
  registering via `codex plugin marketplace add`. Codex uses a different,
  richer manifest schema (`interface`, `apps`, relative-path `skills`/
  `mcpServers` pointers). This requires a format translation layer.
- **Claude Code plugins** (`.claude/plugins/*.ts`): different format,
  TypeScript-based. Tracked separately.
- Building, publishing, or installing plugin *packages* for any harness.
  Nothing here produces a `.zip`/registry artifact.
- A plugin registry, marketplace, or `dotagents plugin install <name>` verb.
  External sources remain Git repos declared in `dotagents.yaml`, exactly as
  today; a `plugin.json` inside one is metadata dotagents reads, not
  something dotagents produces for delivery.
- `com.<vendor>.<client>/` client extension directories and the `extensions`
  field's namespaced payloads (§8). dotagents has no reverse-domain
  namespace of its own to register and doesn't act as a plugin *runtime* —
  see §11.3 (Framing) below.
- `streamable-http` / `sse` MCP servers. dotagents' `mcpServerConfig`
  (`cmd/dotagents/mcp.go:15`) is `Command/Args/Env` only; every harness
  writer (`patchJSONMCPServer`, `patchYAMLMCPServer`, `patchCodexMCPServer`,
  Claude's project-map writer) assumes a launchable stdio process. Remote
  transports are detected, validated per §7.2.1, and skipped with
  §7.2.2(4)'s "unsupported transport" report — this keeps dotagents
  spec-legal without a schema-wide MCP rework. This is fine: §11.1(5) only
  requires *at least one* of `stdio`/`streamable-http`, and dotagents
  supports `stdio`.
- Launching plugin subprocesses. dotagents never execs an MCP server — it
  writes harness-native config, and the *harness* execs the process later.
  This matters for `PLUGIN_ROOT`/`PLUGIN_DATA`/`cwd` handling; see §4.

### Framing: consumer, not conformant client

The spec's failure model (§5.2, §5.3) makes most `plugin.json` violations
**fatal to the whole plugin** — a typo disables its skills too. dotagents
does not adopt that. Per `AGENTS.md`, this is "external source format
detection," not a plugin runtime. Concretely:

- An invalid `plugin.json` disables *plugin-mode features only* (metadata
  capture, `mcp.json` discovery) for that source. Existing `skill_dir`/
  `skill_dirs` discovery is completely unaffected — a source with a broken
  manifest but a working `skills/summarize/SKILL.md` still syncs that skill,
  exactly as it would today with no `plugin.json` at all.
- dotagents never launches a subprocess, so it does not implement §9
  (`PLUGIN_ROOT`/`PLUGIN_DATA` injection into a live process) — it
  approximates the *configuration-time* half of §9 (placeholder expansion,
  reserved env names, absolute-path resolution) so the servers it hands to
  harnesses behave equivalently once *they* launch it.
- dotagents does not implement §8 (client extensions) at all: no namespace,
  no extension-directory scanning. Unknown `extensions` content is ignored
  per §8.1's own non-fatal rule, which requires no code beyond "don't error
  on it."

State this deviation in the README/SKILL.md docs update (§9 below) so it's
not a silent gap.

## 2. Detection

`externalSkillSource.SkillDir`/`SkillDirs` already default to `["skills"]`
when unset (`config.go:156-158`), which is exactly the Agent Plugins §6.1
fixed skill location. So plugin detection does **not** change *where*
dotagents looks for skills in the default case — it only changes (a) what
metadata is available and (b) what happens when that default directory is
missing or empty.

Detection happens once per source, at the cache root (`~/.agents/external/
<repoName>/`), independent of `SkillDir`/`SkillDirs`:

```go
// external.go
func hasPluginManifest(cachePath string) bool {
    return hasFile(filepath.Join(cachePath, "plugin.json"))
}
```

A source is in "plugin mode" for discovery-relaxation purposes only when
**both** hold: `hasPluginManifest(cachePath)` and the source uses the
*default* skill location (`src.SkillDir == "skills" && len(src.SkillDirs)
== 0`). Note `validateConfig` (`config.go:156-158`) already normalizes an
unset `skill_dir` to `"skills"`, so this predicate cannot distinguish "user
wrote nothing" from "user explicitly wrote `skill_dir: skills`" — that's
fine, the two are equivalent in effect (both mean "use the default
location") and the predicate only needs to detect *non*-default overrides
(`skill_dirs: [...]` or a `skill_dir` other than `"skills"`), which it does
correctly. If the user set either to something else, that's an explicit
override and current strict behavior (error if missing/empty) is preserved
unchanged — plugin mode never weakens an explicit non-default request.

`plugin.json` and `mcp.json` themselves get the same symlink-escape check
`discoverExternalSourceSkills` already applies to skill directories
(`external.go:334-340`): resolve via `filepath.EvalSymlinks` and require the
result stay within the resolved cache root, else treat as absent (§4.1
containment; a manifest that escapes the plugin root is exactly the case
§4.1 rule 1 requires rejecting).

## 3. Manifest parsing (`pluginspec.go`, new file)

Mirrors the style of `skillspec.go` (`unknownSkillSpecFields`,
`validateSkillSpecFields`) rather than introducing a generic JSON-schema
validator.

```go
const pluginManifestSchemaID = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"

// Same character-set rule as schemas/1.0.0/plugin.schema.json's "pattern",
// which additionally allows periods versus skillSpecNamePattern.
var pluginNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

var pluginManifestKnownFields = map[string]bool{
    "$schema": true, "name": true, "version": true, "description": true,
    "author": true, "homepage": true, "repository": true, "license": true,
    "keywords": true, "extensions": true,
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

// parsePluginManifest reads plugin.json at pluginRoot.
//   ok=false: manifest is fatally invalid (§5.3, or any §5.2 violation other
//     than an unknown top-level field / non-object extensions). Caller must
//     disable plugin-mode features for this source but MUST NOT touch skill
//     discovery (see §1 Framing).
//   unknown: non-fatal problems to report (unknown top-level fields present
//     even when ok=true; a non-object "extensions" field).
func parsePluginManifest(pluginRoot string) (manifest pluginManifest, unknown []string, ok bool, err error)
```

Implementation notes:

- Decode twice, like `parseSkillSpecFrontmatter` does for YAML: once into
  `map[string]json.RawMessage` to diff against `pluginManifestKnownFields`
  (unknown top-level keys → non-fatal, reported, dropped — §5.2), once with
  a `json.Decoder` that has `DisallowUnknownFields()` for the nested
  `author` object only (unknown keys *inside* `author` ARE fatal per §5.4:
  "Any other field or value type makes the manifest invalid").
- `$schema` must equal `pluginManifestSchemaID` exactly. dotagents
  recognizes no compatible-version mapping in v1 of this feature (§5.2:
  "MAY map multiple... only when it explicitly recognizes... compatible" —
  we don't, yet). Any other value (including a `mcp.schema.json` URL, an
  older spec version, or garbage) is fatal, matching §5.3.
- `name`: required, 1–64 chars, `pluginNamePattern`, no `--`/`..`
  (`strings.Contains(name, "--") || strings.Contains(name, "..")`), first/
  last char alphanumeric (implied by the pattern above already since it's
  anchored `[a-z0-9]...[a-z0-9]`).
- `extensions`: if present and not a JSON object, that's the one other
  non-fatal case (§8.1) — report and ignore, keep loading. dotagents stores
  nothing from `extensions`; it never inspects namespace contents (§1
  Framing).
- Any other decode/type error (bad `version` type, `keywords` not an array
  of strings, `author` malformed, `$schema` wrong) → `ok=false`. Caller
  prints one `warning: external <name> plugin.json invalid (<detail>);
  plugin metadata and mcp.json ignored, skill discovery unaffected` and
  proceeds with legacy discovery.

## 4. MCP discovery from `mcp.json` (`pluginmcp.go`, new file)

Only reached when: `parsePluginManifest` returned `ok=true` **and**
`src.MCP == true` (new opt-in config field, default `false` — see §7).
`mcp.json` absent is not an error (§6.2); anything below downgrades to "MCP
disabled for this source" with a report, never a hard sync failure — see
"Name collisions" below for the one exception, and "Not-yet-cloned sources"
for the pre-first-sync case.

**Not-yet-cloned sources.** Because injection runs inside `loadContext`
(§6), it fires on *every* command, including a fresh `dotagents setup`
before `syncExternalRepos` has ever cloned anything
(`~/.agents/external/<name>/` doesn't exist yet). This is the normal
pre-first-sync state, not an error: `discoverPluginMCPServers`'s caller
must check `hasDir(filepath.Join(cachePath, ".git"))` first and return zero
servers, zero problems — silently, no warning — when the cache isn't there
yet. Contrast with `discoverExternalSourceSkills`, which *does* hard-error
on a missing clone (`external.go:313`, "run dotagents sync"); MCP discovery
must not inherit that, since unlike skill discovery it's not the thing the
user is actively waiting on during setup.

```go
const pluginMCPSchemaID = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"

type pluginMCPServerRaw struct {
    Type    string            `json:"type"`
    Command string            `json:"command"`
    Args    []string          `json:"args"`
    Env     map[string]string `json:"env"`
    Cwd     string            `json:"cwd"`
    URL     string            `json:"url"`
    Headers map[string]string `json:"headers"`
}

// discoverPluginMCPServers loads mcp.json at pluginRoot (already known to
// have a valid plugin.json at the same root with Schema == manifestSchema),
// and returns the servers dotagents can represent, plus problems for
// servers/entries it skips per §7.2.2. pluginDataDir must already exist
// (see "PLUGIN_DATA lifecycle" below) and is used only for placeholder
// expansion, never created here.
func discoverPluginMCPServers(pluginRoot string, manifestSchema string, pluginDataDir string) (servers []mcpServerConfig, problems []string, err error)
```

Rules, in order, each producing a `problems` entry and a skip (not a hard
error) unless stated otherwise:

1. `mcp.json` missing → not an error, zero servers (§6.2).
2. `mcp.json` present but not a JSON object, or `$schema`/`mcpServers`
   missing/wrong-typed, or `$schema != pluginMCPSchemaID`, or `$schema`'s
   declared spec version differs from `plugin.json`'s → **disable MCP for
   the whole source**, zero servers, one problem reported (§7.2.2 rule 2,
   §10.1 "mismatch makes the MCP configuration invalid").
3. Per server entry, `type` decides the variant (§7.2.1's closed union):
   - `"streamable-http"` or `"sse"`: valid per schema but unsupported
     transport in dotagents (§1 Scope) → skip + report "unsupported
     transport %q for server %q" (§7.2.2 rule 4). Sibling servers still
     load.
   - `"stdio"`: proceed to steps 4–8.
   - anything else / missing `type` / fields from another variant present:
     invalid entry → skip + report (§7.2.2 rule 3).
4. **`command`** must be non-empty. Resolve it:
   - Bare token (no `/`, doesn't start with `./`): pass through unchanged —
     dotagents relies on the harness's own PATH resolution, matching
     "Whether a configured PATH... is client-defined" (§7.2.1).
   - `./`-prefixed: resolve absolute against `pluginRoot`
     (`filepath.Join(pluginRoot, strings.TrimPrefix(command, "./"))`), then
     require the resolved (symlink-evaluated) path stay within the
     resolved `pluginRoot` — same containment check as skill dirs. Escape →
     invalid entry, skip + report.
   - Anything else (absolute path, `../`, contains whitespace as a shell
     string) → invalid entry, skip + report. dotagents does not attempt to
     tokenize a shell command string (§7.2.1: "single executable token, not
     a shell command string").
   - **No placeholder expansion in `command`** (§9.2 explicitly excludes
     it) — do not call `expandPluginPlaceholders` here.
5. **`cwd`**: dotagents' `mcpServerConfig` has no `Cwd` field and no harness
   writer sets a working directory, so dotagents cannot honor either the
   spec's "default to plugin root" rule or an explicit `cwd`. **If `cwd` is
   present at all (any value), skip the entry + report** "cwd unsupported;
   server %q skipped" rather than silently dropping it or silently using a
   wrong default. This is a named limitation (see Risks), not a silent gap.
6. **`args`**: each element passed through `expandPluginPlaceholders`
   (below).
7. **`env`**: reject the entry if the *raw, pre-expansion* keys contain
   `PLUGIN_ROOT` or `PLUGIN_DATA` (§9.2: "MUST NOT contain entries named...
   invalid under §7.2.2") — skip + report. Otherwise expand each value via
   `expandPluginPlaceholders`, then **dotagents itself sets**
   `env["PLUGIN_ROOT"] = pluginRoot` and `env["PLUGIN_DATA"] =
   pluginDataDir` (absolute paths) on the projected server. This is the
   configuration-time equivalent of §9.1's "client MUST then set
   PLUGIN_ROOT/PLUGIN_DATA... replacing any entries with equivalent names"
   — dotagents does it once at projection time instead of per-launch,
   because dotagents isn't the process that launches the subprocess (§1
   Framing).
8. **Server name**: validate against a conservative pattern narrower than
   the plugin schema allows, because dotagents' Codex writer
   (`renderCodexMCPSection`, `mcp.go:601`) emits `[mcp_servers.<name>]` TOML
   headers — a `.` in the name would create a nested table
   (`endTOMLSectionIncludingDescendants` already has special-casing for
   `.`-separated descendant sections, so a dotted MCP name would silently
   nest under an unrelated section). Use
   `^[a-z0-9][a-z0-9_-]*$` (no dots, no uppercase). Names failing this
   → skip + report. This is stricter than `mcp.schema.json` (which puts no
   `propertyNames` constraint on `mcpServers`), which is allowed: dotagents
   is free to reject entries it can't represent safely (§7.2.2 rule 3
   covers "does not satisfy requirements" generally; we're adding a
   dotagents-specific requirement, not misreading the spec's).

### Placeholder expansion — single pass (§9.2)

```go
// expandPluginPlaceholders performs one left-to-right scan replacing every
// exact "${PLUGIN_ROOT}" and "${PLUGIN_DATA}" occurrence. Text introduced
// by a replacement is never re-scanned (§9.2: "MUST NOT be scanned for
// further placeholders"). Unrecognized ${...} text is left literal.
func expandPluginPlaceholders(s string, pluginRoot string, pluginData string) string
```

Implementation: a single `strings.Builder` walk over `s`, checking at each
`${` for a literal match against `"${PLUGIN_ROOT}"` / `"${PLUGIN_DATA}"`,
appending the substituted value and advancing the cursor past the *matched
placeholder in the source*, not past the replacement — this is what makes
it non-recursive. Do **not** implement via two chained
`strings.ReplaceAll` calls (would violate §9.2 if one path's value
contains the other literal token, e.g. `pluginData` itself containing the
substring `${PLUGIN_ROOT}` because a user misconfigured a data root — edge
case, but the single-scan implementation is no harder and closes it).

### `PLUGIN_DATA` lifecycle

§9.1 requires `PLUGIN_DATA` to "preserve its contents across plugin
updates." dotagents' `refreshExternalCache` does `git reset --hard` /
`gitFetchReset` on the *cache* directory (`external.go:71-89`) — anything
placed inside `~/.agents/external/<name>/` will be wiped on update. So
`PLUGIN_DATA` must be a **sibling** directory, never inside the clone:

```go
// external.go
func externalDataDir(home string) string {
    return filepath.Join(home, ".agents", "external-data")
}
```

`filepath.Join(externalDataDir(home), repoName(src.URL))`, created with
`os.MkdirAll(..., 0o755)` by `discoverPluginMCPServers`'s caller
(`injectPluginMCPServers`, §6) before use, once per sync/status/doctor
invocation that needs it — idempotent, cheap. Note this also means
`PLUGIN_ROOT` is **always the cache path**, even when the source has
`materialize: true` — materialization copies only the matched skill
directories into `skills/`, not `bin/`, `config.json`, or other files an
MCP server's `./`-relative `command`/`args` might reference. Document this:
materialized sources still resolve `PLUGIN_ROOT` against the *external
cache*, not the materialized copy.

No pruning of orphaned `external-data/<name>/` directories in v1 (source
removed from config, or renamed via URL change → new `repoName`). Flagged
as a follow-up in Risks, not blocking.

### Name collisions

- A plugin-derived server name equal to a name already in `cfg.MCPServers`
  (i.e., something the user wrote directly in `dotagents.yaml`): **user
  config wins**, plugin entry skipped + reported. Never silently overwrite
  a user's own declaration.
- Two different external sources both project a server with the same name:
  **both entries skipped + reported**, not a hard error. Unlike the
  "duplicate materialized skill basename" precedent
  (`external.go:350,483`), this check lives inside `loadContext` (§6),
  which every command calls — a hard error here would abort `dotagents
  status`, `doctor`, and `external list` too, including the exact commands
  a user would reach for to diagnose the collision. Skip-and-report keeps
  those commands usable while still surfacing the problem loudly.

## 5. Identity

`repoName(url)` (`external.go:19`) stays the sole identity key for cache
path, lock entry, `--agents`-style `dotagents external update <name>`
selection, and `dotagents.local.yaml` overlay matching
(`mergeConfig`/`mergeByKey`, `config.go:87`). Reasons to *not* switch to
`plugin.json`'s `name`:

- Plugin `name` allows periods (`a-z0-9-.`); dotagents' own skill-name
  regex (`skillSpecNamePattern`, `skillspec.go:26`) and every identity path
  in `external.go`/`lock.go` assume the repo-basename shape. Reusing it as
  a directory/lock key would need a second sanitization pass for no
  benefit.
- Plugin `name` is author-controlled and can collide across unrelated
  repos (two plugins both named `tools`), whereas `repoName(url)` is
  already how dotagents disambiguates today.
- The source may have no `plugin.json` at all; identity must work
  identically whether or not one is present.

`plugin.json`'s `name`/`version` become **display metadata only** — stored
in the lock entry (§6) and shown by `dotagents external list` (§7), never
used as a key anywhere.

## 6. Lock file changes

```go
// lock.go
type externalLockEntry struct {
    Name          string                 `yaml:"name"`
    URL           string                 `yaml:"url"`
    Branch        string                 `yaml:"branch"`
    Commit        string                 `yaml:"commit"`
    Materialized  materializedSkillNames `yaml:"materialized,omitempty"`
    PluginName    string                 `yaml:"plugin_name,omitempty"`
    PluginVersion string                 `yaml:"plugin_version,omitempty"`
}
```

Both new fields are plain `string`, preserving `externalLockEntry`'s
`==`/`!=` comparability that `lockEntriesEqual` (`lock.go:154`) depends on
— no repeat of the `materializedSkillNames` custom-comparable-string trick
is needed since these are scalars.

`rebuildLockEntries` (`lock.go:114`) gains, right after computing `commit`:

```go
if manifest, _, ok, err := parsePluginManifest(filepath.Join(cacheRoot, name)); err == nil && ok {
    pluginName, pluginVersion = manifest.Name, manifest.Version
}
```

Best-effort: a parse error or invalid manifest just leaves both fields
empty (matches the "invalid manifest degrades gracefully" framing — the
lock file shouldn't fail to write because of a third party's typo).
Nothing else about the lock file format changes; `dotagents.lock`'s header
comment gets one clause added ("...and Agent Plugins metadata when
present").

## 7. Config changes

```go
// main.go
type externalSkillSource struct {
    URL         string   `yaml:"url"`
    SkillDir    string   `yaml:"skill_dir,omitempty"`
    SkillDirs   []string `yaml:"skill_dirs,omitempty"`
    Branch      string   `yaml:"branch"`
    Skills      []string `yaml:"skills,omitempty"`
    Materialize bool     `yaml:"materialize,omitempty"`
    MCP         bool     `yaml:"mcp,omitempty"`         // opt-in: project this source's mcp.json into cfg.MCPServers
    MCPAgents   []string `yaml:"mcp_agents,omitempty"`  // optional target subset; empty = all MCP-capable harnesses (mirrors mcpServerConfig.Agents semantics in desiredMCPServersForAgent)
}
```

`MCP` defaults to `false`. This is a deliberate opt-in, not "MCP servers
load automatically whenever a source happens to ship an `mcp.json`": a
source added purely for its skills should never silently register a
third-party executable into every harness. The codebase already treats
*imported* native MCP config as needing scrutiny
(`importedMCPInvocationHasLiteralSecret`, `isSensitiveMCPArgumentName` in
`setup_scaffold.go`) — opt-in here is consistent with that posture, applied
one layer earlier (config-time, not just import-time).

`validateConfig` (`config.go:117`) gains, alongside the existing
`external_skills` loop, a normalization pass for `MCPAgents` mirroring the
`cfg.MCPServers[i].Agents` loop at `config.go:201-222` (normalize each name,
drop targets without `hasMCPSupport`, warn on the legacy `pi` case) — same
code shape, applied to `src.MCPAgents` instead of `server.Agents`. No
change needed to `SkillDir`/`SkillDirs` defaulting; it already matches the
spec's fixed `skills/` location (§2).

`dotagents.local.yaml` overlay (`mergeConfig`) needs no change: it already
replaces `externalSkillSource` entries wholesale by `repoName(url)`
(`config.go:87`), so `mcp`/`mcp_agents` ride along automatically.

## 8. Making dotagents a plugin

Skills-only, matching §11.2's explicit blessing ("a skills-only client can
conform... provided it satisfies all applicable requirements") mirrored the
other direction: a skills-only *package*.

New file, committed at repo root (this checkout doubles as the maintainer's
`~/.agents`, per `CLAUDE.md`'s "`~/.agents` is symlinked to this repo"):

`/Users/conscience/Workspace/dotagents/plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "dotagents",
  "description": "Skills and agent roles synced by dotagents across coding-agent harnesses.",
  "homepage": "https://github.com/yourconscience/dotagents",
  "repository": "https://github.com/yourconscience/dotagents",
  "license": "MIT"
}
```

Decisions:

- **No `version` field.** A hand-maintained version string here would go
  stale the moment it's not wired into the release process (`.goreleaser.
  yaml`), and a permanently-stale value is worse than an absent optional
  field (§5.4 already treats `version` as fully optional).
- **No `mcp.json` at the config root.** `cfg.MCPServers` entries in
  `dotagents.yaml` carry command/args/env the user configured for *their
  own* harnesses, often with absolute local paths; dotagents is a public
  repo. §11.2 explicitly allows shipping only the skills component.
- **No `extensions` field.** dotagents doesn't control a reverse-domain
  namespace and has nothing to put under one (§1 Framing) — inventing
  `org.dotagents.cli` for no consumer would be exactly the kind of
  unsupported-surface invention `AGENTS.md` prohibits.

Wiring:

- `starter_assets.go`'s `//go:embed` directive gains `plugin.json`:
  `//go:embed .gitignore AGENTS.md dotagents.yaml dotagents.lock plugin.json agents/*.md skills/dotagents skills/grilling memory/hooks memory/lib`.
- `ensureStarterAssets` (`setup_scaffold.go:69`) needs **no code change** —
  it already `fs.WalkDir`s the whole embedded FS generically and does
  copy-if-missing (`os.Lstat` check at line 87) for every path except the
  one special-cased `dotagents.yaml` target rewrite (line 81). `plugin.json`
  falls through the generic branch: created on first `dotagents setup` if
  absent, never overwritten afterward — treated like `AGENTS.md`, not like
  the generated README block (`renderREADMESkills`).
- This is new starter content, so per `AGENTS.md`: "New starter content
  requires an explicit product decision and an inventory test update." The
  product decision is this document; the test update is
  `setup_separation_test.go` (§9, test 16).

## 9. Test plan

New files `pluginspec_test.go`, `pluginmcp_test.go`; additions to
`external_test.go`, `lock_test.go`, `setup_separation_test.go`.

1. `parsePluginManifest`: minimal valid manifest (`$schema`+`name` only) →
   `ok=true`, zero `unknown`.
2. Full valid manifest (all optional fields) → all fields populated
   correctly, `Author` parsed.
3. Missing `$schema`, wrong `$schema` value, missing `name`, empty `name`,
   `name` too long, `name` with uppercase/leading hyphen/`--`/`..` → each
   `ok=false` with a distinguishable error.
4. Unknown top-level field (e.g. `"engines": {}`) → `ok=true`,
   `unknown=["engines"]`, other fields still parsed (§5.2 non-fatal path).
5. `extensions` present but a string/array instead of an object → `ok=true`
   (per §8.1, non-fatal), reported in `unknown`.
6. `author` with an unknown nested field (e.g. `"twitter": "..."`) →
   `ok=false` (§5.4: any other field is invalid) — distinct from case 4.
7. Symlinked `plugin.json` resolving outside the cache root → treated as
   absent (containment, §4.1 rule 1).
8. Discovery relaxation: plugin.json present, default `skill_dir`, no
   `skills/` directory at all → `discoverExternalSourceSkills` returns zero
   skills, **no error** (§6.2), and `syncExternalRepos` succeeds.
9. Regression guard: same fixture but with an explicit `skill_dirs:
   [docs/skills]` override that's missing on disk → still errors exactly as
   today (relaxation must not apply to explicit overrides).
10. Regression guard: full existing `external_test.go` suite (no
    `plugin.json` in any fixture) stays green unmodified — proves zero
    behavior change for non-plugin sources.
11. `discoverPluginMCPServers`: one valid `stdio` server with
    `${PLUGIN_ROOT}`/`${PLUGIN_DATA}` in `args` and `env`, and a `./bin/x`
    `command` → projected `mcpServerConfig` has an absolute, contained
    `Command`, expanded `Args`, `Env["PLUGIN_ROOT"]`/`Env["PLUGIN_DATA"]`
    set to the real absolute paths.
12. Server with `cwd` set (any value, including `${PLUGIN_ROOT}`) → skipped
    + reported, not projected (documented limitation, §4 step 5).
13. Server whose raw `env` contains a `PLUGIN_ROOT` or `PLUGIN_DATA` key →
    skipped + reported (§9.2).
14. Server with `type: "streamable-http"` alongside a valid `stdio` sibling
    → the http one is skipped + reported "unsupported transport", the
    stdio one still projects (§7.2.2 rule 4, independent-failure
    guarantee).
15. `mcp.json` `$schema` version mismatched against `plugin.json`'s → MCP
    fully disabled for the source (zero servers, one problem), skills from
    the same source still discovered.
16. `command` escaping the plugin root (`../../etc/passwd`, or an absolute
    path) → invalid entry, skipped + reported.
17. Server name containing `.` or uppercase → skipped + reported (dotagents
    -specific stricter-than-schema rule, §4 step 8).
18. `expandPluginPlaceholders`: adversarial case where `pluginData` itself
    contains the literal substring `${PLUGIN_ROOT}` → the expanded output
    still contains that literal text unexpanded (proves single-pass, not
    chained `ReplaceAll`).
19. `src.MCP == false` (default) with a fully valid `mcp.json` present →
    zero plugin-derived entries appear in the merged `cfg.MCPServers`
    (opt-in verified).
20. Name collision, plugin vs. user `cfg.MCPServers` entry with the same
    name → user's entry wins, plugin's is skipped + reported, no error.
21. Name collision, two different sources both project the same server
    name → both entries are skipped + reported; `loadContext` still
    succeeds (status/doctor/sync all remain usable, per §4 "Name
    collisions").
21a. Source not yet cloned (fresh `dotagents setup`, `~/.agents/external/
    <name>/` doesn't exist): `loadContext` succeeds with zero plugin-
    derived MCP servers and zero warnings — not an error, not a report
    (§4 "Not-yet-cloned sources").
22. `PLUGIN_DATA` persistence: write a marker file into
    `external-data/<repoName>/`, run `dotagents external update <name>`
    (which re-clones/resets the *cache*), assert the marker survives.
23. Lock round-trip: source with a valid `plugin.json` → `dotagents.lock`
    gains `plugin_name`/`plugin_version`; re-running sync with no changes
    doesn't rewrite the lock file (extends the existing
    `writeLockIfChanged`/`lockEntriesEqual` coverage).
24. `TestEnsureStarterAssetsIncludesPluginManifest` (new, in
    `setup_separation_test.go`): fresh `dotagents setup` creates
    `~/.agents/plugin.json`; a second run with a user-edited `plugin.json`
    present leaves it untouched (copy-if-missing, matching the existing
    `TestEnsureStarterAssetsCreatesMissingOnlyAndExecutableHooks` pattern).
25. `TestCommittedPublicSkillInventoryMatchesLaunchSet` (existing,
    `readme_inventory_test.go`) stays green unmodified — proves the
    skills-only root `plugin.json` doesn't interact with the README
    skill-count generator.

## 10. File-by-file change list

| File | Change |
| --- | --- |
| `cmd/dotagents/pluginspec.go` (new) | `pluginManifest`, `pluginAuthor`, `parsePluginManifest`, `hasPluginManifest`, name-pattern/known-field validation. |
| `cmd/dotagents/pluginmcp.go` (new) | `pluginMCPServerRaw`, `discoverPluginMCPServers`, `expandPluginPlaceholders`, plugin-relative `command` resolution + containment, server-name validation, transport/`cwd`/reserved-env rejection. |
| `cmd/dotagents/main.go` | `externalSkillSource` gains `MCP bool` and `MCPAgents []string`. |
| `cmd/dotagents/config.go` | `validateConfig` normalizes/validates `MCPAgents` (mirrors the existing `cfg.MCPServers[i].Agents` loop). |
| `cmd/dotagents/lock.go` | `externalLockEntry` gains `PluginName`/`PluginVersion` (plain strings, keeps `==` comparability); `rebuildLockEntries` populates them best-effort via `parsePluginManifest`. |
| `cmd/dotagents/external.go` | `externalDataDir(home)` helper; `discoverExternalSourceSkills` takes a plugin-relaxation flag that skips the "missing skill dir" and "zero skills found" hard errors only when plugin-mode + default skill dir; new `injectPluginMCPServers(cfg *config, home string) error` that iterates `cfg.ExternalSkills` where `MCP == true`, calls `discoverPluginMCPServers`, applies the collision rules (§4), appends to `cfg.MCPServers`, and prints warnings for every `problems` entry via `fmt.Fprintf(os.Stderr, ...)` (existing convention, e.g. `external.go:49`). |
| `cmd/dotagents/lock.go` (`loadContext`) | After `cfg, err := loadConfig(...)`, call `injectPluginMCPServers(&cfg, home)` before `selectAgents` — single insertion point so `status`, `sync`, `doctor`, and any other `loadContext` caller see the same merged view. |
| `cmd/dotagents/external_cli.go` | `externalList` appends `plugin <name>@<version>` to its printed line when the lock entry has plugin metadata (cosmetic; no behavior change). |
| `cmd/dotagents/doctor.go` | Optional (not required for v1): a `checkExternalPluginSources` `DoctorCheck` that dry-runs manifest/`mcp.json` parsing across configured sources and surfaces `problems` without requiring a sync, matching `checkSkillSpec`'s style. |
| `starter_assets.go` | `//go:embed` directive gains `plugin.json`. |
| `/plugin.json` (new, repo root) | Skills-only manifest, see §8. |
| `cmd/dotagents/pluginspec_test.go` (new) | Test plan items 1–7. |
| `cmd/dotagents/pluginmcp_test.go` (new) | Test plan items 11–21. |
| `cmd/dotagents/external_test.go` | Test plan items 8–10, 22. |
| `cmd/dotagents/lock_test.go` | Test plan item 23. |
| `cmd/dotagents/setup_separation_test.go` | Test plan item 24. |
| `README.md`, `skills/dotagents/SKILL.md` | Document: (a) an `external_skills` source may optionally be an Agent Plugins v1.0.0 package (`plugin.json`/`mcp.json`); what's consumed (`stdio` MCP only, skills via the existing default location) and what's opt-in (`mcp: true`); (b) invalid `plugin.json` degrades gracefully rather than disabling the source, a named deviation from strict client conformance; (c) the dotagents config root ships its own skills-only `plugin.json` for consumption by other Agent Plugins v1 clients. |

## Risks and open questions

1. **`cwd` is unrepresentable.** Any `mcp.json` server that needs a
   non-default working directory is unusable via this path today. Fixing
   it means adding `Cwd` to `mcpServerConfig` and teaching every harness
   writer (JSON/YAML/TOML/Claude-project-map) to set it — a real feature,
   out of scope here. Flagged, not blocked on.
2. **Orphaned `external-data/<repoName>/` directories** after a source is
   removed from config or its URL changes (new `repoName`). No pruning in
   v1; candidate follow-up: fold into `external update`'s existing
   stale-materialized-skill cleanup pass, or a `dotagents external prune`
   verb.
3. **Dotted/uppercase MCP server names are valid per spec but rejected by
   dotagents** (§4 step 8, TOML-safety). A plugin author who only tests
   against a client with no such restriction may be surprised their server
   silently doesn't appear. Mitigated by the required `problems` report;
   worth surfacing prominently in `dotagents doctor` (see optional check
   above) rather than only at sync time.
4. **`streamable-http`/`sse` MCP servers are permanently invisible** to
   dotagents until `mcpServerConfig` grows a `Type`/`URL`/`Headers` variant
   and every harness writer learns to emit it. This is a bigger, separate
   design (probably worth its own doc) — noted here so it isn't confused
   with this feature's scope.
5. **Fatal-manifest framing is a deliberate spec deviation.** If dotagents
   later wants to claim any form of "Agent Plugins conformant" labeling,
   this decision (§1 Framing) needs revisiting — as designed, dotagents is
   a pragmatic consumer, not a conformant client, and should not claim
   conformance in docs or `$schema` handling.
6. **Bare `command` resolution is harness-dependent, not dotagents-
   guaranteed.** §4 step 4 passes a bare executable name through unchanged
   and relies on "the harness's own PATH resolution" — concretely, for the
   Codex writer this means `renderCodexMCPSection` (`mcp.go:601`) emits
   `command = %q` into TOML with no shell, so resolution depends entirely
   on whatever `PATH` Codex itself has at launch, which dotagents does not
   control or verify. This matches the spec's "client-defined" carve-out
   (§7.2.1), but a plugin author relying on a bare command working
   identically across every harness dotagents targets may be surprised.
   Document in the README update (§10) that plugins bundling their own
   executable should use a `./`-relative `command` for deterministic
   resolution, per the spec's own recommendation.
