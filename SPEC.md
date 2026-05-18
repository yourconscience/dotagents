# SPEC

## Goal

Add a dotagents-owned dependency safety check that rejects external package/tool references whose resolved package version was published less than 7 days ago, plus a weekly dependency maintenance cron mode.

Make the 7-day external package age rule visible to all ongoing agent work through the root `AGENTS.md` instructions.

## Non-goals

- Do not age-gate transitive Go modules in `go.sum`.
- Do not add pnpm or uv project manifests where the repo does not already use them.
- Do not refactor unrelated skills, memory hooks, or agent sync behavior.

## User story / behavior

- `dotagents doctor` includes a package-age check for external package references used by dotagents config and skill docs.
- `dotagents dogfood` runs the same check through doctor.
- Users can disable the package-age check with `--skip-package-age`.
- `dotagents cron --deps --interval weekly` installs a weekly dependency maintenance cron entry.
- `AGENTS.md` tells agents to verify that new external package/tool references are at least 7 days old.

## Acceptance tests

- Unit tests cover package reference parsing, fresh-package failure, old-package pass, registry outage behavior, and skip flag behavior.
- End-to-end tests cover cron entry rendering for weekly dependency updates without modifying the real crontab.
- `go test ./cmd/dotagents` passes.
- `dotagents doctor` and `dotagents dogfood` are run locally; VPS verification is attempted if a reachable VPS target is available.

## Constraints

- Use Go for dotagents CLI changes.
- Keep changes surgical and repo-owned.
- Network registry failures fail the package-age check unless `--skip-package-age` is set.

## Dependencies / integrations

- PyPI JSON API for `uvx` / `uv tool install` package references.
- npm registry JSON API for npm/pnpm package references.
- Crontab remains the local scheduler integration.

## Risks / open questions

- The scope uses the recommended policy: external install/package references only, not transitive Go modules.
