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

The `<topic>` is the subject to search for. Examples: "terminal multiplexer SSH", "Claude Code vs Cursor", "TTS normalization".

## Sources

Search ALL three source categories in parallel. Run multiple WebSearch calls concurrently.
Use the harness's available task or subagent delegation mechanism to fan out searches in parallel when possible.

### 1. X.com - power users

Search for posts from these accounts about the topic. Group into batches of 3-4 per WebSearch query to stay efficient.

**Core list:**

| Handle | Known for |
|---|---|
| @banteg | DeFi/infra dev, strong opinions on tooling |
| @thorstenball | PL/editor dev (Zed core team) |
| @thdxr | SST/infra, terminal tooling |
| @thsottiaux | Dev tooling, AI agents |
| @steipete | iOS/macOS veteran, productivity tooling |
| @trq212 | AI/ML engineering |
| @ericzakariasson | Dev tools, workflows |
| @hardmaru | ML research (Google Brain) |
| @karpathy | AI/ML, founder of Eureka Labs |
| @fchollet | Keras creator, AI/ML |

**Search pattern** - run these queries:

```
site:x.com (from:banteg OR from:thorstenball OR from:thdxr) <topic>
site:x.com (from:thsottiaux OR from:steipete OR from:trq212) <topic>
site:x.com (from:ericzakariasson OR from:hardmaru OR from:karpathy OR from:fchollet) <topic>
```

If results are thin, broaden with:
```
site:x.com <topic> (recommended OR "game changer" OR "switched to" OR "moved to" OR "best" OR "opinion")
```

### 2. Hacker News

Run these WebSearch queries:

```
site:news.ycombinator.com <topic>
```

For top results, fetch the HN thread via WebFetch to extract top comments and sentiment. Use the Algolia API when useful:

```
https://hn.algolia.com/api/v1/search?query=<topic>&tags=story&hitsPerPage=5
```

### 3. Reddit - targeted subreddits

Reddit blocks most search engines (Google exclusivity deal). Use a two-pronged approach:

**Primary: Arctic Shift API** (searches all Reddit posts/comments historically)

Use WebFetch to query the Arctic Shift API. Pick 2-3 subreddits most relevant to the topic.

```
https://arctic-shift.photon-reddit.com/api/posts?subreddit=ClaudeAI,codex,claudecode&query=<topic>&limit=10&sort=score&after=<YYYY-MM-DD from ~6 months ago>
https://arctic-shift.photon-reddit.com/api/posts?subreddit=ExperiencedDevs,devops,commandline&query=<topic>&limit=10&sort=score&after=<YYYY-MM-DD from ~6 months ago>
https://arctic-shift.photon-reddit.com/api/comments?subreddit=ClaudeAI,codex&query=<topic>&limit=10&sort=score&after=<YYYY-MM-DD from ~6 months ago>
```

Calculate the `after=` date dynamically to about 6 months ago relative to today. The API returns JSON with `title`, `score`, `num_comments`, `permalink`, `selftext`.

For top-scoring results, fetch comments via:
```
https://arctic-shift.photon-reddit.com/api/comments?link_id=t3_<post_id>&limit=20&sort=score
```

**Fallback: Google site:reddit.com** (still indexed by Google)

```
site:reddit.com (r/ExperiencedDevs OR r/codex OR r/claudecode OR r/devops) <topic>
site:reddit.com (r/MachineLearning OR r/LocalLLaMA OR r/neovim) <topic>
site:reddit.com (r/commandline OR r/selfhosted OR r/ClaudeAI OR r/ChatGPTPro) <topic>
```

**Fallback 2: PullPush API** (if Arctic Shift is down)
```
https://api.pullpush.io/reddit/search/submission/?q=<topic>&subreddit=ClaudeAI,codex&size=10&sort=score&after=1727740800
```

**Target subreddits:**

| Subreddit | Focus |
|---|---|
| r/ExperiencedDevs | Senior engineer perspectives |
| r/codex | OpenAI Codex agent discussions |
| r/claudecode | Claude Code agent discussions |
| r/devops | Infra, deployment, tooling |
| r/MachineLearning | ML/AI research |
| r/LocalLLaMA | Local AI, inference, agents |
| r/neovim | Terminal-first dev workflows |
| r/commandline | CLI tools, terminal apps |
| r/selfhosted | Self-hosted infra |
| r/ClaudeAI | Claude-specific discussions |
| r/ChatGPTPro | AI coding agent discussions |

Pick 2-3 subreddit groups most relevant to the topic. Do not search all if the topic clearly does not apply (e.g., skip r/MachineLearning for a terminal emulator question).

### 4. Tech blogs and documentation (NEW)

For topics that benefit from authoritative sources beyond social media, also search:

```
site:blog.pragmaticengineer.com <topic>
site:martinfowler.com <topic>
site:jvns.ca <topic>
site:simonwillison.net <topic>
site:danluu.com <topic>
```

Pick 1-2 blog searches most relevant to the topic. Skip if the topic is too niche or product-specific.

## Deduplication and quality filter (NEW)

Before presenting results:
- Deduplicate: if the same link or opinion appears across sources, mention it once and note it was echoed across platforms.
- Recency bias: strongly prefer results from the last 6 months. Include older results only if they are seminal or highly upvoted.
- Signal filter: skip results with very low engagement (< 3 upvotes on Reddit, < 2 points on HN) unless they contain substantive technical content.

## Output format

After collecting results, present a structured summary:

```
## <Topic> - Tech Pulse

### X.com
- **@handle**: "quote or paraphrase" (link)
- ...

### Hacker News
- **Title** (N points, N comments) - top sentiment / key takeaway (link)
- ...

### Reddit
- **r/subreddit - Title** (N upvotes) - key takeaway (link)
- ...

### Blogs (if searched)
- **Author - Title** - key takeaway (link)
- ...

### Summary
2-3 sentence synthesis of the overall sentiment / consensus / disagreements.
Key recommendations that emerged, if any.

### Confidence
[High/Medium/Low] - based on volume and agreement of sources found.
```

## Rules

- Always include direct links to sources.
- Prefer recent results (last 6 months) over old ones.
- If a source has no relevant results, say so briefly rather than omitting it.
- Do NOT fabricate quotes or links. If WebSearch returns nothing for a query, report that.
- Run searches in parallel (multiple WebSearch calls at once) for speed.
- For X.com results that look interesting but lack context, use WebFetch to get the full post.
- Cap total WebSearch calls at ~12 to avoid being slow. Prioritize breadth over depth.
- End with a Confidence indicator so the user knows how much signal was found.
