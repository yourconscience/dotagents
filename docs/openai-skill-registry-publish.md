# Design: `dotagents publish` — OpenAI skill registry target

Status: **core implemented on branch `feat/publish-openai-skills` (2026-09-12).** Config schema, lock schema, bundler, upload, and the `dotagents publish` command with dry-run/json/confirmation are built and unit-tested (`publish_test.go`, full suite green). Still inert by default: no targets are configured and every target is opt-in via `enabled: true`. Deferred pending live-API verification: `--prune`, server-side `status` reconcile, and any dependence on list/delete endpoints (undocumented — see §10). No live upload has run yet.
Date: 2026-09-12
Author trigger: OpenAI Agents API public beta (Codex harness). Background research lives in the maintainer's private knowledge vault under `research/` (not distributed with this repo).

## 1. Summary

OpenAI's Agents API loads the same open Agent-Skills `SKILL.md` that dotagents already treats as canonical, and it exposes a **persistent skill registry**: `POST https://api.openai.com/v1/skills` uploads a skill bundle, stores it server-side, returns a `skill_id`, and manages versioned bundles. The Responses API references skills by `skill_id` + `version`.

This is a **publish verb**, not a sync entity. It does not belong in the `agents:` list. It maps cleanly onto the existing `external_skills` + `dotagents.lock` machinery, but in the opposite direction: `external_skills` materializes remote skills *inward* and pins their upstream commit; `publish` pushes canonical skills *outward* to a remote registry and pins the returned id/version.

## 2. Why not an `agents:` entity (the decision)

Every `agentConfig` in `dotagents.yaml` is a locally-installed harness with:
- a `skill_root` (an on-disk directory dotagents reconciles by placing skills), and
- a `detect` key (checks whether the harness is installed on this machine).

The Agents API has neither. It is a cloud API, not a local install; there is no directory to reconcile and nothing to detect. And it runs the **Codex harness** — the `codex` entity already produces the canonical `SKILL.md` the Agents API consumes. Adding an `openai-agents` row would:
- duplicate the `codex` skill output,
- break the "reconcile a local directory" invariant every current entity satisfies, and
- misfile a per-session runtime concern (`capability_directories`) as a local sync target.

So the format side needs **zero** work — it is already handled by the `codex` entity. The only genuinely new surface is the registry transport, and that is a publish action.

### Scope boundary: registry, not sandbox
Two OpenAI skill-loading mechanisms exist:
- **`capability_directories`** — ephemeral, per-session, from sandbox filesystem paths supplied at session-create time by the *application* calling the Agents API. Out of scope for dotagents.
- **`/v1/skills` registry** — persistent, upload-once, reference-by-id. **This is the only surface dotagents targets.**

## 3. Config schema (`publish_targets:`)

New top-level section in `dotagents.yaml`, sibling to `agents:`, `external_skills:`, etc. Struct mirrors `externalSkillSource` in style.

```yaml
publish_targets:
    - name: openai            # unique target name
      kind: openai-skills     # registry kind (only value for now)
      enabled: false          # default off; opt-in per user privacy/cost stance
      skills:                 # explicit allowlist of skill dir names to publish
        - jobs
        - tech-search
      # optional: bump strategy when content changed. default: new-version
      version_strategy: new-version   # new-version | set-default | dry-run-only
      # auth: env var name holding the API key. never inline the key.
      api_key_env: OPENAI_API_KEY
```

Proposed Go struct (in `main.go`, alongside `externalSkillSource`):

```go
type publishTarget struct {
    Name            string   `yaml:"name"`
    Kind            string   `yaml:"kind"`              // "openai-skills"
    Enabled         bool     `yaml:"enabled"`
    Skills          []string `yaml:"skills"`            // allowlist of local skill dir names
    VersionStrategy string   `yaml:"version_strategy,omitempty"` // default "new-version"
    APIKeyEnv       string   `yaml:"api_key_env,omitempty"`      // default "OPENAI_API_KEY"
}
```

Add `PublishTargets []publishTarget `yaml:"publish_targets,omitempty"`` to `config`.

Design choices:
- **Explicit allowlist, no "publish everything".** Prevents accidentally shipping private/experimental skills to a US-only, no-ZDR service. A skill is published only if named here.
- `enabled: false` by default; the command is a no-op until the user opts a target in.
- No `skill_root`/`detect` — this is a push, not a reconcile.

## 4. Lock schema additions

Extend `dotagents.lock` to pin published state, mirroring `externalLockEntry`.

```yaml
version: 1
external_skills:
    - name: skills
      ...
published_skills:
    - target: openai
      skill: jobs
      skill_id: skill_abc123
      version: "4"   # string: the registry may return non-numeric pointers (e.g. "latest")
      content_hash: sha256:...   # hash of the bundled skill tree
      published_at: 2026-09-12T10:00:00Z
```

Proposed Go structs (in `lock.go`):

