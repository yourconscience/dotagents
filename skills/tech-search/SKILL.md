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

## Default workflow

Start with broad discovery, then deepen only where the source-specific signal matters. Do not build or invoke a repo-owned search engine for this skill.

1. Run WebSearch for the topic, biased toward primary sources, recent discussion, and source-specific pages.
2. Classify the topic and pick targeted follow-ups:
   - **Named feature/tool/project**: primary docs/repo first, then HN/Reddit/X reaction.
   - **Security/reliability topic**: official guidance, OWASP/CVE/vendor research, then practitioner discussion.
   - **Comparison**: search the full comparison and each side separately. Add disambiguating context (`Python package manager`, `macOS window manager`, etc.).
   - **Workflow/how-to**: strip literal phrases like "use cases", "workflow", "how to" from search strings; keep the intent in synthesis.
   - **Ambiguous term**: add domain keywords before searching. Example: `uv poetry Python packaging`, not `uv vs poetry`.
3. Use targeted tools/commands to verify source-specific evidence.
4. Synthesize only after deduping amplification and separating primary facts from community reaction.

The guiding rule: broad web finds the map; source-specific searches verify the terrain.

## Sources

Before searching, run `dotagents sources --compact` to discover available methods. Use the best available method for each source; fall back to the next if it fails at runtime.

Search sources in parallel when practical, but do not force every source. Skipped sources are fine when they are irrelevant, unauthenticated, or low-signal. If a source shows as disabled in `dotagents sources`, do not attempt it.

Reference: `references/reddit-discord-cli-eval.md` records the repo evaluation behind the `rdt-cli` and `discord-cli` recommendations.

### 1. Web and primary sources

Use WebSearch first for:
- official docs and announcements
- vendor/security research
- release notes
- high-signal blog posts
- source-specific discovery (`site:news.ycombinator.com`, `site:reddit.com`, `site:github.com`)

Good query patterns:
```text
<topic> official docs
<topic> GitHub
<topic> site:news.ycombinator.com
<topic> site:reddit.com/r/<subreddit>
<topic> security best practices OWASP CVE
<topic A> vs <topic B> Python packaging
```

For authoritative context: `site:simonwillison.net`, `site:jvns.ca`, `site:danluu.com`, vendor docs, release notes, and primary announcement posts.

### 2. GitHub

GitHub is first-class for developer tools, libraries, agent frameworks, MCP servers, and open source projects. Prefer exact repo/user lookups over keyword search when known.

```bash
gh repo view <owner>/<repo> --json nameWithOwner,description,stargazerCount,forkCount,updatedAt,latestRelease,url
gh issue list -R <owner>/<repo> --search "<topic>" --state all --limit 20 --json title,url,state,comments,updatedAt
gh pr list -R <owner>/<repo> --search "<topic>" --state all --limit 20 --json title,url,state,comments,updatedAt
```

Use GitHub to answer:
- what is the canonical repo?
- is the project active?
- what is shipping or breaking?
- what issues/PRs show real user pain?

### 3. Hacker News

Use Algolia `search` endpoint with a date filter when recency matters. **Do NOT use `search_by_date`** as the primary path - it returns fresh zero-comment noise.

```text
https://hn.algolia.com/api/v1/search?query=<topic>&tags=story&hitsPerPage=10&numericFilters=created_at_i>TIMESTAMP
```

Also use WebSearch for older high-signal HN threads when the topic is slow-moving. A one-year-old 500-comment HN discussion can matter more than a 2-point fresh repost.

Read threads at `https://news.ycombinator.com/item?id=<objectID>` and cite the HN thread when discussing comments or points. If the submitted URL matters, include it separately.

### 4. Reddit

Use the method shown by `dotagents sources reddit`.

**Target subreddits:** r/ExperiencedDevs, r/ClaudeAI, r/ClaudeCode, r/LocalLLaMA, r/MachineLearning, r/devops, r/commandline, r/neovim, r/Python, r/mcp, r/cybersecurity

Pick 2-3 relevant subreddits. Avoid broad Reddit for ambiguous terms; it will chase engagement from irrelevant communities. Add context keywords, e.g. `uv poetry Python packaging`, not `poetry`.

**rdt-cli** (when best method is rdt-cli):
```bash
rdt search "<topic>" -s relevance -t month -n 10 --compact --json
rdt search "<topic>" -r <subreddit> -s top -t year -n 10 --compact --json
rdt read <post_id> -n 20 --json
```

On VPS/headless hosts, `rdt search` may return Reddit `forbidden` without browser cookies; do not copy cookie secrets into chat.

**pullpush** (fallback for historical posts or VPS): `https://api.pullpush.io/reddit/search/submission/?q=<topic>&size=5&sort=desc&sort_type=score`

**websearch** (fallback): `site:reddit.com <topic>` via WebSearch.

### 5. X.com

Use the method shown by `dotagents sources x.com`. Treat X as commentary unless the author is primary to the topic.

**Power users:** @karpathy, @fcholet, @hardmaru, @thorstenball, @thdxr, @steipete, @banteg

**x-cli** (when best method is x-cli):
```bash
x-cli search "(from:karpathy OR from:fchollet) <topic>" --type latest --count 10 --json
x-cli search "<topic> (recommended OR \"game changer\")" --type top --count 10 --json
```

**websearch** (fallback): `site:x.com <topic>` via WebSearch.

### 6. Discord

Discord is disabled by default (ToS risk). If `dotagents sources` shows it disabled, skip entirely. When enabled, search only for topics relevant to known communities (ML, LLMs, Claude, agents, evals, fine-tuning). Skip for generic/unrelated topics.

**discord-cli** (when enabled and available): can sync accessible Discord channels into local SQLite, then search/export with structured YAML/JSON. Do not ask the user to paste raw Discord tokens into chat logs.

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

## Search, dedupe, and ranking rules

- Preserve enough raw evidence in notes or a report path for non-trivial outputs.
- Prefer primary sources before social reaction.
- Deduplicate by canonical URL first; strip `utm_*`, `ref`, `source`, fragments, and Reddit comment slugs.
- Deduplicate cross-platform amplification: one announcement reposted on HN, Reddit, and X is one story with multiple reactions.
- Cap any single author/domain/community from dominating unless the topic is about that author/domain/community.
- Prefer recent results for pulse questions; use older high-signal results for slow-moving tools or foundational comparisons.
- Prefer topic relevance before engagement. A high-upvote irrelevant Reddit thread is noise.
- Keep source-specific failures visible. Explicitly state when X, Discord, GitHub auth, or Reddit cookies were unavailable.
- Never fabricate quotes, counts, or links.

## Output format

```markdown
## <Topic> - Tech Pulse

### Source status
- Web/primary: searched
- GitHub: searched / skipped because ...
- X.com: searched / skipped because ...
- Hacker News: searched / skipped because ...
- Reddit: searched / skipped because ...
- Discord: skipped unless relevant/authenticated

### Key findings
- **Finding** - evidence and direct links.

### Source notes
- **Primary sources**: docs, repos, release notes, vendor/security research.
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

