# SPEC

## Goal

Add a local-first authoring surface for dotagents that works as:

1. a responsive web UI on desktop and mobile;
2. a Bubble Tea TUI in the terminal;
3. the existing editable YAML file.

All three are projections of the same canonical `dotagents.yaml` document. The web UI and TUI must never create a second configuration store or write native harness configuration directly. `dotagents sync` remains the only renderer from canonical config into harness-specific files.

The web UI ships inside the existing `dotagents` binary and runs as a standalone local process. It must not require HarnessKit, Node, or a separate daemon at runtime.

## Non-goals

- Replacing HarnessKit as the native-harness inspection and audit viewer. `dotagents view` remains the HarnessKit launcher.
- Moving skill bodies, role Markdown, plugin source, memory, or lock data into `dotagents.yaml`. These remain canonical repo files; YAML owns configuration and references to those files.
- Editing arbitrary files outside the resolved dotagents config root.
- Installing a LaunchAgent, daemon, Tailscale Serve rule, or public tunnel automatically.
- Managing active agent sessions, terminals, prompts, or usage dashboards.
- Writing directly to `~/.claude`, `~/.codex`, `~/.omp`, `~/.hermes`, or other materialized harness directories from the UI.

## Commands

Keep the new surface under one command family so first-run ownership stays with `setup` and the short top-level command list does not grow unnecessarily.

```text
dotagents config                         # interactive TUI
dotagents config serve                   # web UI, loopback only, opens browser
dotagents config serve --no-open
dotagents config serve --addr 127.0.0.1:8765
dotagents config validate                # validate canonical YAML without writing
dotagents config print                   # print resolved paths and effective config
```

If no canonical config exists, `dotagents config` and `config serve` direct the user to `dotagents setup`; they do not implement a second first-run flow.

## Canonical configuration model

### Files and layers

- Resolution remains: `--config` → `$DOTAGENTS_HOME/dotagents.yaml` → `~/.agents/dotagents.yaml`.
- `dotagents.yaml` is the canonical shared layer.
- `dotagents.local.yaml` is the optional machine-local canonical overlay.
- The UI exposes three explicit modes: **shared**, **local**, and **effective**.
- Shared and local are editable. Effective is a read-only merged projection. The UI never flattens the overlay back into the shared file.
- `dotagents.lock` stays sync-owned and read-only in this feature.

### Shared config service

Add one concrete `configDocument` service used directly by CLI, TUI, and HTTP handlers. Do not add an interface until a second implementation exists.

Responsibilities:

1. Resolve the canonical paths with the existing resolver and reject linked worktree roots with `refuseWorktreeRoot`.
2. Read YAML into a `yaml.Node` document and decode the same document into the existing typed `config` struct.
3. Apply the local overlay through the existing merge semantics, extended so a non-nil local `ui` block replaces the shared `ui` block wholesale, matching the overlay's current whole-entry behavior.
4. Validate editable bytes with `validateConfig(..., expand=false)` so portable `~` paths remain unchanged. Use `expand=true` only on a separate typed copy for effective views and sync planning.
5. Address list entries by stable keys:
   - agents: normalized `name`;
   - MCP servers: `name`;
   - hooks: `name`;
   - external skills: repository identity derived from `url`.
6. Mutate only the selected YAML nodes. Structured edits must preserve unknown fields, comments, ordering, quoting, and untouched subtrees.
7. Allow raw YAML replacement only after parsing and validation succeeds.
8. Compute a SHA-256 revision from the exact bytes read. Reject stale writes instead of overwriting a file changed by another process.
9. Write atomically in the same directory: temporary file, preserved file mode, flush/close, rename. A failed validation or write leaves the original bytes unchanged.
10. Return the exact before/after unified diff for every proposed save.

Do not use `yaml.Marshal(config)` for interactive edits: the current setup/MCP writers reserialize the whole struct and can drop comments or unknown future fields. The config UI needs node-level mutation so it remains forward-compatible.

Before `ui` ships, add typed `UI`/`Links` fields to `config`, extend `mergeConfig`, and route every canonical whole-document writer, including `writeSetupConfig` and `writeEditableMCPConfig`, through `configDocument`. Otherwise an existing `setup` or `mcp` command could silently remove `ui`, comments, or unknown fields.

### Editable fields

The structured editor covers every field currently represented by `config`:

