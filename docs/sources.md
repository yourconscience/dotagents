# External Data Sources Registry

Centralized reference for every external data source accessible to dotagents skills, MCPs, and CLIs.
Each source entry covers: what it is, how we access it, auth requirements, which skills use it, known limitations, and gaps.

## Runtime status

`dotagents doctor` is the canonical health check and includes source availability. The source-specific output below remains a compatibility interface for skills that need method selection:

```bash
dotagents sources             # compatibility: availability table
dotagents sources --compact   # compatibility: one-line skill input
dotagents sources --json      # compatibility: structured output
dotagents sources x.com       # compatibility: one source with setup details
```

Configure in `dotagents.yaml` under `sources:`. Per-machine overrides in `dotagents.local.yaml`.

## Design goals

1. One place to check "can I get data from X?" before writing a new skill or wiring a new MCP.
2. Each source has a canonical access method with a priority. The compatibility `dotagents sources` interface reports the best available method.
3. Auth credentials live in known locations; never duplicated across tools or pasted into chat.
4. Read-only by default. Write/mutate actions require explicit opt-in per source.
5. Configurable per machine: set `preferred: x-api-v2` in `dotagents.local.yaml` to override method selection.

## Access method taxonomy

| Method | Examples | Tradeoffs |
|---|---|---|
| **CLI tool** | `gh`, `x-cli`, `rdt-cli`, `tg`, `gws` | Best for agents; scriptable; output parseable. Preferred when available. |
| **MCP server** | linkedin-scraper-mcp, telegram-readonly, tavily | Native tool-use in agent context; higher setup cost; session lifecycle to manage. |
| **Public API** | HN Algolia, Greenhouse boards, HF Hub | No auth, no breakage risk from session rotation. Limited to public data. |
| **Web search** | Tavily MCP, native WebSearch, WebFetch | Fallback for anything; low precision; can't access authenticated content. |
| **Browser automation** | Playwright (LinkedIn MCP), x-cli login | Fragile; session/cookie rotation; ToS risk. Last resort. |

---

## Source catalog

### X.com / Twitter

| | |
|---|---|
| Canonical access | `x-cli` CLI (Gladium-AI/x-cli) |
| Protocol | X internal GraphQL API via captured browser session |
| Auth | Browser cookies + bearer/CSRF in `~/.x-cli/credentials.json`. Login: `x-cli login` (requires Chrome). |
| Skills | x-cli, x-sim, tech-search, repo-eval |
| Read | timelines, tweets, users, search, followers/following |
| Write | None wired. x-sim is offline simulation only. |
| Fallback | `site:x.com <query>` via WebSearch |
| Limitations | GraphQL query ID drift breaks all commands until manual refresh. Rate-limited. No official API v2 wired (API key exists in research/API_KEYS.md but returns Unauthorized -- likely needs OAuth consumer flow). |
| Gaps | Official API v2 integration would eliminate endpoint drift risk. |

### Hacker News

| | |
|---|---|
| Canonical access | Algolia HN Search API (public, no auth) |
| Protocol | `GET https://hn.algolia.com/api/v1/search?query=<q>&tags=story&hitsPerPage=N` |
| Auth | None |
| Skills | tech-search, repo-eval |
| Read | Story search by relevance. Thread bodies via WebFetch on `news.ycombinator.com/item?id=<id>`. |
| Write | N/A |
| Fallback | `site:news.ycombinator.com` via WebSearch for older threads |
| Limitations | `search_by_date` endpoint returns zero-comment noise; disallowed in tech-search. |
| Gaps | None significant. Stable public API. |

### Reddit

| | |
|---|---|
| Canonical access | `rdt-cli` (Python, `uv tool install rdt-cli`) |
| Protocol | Reddit browser cookies; `rdt search`, `rdt read` |
| Auth | Browser cookies. VPS/headless = 403 without cookies. |
| Skills | tech-search |
| Read | Subreddit search, thread reading. Target subs: r/ExperiencedDevs, r/ClaudeAI, r/ClaudeCode, r/LocalLLaMA, r/MachineLearning, r/devops, r/commandline, r/neovim, r/Python, r/mcp, r/cybersecurity |
| Write | None |
| Fallback 1 | Pullpush API: `https://api.pullpush.io/reddit/search/submission/?q=<q>&size=5&sort=desc&sort_type=score` (historical only, no auth) |
| Fallback 2 | `site:reddit.com <query>` via WebSearch |
| Limitations | Cookie-dependent; headless forbidden. Pullpush is historical, not real-time. |
| Gaps | No official Reddit API integration (requires app registration + OAuth2). Would solve headless access. |

### Discord

