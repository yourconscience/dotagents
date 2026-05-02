---
name: tech-search
description: Search web, specifically Hacker News, X.com, and Reddit from top tech bloggers and communities about a given topic. Use when user says /tech-search or wants curated tech opinions on a topic.
---

# tech-search

Search for opinions and discussions from high-signal tech sources about a given topic.

## Usage

```
/tech-search <topic>
```

## Sources

Search all sources in parallel.

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

**Optional CLI**: `rdt-cli` (`uv tool install rdt-cli`) provides structured output with anti-detection. Same author ecosystem as x-cli. Use `rdt search "<topic>" --compact --json` if installed.

Fallback: Pullpush API for historical posts (`https://api.pullpush.io/reddit/search/submission/?q=<topic>&size=5&sort=desc&sort_type=score`).

### 4. Tech blogs (optional)

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

