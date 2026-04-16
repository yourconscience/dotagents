#!/usr/bin/env bash
set -euo pipefail

# auto-detect owner/repo from git remote
REMOTE_URL="$(git remote get-url origin)"
OWNER="${2:-$(echo "$REMOTE_URL" | sed -E 's#.+[:/]([^/]+)/([^/.]+)(\.git)?$#\1#')}"
REPO="${3:-$(echo "$REMOTE_URL" | sed -E 's#.+[:/]([^/]+)/([^/.]+)(\.git)?$#\2#')}"
if [[ $# -ge 1 ]]; then
  PR="$1"
else
  PR="$(gh pr view --json number --jq '.number' 2>/dev/null)" || { echo "Error: PR number not provided and not on a PR branch." >&2; exit 1; }
fi

QUERY='query($owner:String!, $repo:String!, $number:Int!){ repository(owner:$owner, name:$repo){ pullRequest(number:$number){ reviewThreads(first:100){ nodes { id isResolved isOutdated path line comments(first:20){ nodes { author{ login } body url createdAt } } } } } } }'

echo "PR #${PR} (${OWNER}/${REPO})"

echo
echo "=== CHECKS ==="
gh pr checks "$PR" || true

echo
echo "=== FAILED CHECKS ==="
gh pr view "$PR" --json statusCheckRollup --jq '.statusCheckRollup[]
  | select(.status == "COMPLETED")
  | select(.conclusion != "SUCCESS" and .conclusion != "NEUTRAL" and .conclusion != "SKIPPED")
  | "\(.name)\t\(.conclusion)\t\(.detailsUrl)"' || true

echo
echo "=== UNRESOLVED THREADS ==="
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query="$QUERY" \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | "- [\(.comments.nodes[0].author.login)] \(.path):\(.line // 0)\n  \(.comments.nodes[0].url)\n  \(.comments.nodes[0].body | gsub("\n"; " ") | .[0:260])"' || true

echo
echo "=== BOT THREADS (auto-resolvable) ==="
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query="$QUERY" \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | select(.comments.nodes[0].author.login | test("^(gemini|copilot|cursor|claude|codex)"; "i"))
    | "- [\(.comments.nodes[0].author.login)] \(.path):\(.line // 0)\n  \(.comments.nodes[0].url)"' || true

echo
echo "=== HUMAN THREADS (do NOT auto-resolve) ==="
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query="$QUERY" \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | select((.comments.nodes[0].author.login | test("^(gemini|copilot|cursor|claude|codex)"; "i")) | not)
    | "- [\(.comments.nodes[0].author.login)] \(.path):\(.line // 0)\n  \(.comments.nodes[0].url)"' || true
