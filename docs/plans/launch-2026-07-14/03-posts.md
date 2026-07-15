# Launch posts — drafts for review

Publish order: LinkedIn + X tomorrow (2026-07-15) after the diff review; Show
HN 2–3 days later. Before posting, run the X thread through `x-sim`.
All numbers marked `<check>` must be re-verified against the repo at post time
(skill count becomes generated as part of the builder work).

---

## LinkedIn (single post, EN, personal account)

I use five AI coding agents day to day: Claude Code, Codex, Droid, Hermes,
and Pi. Across them I want the same skills, MCP servers, hooks, and subagent
definitions, but each harness supports a different subset and native format.

For shell config we solved this decades ago: dotfiles. One repo, versioned,
follows you across machines. So I built the same thing for agents.

dotagents: one ~/.agents repo holds your whole agent setup, and a small Go
CLI syncs it into each harness's native format. Four commands: setup, status,
sync, doctor.

The part I actually care about most: skills you pull from other people's
repos are commit-pinned in a lock file and audited for risky patterns before
any agent loads them. We're all currently installing prompt code from
strangers with no pinning at all. That should bother you more than it does.

What struck me while building it: every harness is converging on the same
four concepts — skills, MCP servers, hooks, roles. They just spell them
differently. Once you see that, one canonical layer that renders into each
tool is the obvious move.

It's open source: https://github.com/yourconscience/dotagents

If you also live in more than one coding agent, I'd genuinely like to hear
what breaks for you.

---

## X thread (EN, 4 posts)

**1/**
Dotfiles for your AI agents.

I use 5 coding agents (Claude Code, Codex, Droid, Hermes, Pi). Each wants
skills, MCP, hooks, and roles in its own format. Copying configs between
them got old fast.

So: one ~/.agents repo, synced into every harness natively.

[attach: updated harness-map.png]

**2/**
The insight that made it click: every harness is converging on the same four
concepts — skills, MCP servers, hooks, subagent roles. They just spell them
differently.

That's not fragmentation to fight. It's a rendering problem: one canonical
repo, N native outputs.

**3/**
The part nobody else does: external skills are commit-pinned in a lock file
and audited for risky patterns before any agent loads them.

We're all curl-piping prompt code from strangers into our agents. dotagents
treats skills like dependencies: pinned, reviewed, updated deliberately.

**4/**
Four commands: setup, status, sync, doctor.

Install skills through the native Claude Code or Codex marketplace flow, or
install the full CLI via curl or Go.

https://github.com/yourconscience/dotagents

Built on ideas from @mattpocockuk's skills repo (pinned as externals) —
and Pi support for @badlogicgames' pi. <check handles before posting>

---

## Show HN (post ~2026-07-17, after first feedback folded in)

**Title:** Show HN: Dotagents – dotfiles for your AI coding agents

**Text:**

I work across five coding agents daily (Claude Code, Codex, Factory Droid,
Hermes, Pi). Each one invented its own config surface for the same four
things: skills, MCP servers, hooks, and subagent roles.

dotagents applies the dotfiles pattern: one ~/.agents git repo is the
canonical store, and a Go CLI renders it into each harness's native format.
setup / status / sync / doctor, nothing else to learn.

Two design choices I'd like feedback on:

1. External skills are treated like dependencies: commit-pinned in a lock
file, audited for risky patterns (exfil, shell abuse) before any agent sees
them. As far as I can tell nobody else pins agent skills at all.

2. It's user-level, not project-level. rulesync and ruler generate per-tool
config inside a repo and cover ~30–35 tools; dotagents instead covers 5–6
harnesses deeply (including plugin delivery and mutual-exclusion guards so
you can't double-install) and your setup follows you across machines.

Honest limitations: macOS/Linux only, the harness list is what I personally
use, and role rendering depends on what each harness natively supports
(vanilla pi has no subagents, for example).

**First comment (mine, posted immediately):** short story of the bug that
motivated the doctor command — skills listed twice because plugin + symlink
delivery were both active; plus the supply-chain argument in one paragraph.

---

## Community calendar (user copy-pastes; drafts below each venue)

| Day | Venue | Action |
|---|---|---|
| D0 (2026-07-15) | LinkedIn + X | main posts above; reply to every comment same day |
| D0 evening | Pi Discord (#show-and-tell or closest) | short note: dotagents now treats pi as a first-party target, link + one screenshot. Frame as "built this for my own pi setup", not an ad |
| D1 | r/ClaudeAI | title: "One repo for skills/MCP/hooks across Claude Code and 4 other agents" — body = condensed LinkedIn post + what's Claude-specific (plugin delivery, dual-install guard). Check sub self-promo rules before posting <check> |
| D1 | Claude Code Discord (if a showcase channel exists) | same note as Pi Discord, Claude-angle |
| D2 | r/ChatGPTCoding | Codex-angle version (plugin marketplace install path) <check sub rules> |
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

- [ ] Builder diff merged, tester report has no blockers
- [ ] Fresh-machine install per README actually works (tester §D)
- [ ] harness-map.png regenerated (used in both posts)
- [ ] Skill count / claims in posts match repo (`<check>` marks resolved)
- [ ] X thread run through `x-sim`, adjust wording only (not the claims)
- [ ] Upstream handles verified (@mattpocockuk, badlogic's current X handle)
- [ ] Landing page (GitHub Pages) reflects new README
