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

## Worker-first workflow

Use the repo-owned worker before manual source spelunking:

```bash
go run ./skills/tech-search/tools/tech-pulse search "<topic>" --days 30 --format json --raw /tmp/tech-pulse.json
```

Useful commands:

```bash
go run ./skills/tech-search/tools/tech-pulse diagnose
go run ./skills/tech-search/tools/tech-pulse search "<topic>" --sources hn,reddit,x,github --format markdown
go run ./skills/tech-search/tools/tech-pulse search "<topic>" --sources hn,github --days 90 --format json
```

The worker searches sources in parallel, normalizes records, strips tracking parameters, deduplicates repeated links/stories, ranks by topic overlap plus engagement, and reports per-source availability/errors. Treat its output as the raw evidence layer for synthesis, not as the final answer when the user asks for judgment.

## Query planning

Before running the worker, decide whether the topic is:

- **Specific artifact**: post, repo, paper, release, incident. Structure synthesis around the artifact's claims, then public reaction.
- **Named tool/project/company/person**: include GitHub and X. Add repo/user names when obvious from the topic.
- **Comparison**: search the full comparison and each side separately.
- **Workflow/how-to**: strip literal phrases like "use cases", "workflow", "how to" from search strings; keep them in the ranking question.
- **Ambiguous term**: add context keywords, e.g. `warp terminal coding`, not just `warp`.

If the worker result is thin or skewed, run one narrower follow-up rather than broadening blindly.

## Sources

Search source backends in parallel through `tech-pulse` by default. Manual commands below are fallbacks for deeper inspection.

Reference: `references/reddit-discord-cli-eval.md` records the repo evaluation behind the `rdt-cli` and `discord-cli` recommendations.

### 1. GitHub

GitHub is first-class for developer tools, libraries, agent frameworks, MCP servers, and open source projects. The worker uses GitHub REST search for repositories and recently updated issues/PRs. `GITHUB_TOKEN` or `GH_TOKEN` increases rate limits but is not required.

Manual deepening:

```bash
gh repo view <owner>/<repo> --json nameWithOwner,description,stargazerCount,updatedAt,latestRelease
gh issue list -R <owner>/<repo> --search "<topic>" --state all --limit 20 --json title,url,state,comments,updatedAt
gh pr list -R <owner>/<repo> --search "<topic>" --state all --limit 20 --json title,url,state,comments,updatedAt
```

For people, prefer concrete GitHub usernames over keyword searches when known.

### 2. X.com

Use `x-cli` for X.com searches. Check auth with `x-cli auth status`. The worker skips X when `x-cli` is unavailable.

**Power users:** @karpathy, @fchollet, @hardmaru, @thorstenball, @thdxr, @steipete, @banteg

Manual deepening:

```bash
x-cli search "(from:karpathy OR from:fchollet) <topic>" --type latest --count 10 --json
x-cli search "<topic> (recommended OR \"game changer\")" --type top --count 10 --json
```

Fallback: `site:x.com <topic>` via WebSearch if x-cli auth is broken.

### 3. Hacker News

The worker uses the Algolia `search` endpoint with a date filter. **Do NOT use `search_by_date`** as the primary path - it returns fresh zero-comment noise. The hybrid path gives recent + high-signal results.

Manual query shape:

```text
https://hn.algolia.com/api/v1/search?query=<topic>&tags=story&hitsPerPage=10&numericFilters=created_at_i>TIMESTAMP
```

Read threads at `https://news.ycombinator.com/item?id=<objectID>`.

### 4. Reddit

Preferred path is `rdt-cli` when installed. The worker tries `rdt search ... --compact --json`, then falls back to Reddit RSS. Raw Reddit `.json` endpoints often 403 and should be treated as a one-shot fallback only, not the primary plan.

Manual deepening:

