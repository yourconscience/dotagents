---
name: tech-search
description: Search web, specifically Hacker News, X.com, Reddit, and Discord from top tech bloggers and communities about a given topic. Use when user says /tech-search or wants curated tech opinions on a topic.
---

# tech-search

Search for opinions and discussions from high-signal tech sources about a given topic.

## Usage

```
/tech-search <topic>
```

## Sources

Search all sources in parallel.

Reference: `references/reddit-discord-cli-eval.md` records the repo evaluation behind the `rdt-cli` and `discord-cli` recommendations.

### 1. X.com

Use `x-cli` for X.com searches. Check auth with `x-cli auth status`.


**Power users:** @karpathy, @fchollet, @hardmaru, @thorstenball, @thdxr, @steipete, @banteg

**Queries:**
```bash
x-cli search "(from:karpathy OR from:fchollet) <topic>" --type latest --count 10
x-cli search "<topic> (recommended OR \"game changer\")" --type top --count 10
```

Fallback: `site:x.com <topic>` via WebSearch if x-cli auth is broken.

### 2. Hacker News

Algolia API. **Critical: do NOT use `search_by_date`** — it returns only zero-comment noise. Use the hybrid approach instead: `search` endpoint (popularity-ranked) with a date filter.

**Primary query (last month, high signal):**
```
https://hn.algolia.com/api/v1/search?query=<topic>&tags=story&hitsPerPage=10&numericFilters=created_at_i>TIMESTAMP
```

Generate timestamp before querying:
- macOS: `date -v-1m +%s`
- Linux: `date -d '1 month ago' +%s`

For fast-moving topics (last week): `date -v-1w +%s` (macOS) or `date -d '1 week ago' +%s` (Linux).

Read threads at `https://news.ycombinator.com/item?id=<objectID>`.

**Pitfall:** The bare `search` endpoint (no `numericFilters`) returns all-time popular stories, often months old. `search_by_date` returns only fresh posts with no votes/comments. Only the hybrid query gives recent + high-signal results.

### 3. Reddit

**IMPORTANT**: Reddit 403s requests without a browser User-Agent. Always set the header.

Global search (preferred - catches cross-subreddit discussion):
```bash
curl -s -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36" \
  "https://www.reddit.com/search.json?q=<topic>&sort=relevance&t=month&limit=10"
```

For broader/deeper dives use `t=year`. For breaking news use `t=week`.

Subreddit-scoped search (when topic maps to a known community):
```bash
curl -s -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36" \
  "https://www.reddit.com/r/<subreddit>/search.json?q=<topic>&restrict_sr=1&sort=relevance&t=year&limit=10"
```

Read full comment threads for high-signal posts:
```bash
curl -s -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36" \
  "https://www.reddit.com/r/<sub>/comments/<id>.json"
```

**Target subreddits:** r/ExperiencedDevs, r/ClaudeAI, r/LocalLLaMA, r/MachineLearning, r/devops, r/commandline, r/neovim

Pick 2-3 relevant to the topic. Add context keywords for ambiguous terms (e.g. "warp terminal coding" not just "warp").

**Preferred CLI when installed**: `rdt-cli` (`uv tool install rdt-cli`) provides structured output, compact agent-friendly results, browser-cookie auth when needed, and anti-detection/backoff. It is a good replacement for raw Reddit JSON in `/tech-search` because it handles subreddit search, global search, compact output, and post/comment reads consistently.

Use:
```bash
rdt search "<topic>" -s relevance -t month -n 10 --compact --json
rdt search "<topic>" -r <subreddit> -s top -t year -n 10 --compact --json
rdt read <post_id> -n 20 --json
```

If `rdt` is not installed or fails, fall back to direct Reddit JSON with browser User-Agent. On VPS/headless hosts, `rdt search` may return Reddit `forbidden` without browser cookies; do not copy cookie secrets into chat. For historical posts or VPS fallback, use Pullpush API (`https://api.pullpush.io/reddit/search/submission/?q=<topic>&size=5&sort=desc&sort_type=score`).

### 4. Discord

Search Discord servers via the user search API. Requires `$DISCORD_TOKEN` env var.

**Preferred CLI for repeated/community monitoring**: `discord-cli` (`uv tool install kabi-discord-cli`) can sync accessible Discord channels into local SQLite, then search/export them with structured YAML/JSON. Use it only for accounts the user controls: it uses a Discord user token and may violate Discord ToS or trigger account restrictions. Do not ask the user to paste raw tokens into chat logs.

Good fit:
```bash
discord status --yaml
discord dc guilds --yaml
discord dc channels <guild_id> --yaml
discord dc search <guild_id> "<topic>" -n 10 --json      # native Discord search
discord search "<topic>" -n 20 --json                    # local SQLite after sync
```

For one-off searches, keep the raw Discord API path below because it is simpler and avoids requiring a local SQLite sync first.

**Known servers:**

| Name | Guild ID |
|---|---|
| NousResearch | 1053877538025386074 |
| Anthropic/Claude | 1456350064065904867 |

Only search Discord when the topic is relevant to these communities (ML, LLMs, Claude, agents, evals, fine-tuning, etc). Skip for generic/unrelated topics.

**Query:**
```bash
curl -s \
  -H "Authorization: $DISCORD_TOKEN" \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) discord/0.0.309 Chrome/124.0.6367.243 Electron/30.4.0 Safari/537.36" \
  "https://discord.com/api/v9/guilds/<GUILD_ID>/messages/search?content=<QUERY>&limit=10&sort_by=timestamp&sort_order=desc"
```

Use `rtk proxy curl` to bypass token filtering. URL-encode the query string.

**Response parsing:** `messages` is a nested array. Each inner array is a context group; the message with `"hit": true` is the actual match. Extract hits with jq:
```bash
| jq '[.messages[][] | select(.hit == true) | {author: .author.username, content: .content[0:200], timestamp: .timestamp, channel_id: .channel_id}]'
```

**If HTTP 202:** index not ready. Retry after `retry_after` seconds (usually 2s).

**Rate limits:** conservative - one search per server per invocation. No pagination unless explicitly needed.

### 5. Tech blogs (optional)

For authoritative sources: `site:simonwillison.net`, `site:jvns.ca`, `site:danluu.com` via WebSearch.

## Output format

```
## <Topic> - Tech Pulse

### X.com
- **@handle**: "quote" (link)

### Hacker News
- **Title** (N points, N comments) - key takeaway (link)

### Reddit
- **r/subreddit - Title** (N upvotes) - key takeaway (link)

### Discord
- **#channel @author** (server, date): "quote" - key takeaway

### Summary
2-3 sentence synthesis. Key recommendations if any.

### Confidence
[High/Medium/Low] based on volume and agreement.
```

## Rules

- Always include direct links. Never fabricate quotes or links.
- Prefer recent results (last 6 months).
- Run searches in parallel. Cap at ~12 total calls.
- Deduplicate cross-platform mentions.