- version;
- agents: name, enabled, detect command, skill root, agent root, role model;
- external skills: URL, branch, skill directory/directories, selected skills, materialize, MCP enablement and target agents;
- MCP servers: name, enabled, command, arguments, environment, and target agents;
- hooks: name, enabled, event, command, timeout, target agents;
- context note token threshold;
- UI navigation links: name and absolute HTTPS URL or origin-relative path.

### Linked surfaces

Add an optional UI-only section to the canonical schema:

```yaml
ui:
  links:
    - name: Usage
      url: /usage
```

- Links are navigation metadata, not a sixth synced harness surface.
- `url` accepts an absolute `https://` URL or an origin-relative path beginning with `/`.
- Relative `/usage` is preferred for the personal Tailscale setup: when dotagents is served under `/dotagents` and the existing usage app remains under `/usage`, both use the same tailnet origin without storing a machine hostname in public configuration.
- The Settings screen edits these links through the same YAML diff/save flow. The user's machine-specific link belongs in `dotagents.local.yaml`; public starter configuration stays machine-neutral.
- The web header shows **Settings** and configured links such as **Usage**. Links open as normal top-level navigation, never in an iframe, and never proxy credentials or usage data through dotagents.
- TUI shows the same links in its UI section and can copy/open the selected URL where the platform supports it.

Raw YAML remains available for future or unknown fields. Unknown fields must survive structured edits even before the forms understand them.

## Mutation flow

Every write follows the same review-first state machine in web and TUI:

```text
Edit → Validate → Preview YAML diff → Save canonical YAML
                                      ↓ optional, separate action
                               Preview sync plan → Confirm → Sync
```

- Saving YAML does not implicitly run `sync`.
- Sync preview reuses the existing inspection/report pipeline.
- Sync apply requires the preview digest and current config revision. If either changed, preview again.
- Destructive removals and role overwrites retain the existing per-harness confirmation semantics.
- API errors use stable codes (`invalid_yaml`, `invalid_config`, `stale_revision`, `sync_plan_changed`) plus actionable text.

## Web API

Minimum JSON API:

```text
GET  /api/state                 paths, active layer, typed config, raw YAML, revision
POST /api/config/validate       candidate raw YAML or structured patch; returns errors + diff
PUT  /api/config/raw            expected revision + complete YAML; validates and writes
PATCH /api/config               expected revision + keyed operations; validates and writes
POST /api/sync/preview          returns per-harness plan + digest
POST /api/sync/apply            expected revision + plan digest + confirmed destructive items
GET  /api/status                current per-harness sync state
```

No generic filesystem, shell, command, or path endpoint. Responses must not expose environment values marked as secrets; environment keys may be displayed, values are masked by default and only sent when editing the selected canonical YAML field.

## Web security and mobile access

- Bind only to loopback. Reject wildcard and non-loopback addresses.
- Generate a random startup token. Bootstrap it into an `HttpOnly`, `SameSite=Strict` session cookie and remove the token from the visible URL. `dotagents config serve --secure-cookie` additionally marks it `Secure` and is required when the browser reaches the loopback server through Tailscale HTTPS; plain local HTTP omits that flag.
- Require the session for every API request, validate `Origin`, and require a CSRF header for mutations.
- Send `Cache-Control: no-store`, a restrictive CSP, `X-Content-Type-Options: nosniff`, and `frame-ancestors 'none'`.
- Keep configuration and secrets in memory only for the request lifecycle; never log request bodies or tokens.
- Tailscale access is an explicit operator step, for example:

```bash
dotagents config serve --no-open --secure-cookie --addr 127.0.0.1:8765
tailscale serve --bg --set-path /dotagents http://127.0.0.1:8765
```

The Tailscale syntax above was verified against the installed client on 2026-09-12. The application does not install or persist this route itself.

## Web UX

### Information architecture

Desktop uses a compact editor rather than a card dashboard:

```text
┌ Sources ──────┬ Configuration ───────────────────┬ Change rail ─────┐
│ Shared        │ Agents / MCP / Hooks / Sources   │ YAML lines       │
│ Local         │ searchable list + detail editor  │ validation       │
│ Effective     │                                  │ diff + save      │
└───────────────┴──────────────────────────────────┴───────────────────┘
```

Mobile becomes a single drill-down stack:

```text
Sources → Section list → Item editor → Diff / Save
```

