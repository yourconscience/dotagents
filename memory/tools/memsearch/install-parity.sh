#!/usr/bin/env bash
#
# memsearch parity installer.
#
# Brings any machine to full, equal-citizen memsearch parity with the canonical
# knowledge vault: installs the engine with local ONNX embeddings, writes the
# canonical config (provider=onnx, collection=ai), and builds the derived index
# from the vault markdown.
#
# Markdown is the ONLY source of truth. The per-machine index (~/.memsearch/
# milvus.db) is a disposable derivative -- it is never synced and never
# canonical, so every machine builds its own from the shared markdown.
#
# Idempotent: re-running installs only what is missing, re-asserts config, and
# refreshes the index in place. Use --rebuild for a full rebuild from zero.
#
# Usage:
#   install-parity.sh [--rebuild] [--no-verify]
#
# Environment:
#   KNOWLEDGE_DIR         vault root         (default: ~/Workspace/knowledge)
#   MEMSEARCH_COLLECTION  collection name    (default: ai)
#   MEMSEARCH_HOME        engine state dir   (default: ~/.memsearch)

set -euo pipefail

KNOWLEDGE_DIR="${KNOWLEDGE_DIR:-$HOME/Workspace/knowledge}"
COLLECTION="${MEMSEARCH_COLLECTION:-ai}"
MEMSEARCH_HOME="${MEMSEARCH_HOME:-$HOME/.memsearch}"

rebuild=0
verify=1

usage() {
	sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'
}

for arg in "$@"; do
	case "$arg" in
		--rebuild) rebuild=1 ;;
		--no-verify) verify=0 ;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			echo "install-parity: unknown argument: $arg" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [ ! -d "$KNOWLEDGE_DIR" ]; then
	echo "install-parity: vault not found at $KNOWLEDGE_DIR" >&2
	exit 1
fi

# 1. Install memsearch with the onnx extra (local embeddings, no API keys).
if command -v memsearch >/dev/null 2>&1; then
	echo "install-parity: memsearch present ($(memsearch --version))"
else
	if ! command -v uv >/dev/null 2>&1; then
		echo "install-parity: uv not found; install uv first (https://docs.astral.sh/uv/)" >&2
		exit 1
	fi
	echo "install-parity: installing memsearch[onnx] via uv ..."
	uv tool install 'memsearch[onnx]'
fi

# 2. Canonical config: local ONNX provider + shared collection.
memsearch config set embedding.provider onnx
memsearch config set milvus.collection "$COLLECTION"
echo "install-parity: config provider=$(memsearch config get embedding.provider) collection=$(memsearch config get milvus.collection)"

# 3. Build the derived index from the canonical vault markdown.
if [ "$rebuild" -eq 1 ]; then
	echo "install-parity: full rebuild from zero (dropping collection $COLLECTION) ..."
	memsearch reset --collection "$COLLECTION" --yes || rm -f "$MEMSEARCH_HOME/milvus.db"
fi
echo "install-parity: indexing $KNOWLEDGE_DIR into collection $COLLECTION ..."
memsearch index "$KNOWLEDGE_DIR" --collection "$COLLECTION"

# 4. Verify a real search returns results.
if [ "$verify" -eq 1 ]; then
	echo "install-parity: verifying ..."
	memsearch stats --collection "$COLLECTION"
	memsearch search "knowledge vault memory" --collection "$COLLECTION" -k 1
fi

echo "install-parity: done"