```go
type lockFile struct {
    Version         int                  `yaml:"version"`
    ExternalSkills  []externalLockEntry  `yaml:"external_skills"`
    PublishedSkills []publishedLockEntry `yaml:"published_skills,omitempty"`
}

type publishedLockEntry struct {
    Target      string `yaml:"target"`
    Skill       string `yaml:"skill"`
    SkillID     string `yaml:"skill_id"`
    Version     string `yaml:"version"`
    ContentHash string `yaml:"content_hash"`
    PublishedAt string `yaml:"published_at"`
}
```

The `content_hash` is the idempotency key: if a skill's bundled tree hashes to the same value already in the lock, publish is skipped. This is the outward analogue of `externalLockEntry.Commit`.

## 5. CLI

New top-level command, flag style matching `sync`:

```bash
dotagents publish                       # publish all enabled targets' allowlisted skills that changed
dotagents publish --target openai       # limit to one target
dotagents publish --skills jobs,tech-search   # limit to named skills
dotagents publish --dry-run             # show what would upload; no network writes
dotagents publish --json                # machine-readable result
dotagents publish -y                    # skip the confirmation prompt (CI)
```

Default behavior (no flags): for each `enabled` target, diff each allowlisted skill's `content_hash` against the lock; upload only changed/new skills; update the lock with returned `skill_id`+`version`. Unchanged skills print "up to date" and are skipped.

Placement: new `cmd/dotagents/publish.go` (flat, matching `sync.go`, `mcp.go`, `promote.go`). Register in the command dispatcher next to `sync`/`mcp`.

## 6. Bundling & validation (enforce API limits before upload)

Before any network call, build the bundle and validate against documented limits:
- zip ≤ 50 MB, ≤ 500 files, ≤ 25 MB uncompressed;
- exactly one top-level folder in the zip;
- exactly one `SKILL.md` with valid front matter (`name`, `description`).

Reuse the existing skill-discovery / skillspec validation (`skill_discovery.go`, `skillspec.go`) for the front-matter and structure checks so the same parser gates both local sync and publish. A skill that fails local validation must fail publish with the same error — no divergent validators.

Bundling: stdlib only — `archive/zip` + `mime/multipart` + `net/http`. **No OpenAI SDK dependency** (a multipart POST does not warrant one; keeps the tree dependency-free and avoids a new external package to age-check).

## 7. Auth, privacy, cost gates

- API key read **only** from the env var named by `api_key_env` (default `OPENAI_API_KEY`). Never inline in `dotagents.yaml`, never write it to the lock, never log it.
- Publish is destructive-adjacent (sends skill content to a third party). It therefore:
  - requires `enabled: true` per target,
  - requires an explicit skill allowlist,
  - prompts for confirmation unless `-y`, showing the exact skill names and target,
  - refuses if `OPENAI_API_KEY` is unset rather than falling back to any other credential.
- Print a one-line privacy reminder on every real (non-dry-run) publish: skills are uploaded to a US-only, no-ZDR service; do not publish vault- or secret-bearing skills.

## 8. `status` integration

`dotagents status` gains a compact "published" section per enabled target: which allowlisted skills are up to date (hash matches lock), drifted (local hash differs → would re-publish), or never published. Mirrors how `status` already reports managed/drifted/missing for local skill roots. Verbose mode lists `skill_id`+`version`.

## 9. Verification plan (when built)

- Unit: bundle builder respects file/size/single-folder limits; content_hash is stable and order-independent; validator rejects missing/duplicate `SKILL.md`.
- Idempotency: second `publish` with no local change performs zero network writes and leaves the lock byte-identical.
- Dry-run: `--dry-run` performs no network writes and prints the planned uploads.
- Live smoke (opt-in, one throwaway skill, real key): upload → assert returned `skill_id`+`version` land in the lock → re-run → assert skip. Delete the test skill from the registry afterward.
- No live test touches allowlisted real skills or the private vault.

## 10. Open questions

- Does `/v1/skills` support **delete** and **list**? Needed for a `dotagents publish --prune` (remove registry skills no longer in any allowlist) and for `status` to reconcile against the server rather than only the lock. Verify against the current API before building either.
- Version semantics: does uploading identical content create a new version or dedupe server-side? If the server dedupes, the local `content_hash` skip is an optimization, not a correctness requirement.
- `agents/openai.yaml` sidecar (display_name/icons/brand_color/default_prompt): include in the bundle when present, but never hand-maintain — generate on demand via the upstream `generate_openai_yaml.py`. Out of scope for v1; base `SKILL.md` publishes fine without it.
- Should Codex's own `client.beta.agents.sessions` / `capability_directories` path ever be dotagents' concern? Current answer: no — that is the calling application's runtime, not a config-store sync surface.

## 11. Non-goals / gate

- Not building this now. It ships only when there is a concrete long-running cloud-agent workload that justifies metered API billing (tokens + tools + container time), separate from the ChatGPT/Codex subscription.
- Not an `agents:` entity (see §2). Do not add `openai-agents` to the harness list.
- Not touching `capability_directories`, sandboxes, or `codex exec-server`.
- Not a new skill; no new runtime dependency; no automation/cron.
```
