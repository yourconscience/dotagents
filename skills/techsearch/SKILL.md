---
name: techsearch
description: Search web, specifically Hacker News, X.com, and Reddit from top tech bloggers and communities about a given topic. Use when user says /techsearch or wants curated tech opinions on a topic.
---

# techsearch

Search for opinions and discussions from high-signal tech sources about a given topic.

## Usage

```
/techsearch <topic>
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

Use Algolia API:
```
https://hn.algolia.com/api/v1/search?query=<topic>&tags=story&hitsPerPage=10
```

For recency: use `search_by_date` instead. Read threads at `https://news.ycombinator.com/item?id=<objectID>`.

### 3. Reddit

Use JSON endpoints directly:
```
https://www.reddit.com/r/<subreddit>/search.json?q=<topic>&restrict_sr=1&sort=relevance&t=year&limit=10
```

**Target subreddits:** r/ExperiencedDevs, r/ClaudeAI, r/LocalLLaMA, r/MachineLearning, r/devops, r/commandline, r/neovim

Pick 2-3 relevant to the topic. Fallback: Arctic Shift API or `site:reddit.com/r/<sub>` via WebSearch.

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

## GitHub Repo Triage

For comparing/vetting GitHub repos. Use the repo_triage tool:

```bash
# Topic discovery
go run ~/.agents/skills/techsearch/tools/repo_triage --topic "rust terminal multiplexer"

# Specific repos
go run ~/.agents/skills/techsearch/tools/repo_triage --repos tmux/tmux,zellij-org/zellij
```

Output includes: stars, last push, last release, top issues, friction keywords, vibe label.

Present as buckets: `Worth trying`, `Maybe`, `Avoid for now`.