```bash
rdt search "<topic>" -s relevance -t month -n 10 --compact --json
rdt search "<topic>" -r <subreddit> -s top -t year -n 10 --compact --json
rdt read <post_id> -n 20 --json
```

**Target subreddits:** r/ExperiencedDevs, r/ClaudeAI, r/LocalLLaMA, r/MachineLearning, r/devops, r/commandline, r/neovim

Pick 2-3 relevant to the topic. Add context keywords for ambiguous terms. On VPS/headless hosts, `rdt search` may return Reddit `forbidden` without browser cookies; do not copy cookie secrets into chat. For historical posts or VPS fallback, use Pullpush API (`https://api.pullpush.io/reddit/search/submission/?q=<topic>&size=5&sort=desc&sort_type=score`).

### 5. Discord

Discord remains opt-in because it uses user-token auth and may carry account-risk. Search Discord only when the topic is relevant to known communities (ML, LLMs, Claude, agents, evals, fine-tuning, etc). Skip for generic/unrelated topics.

**Preferred CLI for repeated/community monitoring**: `discord-cli` (`uv tool install kabi-discord-cli`) can sync accessible Discord channels into local SQLite, then search/export them with structured YAML/JSON. Use it only for accounts the user controls. Do not ask the user to paste raw Discord tokens into chat logs.

Good fit:

```bash
discord status --yaml
discord dc guilds --yaml
discord dc channels <guild_id> --yaml
discord dc search <guild_id> "<topic>" -n 10 --json
discord search "<topic>" -n 20 --json
```

**Known servers:**

| Name | Guild ID |
|---|---|
| NousResearch | 1053877538025386074 |
| Anthropic/Claude | 1456350064065904867 |

For one-off raw API search, use `rtk proxy curl` to bypass token filtering, URL-encode the query string, and extract only `messages[][]` entries where `"hit": true`.

### 6. Tech blogs and web

For authoritative context: `site:simonwillison.net`, `site:jvns.ca`, `site:danluu.com`, vendor docs, release notes, and primary announcement posts via WebSearch. Use this to supplement the worker, not to replace source-specific searches.

## Search, dedupe, and ranking rules

- Save or preserve raw worker JSON for non-trivial reports.
- Deduplicate by canonical URL first; strip `utm_*`, `ref`, `source`, fragments, and Reddit comment slugs.
- Deduplicate cross-platform amplification: one announcement reposted on HN, Reddit, and X is one story with multiple reactions, not three independent facts.
- Cap any single author/domain/community from dominating unless the topic is about that author/domain/community.
- Prefer recent results for pulse questions (`--days 30`); use `--days 90` for slow-moving developer tools.
- Prefer engagement only after topic relevance. A high-upvote irrelevant Reddit thread is noise.
- Keep source-specific failures visible. Explicitly state when X, Discord, GitHub auth, or Reddit cookies were unavailable.
- Never fabricate quotes, counts, or links.

## Output format

```markdown
## <Topic> - Tech Pulse

### Source status
- GitHub: searched / skipped because ...
- X.com: searched / skipped because ...
- Hacker News: searched
- Reddit: searched
- Discord: skipped unless relevant/authenticated

### Key findings
- **Finding** - evidence and direct links.

### Source notes
- **GitHub**: repo/issues/PR signal.
- **X.com**: notable expert posts or lack of signal.
- **Hacker News**: high-signal threads.
- **Reddit**: subreddit/user pain or adoption signal.
- **Discord**: only when searched.

### Summary
2-3 sentence synthesis. Key recommendations if any.

### Confidence
[High/Medium/Low] based on source coverage, volume, recency, and agreement.
```

If combined with `/repo-eval` or if the user requests a `.md` report, produce a Markdown report under `~/Workspace/reports/` and verify it exists before finalizing. Preserve source-specific sections, include direct thread/post links, distinguish amplification from substantive critique, and explicitly state when a source was skipped because credentials such as `DISCORD_TOKEN` were unavailable.