A sticky bottom bar contains only context-valid actions: **Validate**, **Review changes**, **Save YAML**. **Preview sync** and **Sync** remain a separate final screen.

### Visual direction

Treat the product as an instrument panel for configuration provenance, not a generic SaaS dashboard.

- Memorable element: the **change rail**, which maps a structured field to its canonical YAML source lines and layer.
- Layout: dense left-aligned ledger rows, clear nesting, no grid of rounded cards.
- Palette: paper `#F6F7F9`, ink `#18202A`, graphite `#46515F`, cobalt `#155EEF`, success `#16803B`, danger `#B42318`.
- Type: native UI sans for controls; native monospace only for paths, commands, diffs, and YAML. No network fonts.
- Motion: only state transitions for opening an item and revealing validation/diff results; respect reduced motion.
- Accessibility: semantic controls, visible focus, keyboard navigation, 44px mobile targets, safe-area padding, WCAG AA contrast, and no horizontal page overflow at 320px.

The distinguishing choice is provenance visibility: every value says whether it comes from shared YAML, local overlay, a default, or the effective merge. Remove decoration that does not communicate that state.

### Frontend delivery

Use separate `index.html`, CSS, and JavaScript modules embedded with `go:embed`. Start with browser-native HTML controls and ES modules; no frontend framework or build-time Node dependency. Add a framework or editor package only after measured complexity proves native controls insufficient.

## TUI UX

Reuse Bubble Tea and Lip Gloss already in the module. The TUI and web UI share the config service and mutation state machine, not rendering code.

```text
shared | local | effective
agents  mcp  hooks  external skills  advanced  yaml

> claude-code   enabled   ~/.claude/skills
  codex         enabled   ~/.codex/skills

[e] edit  [/] search  [y] yaml  [v] validate  [r] review  [s] save  [q] quit
```

- Wide terminals show section list, detail form, and change rail.
- Narrow terminals show one pane at a time.
- The YAML tab is a real editable text area, not a shell-out to `$EDITOR` in the first version.
- Save always opens the same validation + diff confirmation view used by structured edits.
- External file changes surface as a stale-revision screen with **reload** or **copy unsaved YAML**; never overwrite.

## Implementation plan

### Slice 1: canonical document editing

- Introduce `config_document.go` with YAML node loading, typed decoding, stable-key lookup, validation, revisions, diff generation, and atomic write.
- Add typed `UI`/`Links` fields and explicit local-overlay semantics, then route every canonical whole-document writer, including `writeSetupConfig` and `writeEditableMCPConfig`, through `configDocument` before linked surfaces ship. Existing commands must not strip `ui`, comments, or unknown fields.
- Add `dotagents config validate` and `dotagents config print` as non-interactive proof of the shared service.
- Focused tests: comment/unknown-field preservation, overlay isolation, invalid-write rollback, stale revision, and file-mode preservation.
- Fresh canonical files use mode `0o644`; replacement preserves the existing file mode.

### Slice 2: TUI authoring

- Add `dotagents config` using the existing Bubble Tea dependency.
- Implement section navigation, complete field editing, raw YAML, validation, diff review, and save.
- Reuse existing setup review styles where useful, but keep setup import review and config editing as separate models.
- Exercise it against a temporary config root in a real PTY; tests never touch live harness configuration.

### Slice 3: web server and desktop UI

- Add `dotagents config serve` with loopback validation, token bootstrap, security headers, revision-aware APIs, and embedded separate assets.
- Implement all structured fields, raw YAML, diff/save, status, and sync preview.
- Keep sync apply behind a separate explicit confirmation screen.
- Exercise the actual page in a browser against a temporary config root: structured edit → diff → save → reload → raw YAML edit.

### Slice 4: mobile and Tailscale proof

- Verify the 320px and current iPhone viewport flows with the real browser surface: navigation, YAML editing, validation errors, diff review, stale-write recovery, and sync confirmation.
- Run the server on loopback and expose it through a temporary Tailscale Serve path; verify HTTPS access from a tailnet client without changing existing routes.
- Document the operator-owned Tailscale command only after the live proof. Do not install persistent automation.

### Slice 5: safe sync completion and documentation

- Complete preview-digest guarded sync apply in web and TUI.
- Update README, CLI help, `skills/dotagents/SKILL.md`, setup docs, and release-site copy together.
- Keep `dotagents view` documented as HarnessKit inspection; document `dotagents config` as canonical authoring.
- Remove throwaway smoke scripts and update this spec with Outcome / Deviations.

