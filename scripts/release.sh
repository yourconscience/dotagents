#!/usr/bin/env sh
# Cut a dotagents release: verify, tag, push. CI does the rest.
#
# Usage: scripts/release.sh v0.7.0
#
# Pushing a v* tag triggers .github/workflows/release.yml, which:
#   1. runs goreleaser  -> binaries + checksums + GitHub release + homebrew tap bump
#   2. publishes the npm wrapper (dotagents) with provenance
TAG="${1:-}"
YES="${2:-}"

[ -f .goreleaser.yaml ] || { echo "run from the dotagents repo root"; exit 1; }
case "$TAG" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "usage: scripts/release.sh v0.7.0 [--yes]"; exit 1 ;; esac
confirm() {
	[ "$YES" = "--yes" ] && return 0
	printf '%s [y/N] ' "$1"
	read -r answer
	[ "$answer" = y ]
}
case "$TAG" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "tag must look like v0.7.0"; exit 1 ;; esac

branch=$(git rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || { echo "must be on main (on $branch)"; exit 1; }
[ -z "$(git status --porcelain | grep -v '^??')" ] || { echo "working tree has uncommitted changes"; exit 1; }
git fetch origin --tags -q
[ "$(git rev-parse main)" = "$(git rev-parse origin/main)" ] || { echo "main not in sync with origin/main"; exit 1; }
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null && { echo "$TAG already exists"; exit 1; }

echo ">> go test ./..."
go test ./...

prev=$(git describe --tags --abbrev=0)
echo ">> changes since $prev:"
git log --oneline "$prev"..main | sed 's/^/   /'

confirm "release $TAG? [y/N] " || { echo "aborted"; exit 1; }
git tag -a "$TAG" -m "$TAG"
git push origin "$TAG"

echo ">> tag pushed. watch: gh run watch --interval 30 \$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')"
echo ">> after CI: brew info dotagents && npm view dotagents version"
