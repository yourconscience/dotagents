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

Primary: use `x-cli` for X.com searches. It uses X's internal GraphQL API with browser-based auth, providing reliable and structured results.

Quick check:

```bash
x-cli auth status
```

If not authenticated, run `x-cli auth login` to open a Chrome window for manual login.

Search for posts from these accounts about the topic. Group into batches of 3-4 accounts per query to stay efficient.

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

**Search pattern** - run these queries via x-cli:

```
(from:banteg OR from:thorstenball OR from:thdxr) <topic>
(from:thsottiaux OR from:steipete OR from:trq212) <topic>
(from:ericzakariasson OR from:hardmaru OR from:karpathy OR from:fchollet) <topic>
```

If results are thin, broaden with:
```
<topic> (recommended OR "game changer" OR "switched to" OR "moved to" OR "best" OR "opinion")
```

**x-cli usage:**

```bash
# Latest tweets (fresh opinions)
x-cli search "(from:karpathy OR from:fchollet) <topic>" --type latest --count 10

# Top/relevant tweets (higher signal)
x-cli search "<topic>" --type top --count 10

# JSON output for parsing
x-cli search "<query>" --type latest --count 10 --json
```

Guidance:
- Start with `--type latest` for fresh opinions. Retry with `--type top` if the result set is noisy.
- x-cli stores session credentials at `~/.x-cli/credentials.json`.
- If auth expires, re-run `x-cli auth login`.
- Use `--json` when you need structured output for downstream parsing.
- The output includes engagement metrics (retweets, likes, replies, views) for quality assessment.

Fallback: harness web search with `site:x.com`.

```text
site:x.com <topic> (recommended OR "game changer" OR "switched to" OR "moved to" OR "best" OR "opinion")
```

Use this only if x-cli auth is broken and cannot be refreshed.

### 2. Hacker News

Primary: use the Algolia HN Search API for topic search. It is the best low-friction search interface for HN.

Search:

```text
https://hn.algolia.com/api/v1/search?query=<topic>&tags=story&hitsPerPage=10
https://hn.algolia.com/api/v1/search_by_date?query=<topic>&tags=story&hitsPerPage=10
```

Guidance:
- Use `search` first for higher-signal results.
- Use `search_by_date` if recency matters or the first search looks stale.
- Prefer stories with meaningful discussion volume.
- Extract `title`, `points`, `num_comments`, `created_at`, `url`, and `objectID`.

Read threads:

```text
https://news.ycombinator.com/item?id=<objectID>
```

- For the best candidate stories, fetch the actual HN thread page to extract top comments and overall sentiment.

Fallback: official HN Firebase API.

- Use this when Algolia looks stale, misses a very recent post, or you already know an exact item ID.
- Firebase is not a practical primary search API for topical queries.

```text
https://hacker-news.firebaseio.com/v0/topstories.json
https://hacker-news.firebaseio.com/v0/newstories.json
https://hacker-news.firebaseio.com/v0/askstories.json
https://hacker-news.firebaseio.com/v0/showstories.json
https://hacker-news.firebaseio.com/v0/item/<id>.json
```

Browser fallback:

```text
https://news.ycombinator.com/news
https://news.ycombinator.com/newest
https://news.ycombinator.com/ask
https://news.ycombinator.com/show
```

### 3. Reddit - targeted subreddits

Primary: use Reddit JSON endpoints directly. This is the lowest-friction default and avoids dependence on third-party archives.

Search:

```text
https://www.reddit.com/r/<subreddit>/search.json?q=<topic>&restrict_sr=1&sort=relevance&t=year&limit=10
https://www.reddit.com/search.json?q=<topic>%20subreddit:<subreddit>&sort=relevance&t=year&limit=10
```

Read threads:

```text
https://www.reddit.com<permalink>.json
```

Important:
- Prefer `.json` endpoints over HTML Reddit pages.
- Reddit HTML search pages may trigger verification or JS interstitials.
- Restrict to 2-3 relevant subreddits instead of searching everything.

Fallback 1: Arctic Shift.

- Use Arctic Shift when Reddit search results are thin, when you want broader historical coverage, or when you need comment-level search.
- Use the documented `/search` endpoints.

```text
https://arctic-shift.photon-reddit.com/api/posts/search?subreddit=ClaudeAI,codex,claudecode&query=<topic>&limit=10&sort=desc&after=6months
https://arctic-shift.photon-reddit.com/api/posts/search?subreddit=ExperiencedDevs,devops,commandline&query=<topic>&limit=10&sort=desc&after=6months
https://arctic-shift.photon-reddit.com/api/comments/search?subreddit=ClaudeAI,codex&body=<topic>&limit=10&sort=desc&after=6months
https://arctic-shift.photon-reddit.com/api/comments/tree?link_id=t3_<post_id>&limit=200
```