## Acceptance tests

1. A structured edit in web changes the selected YAML node, survives reload, and appears immediately in TUI and raw YAML.
2. A TUI edit appears in web after reload and changes no native harness file until explicit sync.
3. A raw YAML edit updates the structured form after validation.
4. Structured editing preserves comments, unknown keys, ordering, quoting, and unrelated bytes where possible.
5. Editing `dotagents.local.yaml` changes only the local layer; the effective view reflects the merge and shared YAML remains byte-identical.
6. Invalid YAML or invalid typed config cannot replace the canonical file.
7. Concurrent external modification produces `stale_revision`; neither web nor TUI overwrites it.
8. Save shows the exact YAML diff. Sync shows a separate per-harness plan and rejects a changed digest.
9. Web server rejects non-loopback binds, unauthenticated API calls, invalid origins, CSRF-less mutations, and arbitrary path requests.
10. Desktop and 320px mobile browser smoke complete edit → validate → review → save without horizontal page overflow.
11. Temporary Tailscale HTTPS access works while the server remains loopback-bound; removing the temporary route removes remote access.
12. Focused tests and `go test ./...` pass without mutating live harness configuration.
13. With the personal local overlay containing `ui.links: [{name: Usage, url: /usage}]`, desktop and mobile show a working **Usage** navigation link while the Settings view can edit it through the normal YAML review flow.

## Risks / open questions

- `yaml.v3` preserves node metadata but can still normalize formatting around modified nodes. The Slice 1 preservation tests define the acceptable boundary before UI work starts.
- Environment values may contain secrets. The UI needs masking and log redaction; deciding whether to reveal existing values at all should be made during Slice 1 threat modeling.
- `dotagents.local.yaml` replacement semantics are whole-entry, not field-level. The editor must explain this; changing overlay semantics is out of scope.
- Browser text editing on iOS can be awkward. Start with a normal textarea; add a code editor dependency only if live mobile testing shows a concrete failure.
- Existing experimental dashboard work from May 2026 was a read-only catalog and session launcher on an obsolete repo layout. Reuse its proven loopback guardrails and separate-asset lesson, not its API or information architecture.

## Codebase notes

- Existing typed schema: `cmd/dotagents/main.go`, `mcp.go`, and `hooks.go`.
- Existing resolution, overlay, validation, and worktree safety: `cmd/dotagents/config.go`.
- Existing TUI foundation: `cmd/dotagents/review.go` and `review_apply.go`.
- Existing HarnessKit boundary: `cmd/dotagents/view.go` and `docs/harnesskit-integration.md`.
- Existing whole-document writers: `writeSetupConfig` and `writeEditableMCPConfig`; these are insufficient for comment/unknown-field-preserving interactive edits.

## Outcome / Deviations

Implemented in the current checkout:

- `configDocument` now owns YAML node loading, typed validation, stable-key
  mutations, SHA-256 revisions, atomic mode-preserving writes, and unified
  diffs. Shared and local overlays remain isolated; a non-nil local `ui`
  block replaces shared UI links.
- Setup and MCP canonical whole-document writes route through the document
  service, preserving unknown fields and comments instead of flattening the
  typed struct.
- `dotagents config`, `config validate`, and `config print` are wired. The
  Bubble Tea editor has shared/local/effective tabs, raw YAML editing,
  validation, diff review, save, and stale-revision protection.
- `config serve` embeds separate HTML/CSS/ES-module assets and exposes the
  revision-aware state, validation, raw/structured mutation, sync preview,
  guarded sync apply, and status APIs. Loopback binding, startup-token
  bootstrap, strict session cookies, origin/CSRF checks, security headers,
  secret masking, and `--secure-cookie` are implemented.
- Public help, README, skill documentation, setup documentation, and release
  site copy describe canonical authoring separately from HarnessKit `view`.

Known deviations:

- The TUI uses a native minimal textarea rather than a full structured form;
  every schema field remains editable through its YAML tab, while web
  structured controls currently expose agent enablement and raw YAML covers
  the complete schema.
- Browser and temporary Tailscale HTTPS proof require a desktop browser and
  tailnet route outside this checkout. Focused HTTP/API and asset checks are
  included; no persistent route or automation is installed.