| | |
|---|---|
| Canonical access | `discord-cli` (Python, `uv tool install kabi-discord-cli`) |
| Protocol | Discord user-token API. Local SQLite sync for channels. |
| Auth | `DISCORD_TOKEN` env var (user token, not bot). CLI can scan local browser/Discord storage. |
| Skills | tech-search (opt-in only, ML/LLM/agents/evals topics) |
| Read | Channel search, message reading. Known guilds: NousResearch (`1053877538025386074`), Anthropic/Claude (`1456350064065904867`). |
| Write | None |
| Fallback | `rtk proxy curl` direct Discord API (same token) |
| Limitations | User-token usage may violate Discord ToS. Account risk noted. Opt-in only. Local search requires prior sync. |
| Gaps | Bot-token access would be ToS-safe but requires server admin approval. |

### GitHub

| | |
|---|---|
| Canonical access | `gh` CLI + `gh api graphql` |
| Protocol | GitHub REST + GraphQL via OAuth token |
| Auth | `gh auth login` (OAuth browser flow) |
| Skills | tech-search, repo-eval, pr-triage, jobs (career page detection) |
| Read | Repos, issues, PRs, releases, commits, check runs, languages, contributor stats. Clone to `~/Public/<repo>` for code analysis. |
| Write | PR comments via pr-triage (draft-then-review by default; direct replies to bot comments allowed). |
| Limitations | None significant. Always-available. |
| Gaps | None. |

### LinkedIn

| | |
|---|---|
| Canonical access | `linkedin-scraper-mcp` MCP server (Playwright browser automation) |
| Protocol | Headless Chrome via Playwright. No official LinkedIn API. |
| Auth | One-time browser login: `uvx linkedin-scraper-mcp --login`. Session managed by Playwright. |
| Skills | jobs |
| MCP tools | get_inbox, search_conversations, get_conversation, get_person_profile, get_company_profile, get_company_employees, get_job_details, search_people, search_jobs, search_companies |
| Read | DMs, profiles, job postings, company data |
| Write | connect_with_person, send_message exist but are NOT used autonomously. Jobs skill: "never send messages or connection requests unless user explicitly asks." |
| Config | dotagents.yaml, ~/.codex/config.toml, ~/.hermes/config.yaml (all enabled) |
| Limitations | Browser automation fragile. Session expires. MCP unavailability handled gracefully by jobs skill. |
| Gaps | No CLI wrapper. Official LinkedIn API requires company-level partnership. |

### Telegram

| | |
|---|---|
| Canonical access | `tg` CLI + `telegram-readonly` MCP server (both backed by same Telethon daemon) |
| Protocol | MTProto via Telethon. Singleton daemon on Unix socket `~/.local/share/dotagents/telegram-readonly/daemon.sock`. |
| Auth | `TELEGRAM_API_ID` + `TELEGRAM_API_HASH` in `~/.agents/mcp/telegram-readonly/.env`. Session file at `~/.local/share/dotagents/telegram-readonly/telegram.session`. Login: `uv run python login.py`. |
| Skills | tg |
| MCP tools | list_dialogs, get_recent_messages, search_messages, get_chat_info |
| CLI commands | `tg dialogs`, `tg read`, `tg search`, `tg info` |
| Read | Chat listing, message reading, search across chats |
| Write | None. Read-only enforced in both MCP and CLI. |
| Config | ~/.codex/config.toml, ~/.hermes/config.yaml |
| Limitations | Idle timeout 30m (daemon sleeps). Session can expire requiring re-login. |
| Gaps | No write capability by design. Hermes has `TELEGRAM_HOME_CHANNEL: 38369051` for its own gateway but that's separate from dotagents. |

### Google Workspace (Gmail, Drive, Docs, Sheets, Calendar)

| | |
|---|---|
| Canonical access | `gws` CLI |
| Protocol | Google REST APIs via OAuth |
| Auth | `gws auth login`. Credentials at `~/.config/gws/`. Also `~/.hermes/auth/google_oauth.json` for Hermes. |
| Skills | gws, jobs (Gmail recruiter signal queries) |
| Read | Full CRUD on Drive, Docs, Sheets; Gmail search/read; Calendar events |
| Write | Docs, Sheets, Drive file ops. Gmail: "never send mail unless explicitly asked." |
| Limitations | None significant. |
| Gaps | Claude Code MCP for Google Workspace exists (claude.ai Gmail/Drive/Calendar) but is separate from gws CLI. Parity not enforced. |

### Web Search (general)

| | |
|---|---|
| Canonical access | Tavily MCP server (primary) + native agent WebSearch (fallback) |
| Protocol | Tavily: `npx mcp-remote https://mcp.tavily.com/mcp`. Native: agent built-in. |
| Auth | Tavily: OAuth via mcp-remote (server-side, no local API key). Native: none. |
| Skills | tech-search, repo-eval, jobs, and general fallback for any source |
| MCP tools | tavily_search, tavily_extract, tavily_crawl, tavily_map, tavily_research |
| Read | Web search, page extraction, site mapping |
| Limitations | Tavily auth mechanism opaque (mcp-remote handles it). WebFetch blocked by many sites (RA, LinkedIn, etc.). |
| Gaps | Tavily auth needs verification if it stops working. |

### Job ATS Portals (Greenhouse, Ashby, Lever)