Arctic Shift caveats:
- No uptime or performance guarantees.
- Some queries may time out.
- Very recent post scores and comment counts can lag.

Fallback 2: harness web search with `site:reddit.com`.

```text
site:reddit.com/r/ExperiencedDevs <topic>
site:reddit.com/r/codex <topic>
site:reddit.com/r/claudecode <topic>
site:reddit.com/r/devops <topic>
site:reddit.com/r/MachineLearning <topic>
site:reddit.com/r/LocalLLaMA <topic>
site:reddit.com/r/neovim <topic>
site:reddit.com/r/commandline <topic>
site:reddit.com/r/selfhosted <topic>
site:reddit.com/r/ClaudeAI <topic>
site:reddit.com/r/ChatGPTPro <topic>
```

- Use the harness search tool if available.
- Do not fetch Google HTML pages directly.
- Do not rely on PullPush as a standard fallback unless you verify a live endpoint first in the current environment.

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
- Run searches in parallel for speed: API queries, helper calls, or WebSearch depending on source.
- For X.com results that need more context, prefer the helper payload first. Use WebFetch only as a best-effort fallback because generic fetchers may fail on X pages.
- Cap total search calls at about 12-15 to avoid being slow. Prioritize breadth over depth.
- End with a Confidence indicator so the user knows how much signal was found.

## GitHub Repo Triage

Use this when the user is comparing or vetting candidate GitHub repos to try or read next. Do not use it for general opinion pulse work. This mode is a best-effort heuristic filter, not an audit, not a score, and not a dependency or security analyzer.

### When to invoke

- The user has a topic and wants a shortlist of repos worth checking next.
- The user already has a candidate repo list and wants issue-surface triage.
- The user wants a handoff from discovery into repo-by-repo vetting.

### Modes

1. Topic -> shortlist

- Use this for discovery only. It keeps the GitHub search relevance order and fetches metadata only.
- Run from the tool directory:

```bash
cd ~/Workspace/dotagents/skills/techsearch/tools/repo_triage
go run . --topic "rust terminal multiplexer"
```

2. Repos list -> triage

- Use this when the user already has concrete candidates.
- Full mode fetches metadata, release/default-branch/security signals, and top open issues by reaction count.

```bash
cd ~/Workspace/dotagents/skills/techsearch/tools/repo_triage
go run . --repos tmux/tmux,zellij-org/zellij,ghostty-org/ghostty
```

- Use `--light` when you only want a fast metadata pass and will inspect issues later.

```bash
cd ~/Workspace/dotagents/skills/techsearch/tools/repo_triage
go run . --repos tmux/tmux,zellij-org/zellij --light
```

3. Handoff hint

- After topic mode, give the user or next agent a concrete follow-up command with the strongest candidates:

```bash
cd ~/Workspace/dotagents/skills/techsearch/tools/repo_triage
go run . --repos owner/a,owner/b,owner/c
```

- If the user wants a reusable binary:

```bash
cd ~/Workspace/dotagents/skills/techsearch/tools/repo_triage
go build -o repo_triage .
./repo_triage --repos owner/a,owner/b
```

### Output JSON

The tool prints one JSON object to stdout:

- `mode`: `shortlist` or `triage`
- `query`: the topic string or normalized repo slug list
- `repos[]`: per-repo signals
- `errors[]`: per-repo fetch failures without failing the whole run

Key repo fields:

- `last_push`, `last_commit_date`, `last_release`: maintenance freshness
- `commit_last_30d`, `commit_last_60d`, `commit_last_180d`: soft activity flags
- `top_issues[]`: highest-reaction open issues with title, reactions, age, URL, and short snippet
- `friction_keywords[]`: matched issue-title smells such as `install`, `setup`, `crash`, `data loss`, `m1`
- `security_policy`: whether a `SECURITY.md` policy was found in common GitHub locations
- `vibe` and `vibe_reason`: soft label only, meant to help sorting, not to replace judgment

Interpretation guidance:

- `worth-trying`: active and comparatively clean top-issue surface
- `maybe`: some healthy signals, but notable friction or weaker maintenance evidence
- `avoid-for-now`: archived, stale, or too many strong unresolved complaints
- `mixed-signals`: thin or partial data, or metadata-only runs

### Render Instructions

When presenting results to the user:

- Bucket repos into `Worth trying`, `Maybe`, and `Avoid for now`.
- If all outputs are thin or partial, keep a `Mixed signals` bucket instead of forcing confidence.
- Include a compact comparison table with: repo, stars, last push, last release, open issues, friction keywords, vibe.
- Follow with short per-repo notes:
  - 1 sentence on maintenance posture
  - 1 sentence on the top issue smells or absence of them
  - 1 direct link to the most useful repo or issue entry
- Surface raw signals first. Do not turn the vibe into a numeric score.
