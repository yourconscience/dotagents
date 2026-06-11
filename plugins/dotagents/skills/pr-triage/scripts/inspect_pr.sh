#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ge 1 ]]; then
  PR_ARG="$1"
  PR_META="$(gh pr view "$PR_ARG" --json number,url --jq '(.number | tostring) + " " + .url')"
else
  PR_META="$(gh pr view --json number,url --jq '(.number | tostring) + " " + .url')"
fi
read -r PR PR_URL <<<"$PR_META"
if [[ "$PR_URL" =~ github.com/([^/]+)/([^/]+)/pull/[0-9]+ ]]; then
  OWNER="${BASH_REMATCH[1]}"
  REPO="${BASH_REMATCH[2]}"
else
  echo "Error: could not resolve repository from PR URL: ${PR_URL}" >&2
  exit 1
fi

QUERY='query($owner:String!, $repo:String!, $number:Int!){ repository(owner:$owner, name:$repo){ pullRequest(number:$number){ reviewThreads(first:100){ nodes { id isResolved isOutdated path line comments(first:20){ nodes { author{ login } body url createdAt } } } } } } }'

echo "PR #${PR} (${OWNER}/${REPO})"

echo
echo "=== CHECKS ==="
set +e
gh pr checks "$PR"
checks_status=$?
set -e
if [[ "$checks_status" -ne 0 && "$checks_status" -ne 8 ]]; then
  exit "$checks_status"
fi

echo
echo "=== FAILED CHECKS ==="
gh pr view "$PR" --json statusCheckRollup --jq '.statusCheckRollup[]
  | select(.status == "COMPLETED")
  | select(.conclusion != "SUCCESS" and .conclusion != "NEUTRAL" and .conclusion != "SKIPPED")
  | "\(.name)\t\(.conclusion)\t\(.detailsUrl)"'

echo
echo "=== UNRESOLVED THREADS ==="
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query="$QUERY" \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | "- [\(.comments.nodes[0].author.login)] \(.path):\(.line // 0)\n  \(.comments.nodes[0].url)\n  \(.comments.nodes[0].body | gsub("\n"; " ") | .[0:260])"'

echo
echo "=== BOT THREADS (auto-resolvable) ==="
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query="$QUERY" \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | select(.comments.nodes[0].author.login | test("^(chatgpt-codex|gemini|copilot|cursor|claude|codex|coderabbitai)"; "i"))
    | "- [\(.comments.nodes[0].author.login)] \(.path):\(.line // 0)\n  \(.comments.nodes[0].url)"'

echo
echo "=== HUMAN THREADS (do NOT auto-resolve) ==="
gh api graphql \
  -F owner="$OWNER" -F repo="$REPO" -F number="$PR" \
  -f query="$QUERY" \
  --jq '.data.repository.pullRequest.reviewThreads.nodes[]
    | select(.isResolved==false and .isOutdated==false)
    | select((.comments.nodes[0].author.login | test("^(chatgpt-codex|gemini|copilot|cursor|claude|codex|coderabbitai)"; "i")) | not)
    | "- [\(.comments.nodes[0].author.login)] \(.path):\(.line // 0)\n  \(.comments.nodes[0].url)"'
