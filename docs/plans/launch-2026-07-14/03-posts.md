# Launch posts — drafts for review

Publish LinkedIn + X after the separation PR is merged and the fresh-home E2E
passes; publish Show HN 2–3 days later. Before posting, run the X thread
through `x-sim`. Re-verify every harness and install claim against the merged
repo at post time.

---

## LinkedIn (single post, EN, personal account)

My AI coding agents all want the same four things: skills, MCP servers, hooks,
and subagent roles. Each harness stores them differently, so I kept copying
and translating the same setup.

For shell config we solved this decades ago: dotfiles. One private repo,
versioned, follows you across machines.

So I built dotagents: a small open-source Go CLI that keeps `~/.agents` as
your private canonical config and syncs it into each harness's native format.
The tool and your config are deliberately separate repos. Cloning the tool
doesn't install my personal prompts, hooks, or MCP servers.

Four commands: setup, status, sync, doctor.

The part I care about most: external skills are commit-pinned in a lock file
and audited for risky patterns before an agent loads them. Prompt code from
someone else's repo is still a dependency; it should be reviewed and updated
deliberately.

It's open source: https://github.com/yourconscience/dotagents

If you use more than one coding agent, I'd like to hear which config surface
is hardest to keep consistent.

## X thread (EN, 4 posts)

**1/**
Dotfiles for your AI agents.

Skills, MCP servers, hooks, and roles keep reappearing across coding agents,
but every harness stores them differently.

I got tired of copying config between them.

**2/**
dotagents keeps one private `~/.agents` repo as the canonical source and syncs
each surface into the harness's native format.

The CLI lives in a separate public repo. Installing the tool does not install
my personal skills, hooks, or MCP servers.

[attach: updated harness-map.png]

**3/**
External skills are treated like dependencies: commit-pinned in a lock file,
audited for risky patterns, and updated deliberately.

Prompt code from somebody else's repo deserves the same supply-chain caution
as executable code.

**4/**
Four commands: setup, status, sync, doctor.

Install the CLI, run setup, and keep your config in your own private repo.

https://github.com/yourconscience/dotagents

The public starter includes a pinned skill from @mattpocockuk's repo.
<check handle before posting>

## Show HN (after first feedback is folded in)

**Title:** Show HN: Dotagents – dotfiles for your AI coding agents

**Text:**

I work across several coding-agent harnesses. Each one has its own native
format for the same four concepts: skills, MCP servers, hooks, and subagent
roles.

dotagents applies the dotfiles pattern at the user level. Your private
`~/.agents` repo is the canonical store; a Go CLI syncs supported surfaces
into each harness's native format. The public CLI repo and private user config
are intentionally separate, so installing the tool never exposes or installs
the maintainer's personal setup.

setup / status / sync / doctor, nothing else to learn.

Two design choices I'd like feedback on:

1. External skills are treated like dependencies: commit-pinned in a lock
file and audited for risky patterns before an agent sees them.

2. It is user-level, not project-level. Tools such as rulesync and ruler
generate per-tool files inside a project; dotagents manages one personal
configuration across projects and machines.

Honest limitations: macOS/Linux only, support follows harness-native
capabilities, and not every harness exposes all four surfaces.

**First comment (mine, posted immediately):** explain why the original public
repo containing the maintainer's personal skills, hooks, and MCP catalog was
the wrong trust boundary; then summarize the external-skill pinning model.

## Community calendar (user copy-pastes; drafts below each venue)

| Day | Venue | Action |
|---|---|---|
| D0 (after separation merge) | LinkedIn + X | main posts above; reply to every comment same day |
| D0 evening | Pi Discord (#show-and-tell or closest) | short note: dotagents now treats pi as a first-party target, link + one screenshot. Frame as "built this for my own pi setup", not an ad |
| D1 | r/ClaudeAI | condensed LinkedIn post + Claude-specific skills, MCP, hooks, and roles behavior. Check sub self-promo rules before posting <check> |
| D1 | Claude Code Discord (if a showcase channel exists) | same note as Pi Discord, Claude-angle |
| D2 | r/ChatGPTCoding | Codex-angle version focused on native config sync and private user ownership <check sub rules> |
| D2–3 | awesome-list PRs | awesome-claude-code + any current awesome-ai-coding-agents lists; verify each list's contribution rules and that the category fits <research at PR time> |
| D3–4 | Show HN | morning US time, Tue–Thu; user stays available 3–4h for comments |

Rules for all venues: no star-begging anywhere, disclose "I built this",
one post per venue, answer criticism straight (the HN crowd will ask "why
not rulesync" — the answer is user-level vs project-level + pinning, it's
already in the post).

## Star expectations (honest)

- Day 1: ~20–40, mostly personal network from LinkedIn/X. Normal, not gaming.
- Week 1 target: 50–100 if the hook lands in one community.
- Real driver long-term: HN front page or an upstream author retweet; both
  are lottery tickets, the calendar just buys tickets.

## Pre-publish checklist

- [ ] Separation PR merged with no unresolved review threads
- [ ] Fresh-home setup/import/sync/status E2E passes
- [ ] harness-map.png matches the supported harness matrix
- [ ] Every claim in the posts matches the merged repo
- [ ] X thread run through `x-sim`, adjust wording only (not the claims)
- [ ] Upstream handles verified (@mattpocockuk, badlogic's current X handle)
- [ ] Landing page (GitHub Pages) reflects new README