| | |
|---|---|
| Canonical access | `portals-scan` Go tool (skills/jobs/tools/portals-scan/) |
| Protocol | Direct HTTP to public board APIs. No auth. |
| Endpoints | Greenhouse: `boards-api.greenhouse.io/v1/boards/{slug}/jobs`; Ashby: `api.ashbyhq.com/posting-api/job-board/{slug}`; Lever: `api.lever.co/v0/postings/{slug}` |
| Skills | jobs (/jobs scan) |
| Read | Job listings with title, location, team, compensation (where available) |
| Write | None |
| Limitations | Only 3 ATS platforms. Companies with custom career portals need `enabled: false`. 10 concurrent scans. |
| Gaps | No Workday, iCIMS, SmartRecruiters, BambooHR. These cover a large chunk of enterprise hiring. |

### Glassdoor / Levels.fyi

| | |
|---|---|
| Canonical access | WebSearch only (no structured integration) |
| Skills | jobs (/jobs check -- comp research) |
| Read | Whatever WebSearch returns for `"company" "role" salary levels.fyi glassdoor` |
| Limitations | No structured scraping. "If no data, say so -- do not invent numbers." |
| Gaps | levels.fyi has an unofficial API. Glassdoor blocks scraping aggressively. Low priority unless comp research becomes frequent. |

### Hugging Face

| | |
|---|---|
| Canonical access | HF Hub REST API (public) |
| Protocol | `GET https://huggingface.co/api/models/<org>/<model>` |
| Auth | None for public models |
| Skills | repo-eval (ML/model repos only) |
| Read | Model/dataset metadata: public/private state, downloads, likes, tags, files |
| Limitations | Read-only. Pickle formats flagged as trusted-code artifacts, never loaded. |
| Gaps | `huggingface-cli` exists but not wired. Would give authenticated access to gated models. |

### Resident Advisor (RA)

| | |
|---|---|
| Canonical access | **Not integrated.** Undocumented GraphQL endpoint, reverse-engineered. |
| Protocol | POST `https://ra.co/graphql` with operation `GET_EVENT_LISTINGS`. Area codes: Amsterdam=29. |
| Auth | None (public, but blocks standard user-agents and WebFetch/Tavily). Requires browser-like UA + Referer header. |
| Read | Event listings with title, venue, artists, genres, attendance, times |
| Limitations | No official API. GraphQL schema undocumented, reverse-engineered. May break. RA blocks all fetch tools (403). |
| Gaps | Needs a CLI wrapper or skill integration if event lookups become recurring. |

---

## Not yet integrated (known wants)

| Source | Why | Effort | Priority |
|---|---|---|---|
| **RA** | Event discovery in NL/EU cities | Low: wrap the known GraphQL endpoint in a CLI or skill step | Nice-to-have |
| **Glassdoor (structured)** | Comp research for job search | High: aggressive anti-scraping; unofficial APIs break | Low |
| **Workday/iCIMS ATS** | Enterprise career portals | Medium: each platform is different, no unified API | Low unless targeting large employers |
| **Reddit official API** | Headless-safe access without cookies | Medium: app registration + OAuth2 flow | Medium (fixes VPS access) |
| **X official API v2** | Eliminate endpoint drift from x-cli | Medium: need OAuth consumer flow, not just bearer token | Medium (stability) |
| **HF CLI** | Gated model access | Low: `pip install huggingface-cli`, wire into repo-eval | Low |

## Auth credential locations

| Source | Location | Rotation |
|---|---|---|
| X.com | `~/.x-cli/credentials.json` | Manual re-login on session expiry or GraphQL drift |
| Reddit | Browser cookies (managed by rdt-cli) | Manual; expires unpredictably |
| Discord | `DISCORD_TOKEN` env var | Manual; user-token, high risk |
| GitHub | `gh auth` keychain | Long-lived OAuth; rarely expires |
| LinkedIn | Playwright session (linkedin-scraper-mcp) | `uvx linkedin-scraper-mcp --login` on expiry |
| Telegram | `~/.local/share/dotagents/telegram-readonly/telegram.session` | Re-login via `uv run python login.py` |
| Google | `~/.config/gws/` + `~/.hermes/auth/google_oauth.json` | OAuth refresh; rarely expires |
| Tavily | mcp-remote managed (server-side) | Unknown; needs verification |

## Principles

1. **CLI-first.** If a CLI exists, prefer it over MCP. MCPs are for agent-native tool-use where CLI piping is awkward.
2. **Read-only default.** Write actions (sending messages, posting comments, connecting) require explicit user opt-in per invocation.
3. **Graceful degradation.** Every source with auth has a documented fallback (usually WebSearch). Skills must not block on MCP unavailability.
4. **No secrets in chat.** Credentials stay in their canonical locations. Agents never paste tokens, cookies, or API keys into conversation logs.
5. **ToS awareness.** Sources using unofficial access (x-cli, discord user-token, LinkedIn Playwright) are labeled with risk. Opt-in where risk is high.
6. **Agent-agnostic.** Source access methods work across Claude Code, Codex, Hermes, Droid. Agent-specific wiring details go in dotagents.yaml, not here.
