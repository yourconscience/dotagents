# Design: separate dotagents CLI from user config

Grilling session 2026-07-16. All decisions confirmed by user.

## Problem

The public repo `yourconscience/dotagents` is both the Go CLI tool AND the
user's personal `~/.agents` (symlinked). New users see 22 personal skills,
25 hooks, personal MCP servers — confusing and unusable. The tool and the
user's dotfiles must live in separate repos.

## Architecture

Two repos after separation:

**A — public `yourconscience/dotagents`** (tool):
```
cmd/dotagents/          # Go CLI source
skills/
  dotagents/            # self-management skill (ships with tool)
  grilling/             # external skill from mattpocock (pinned, demo of external pinning)
agents/
  architect.md          # 5 generic roles — useful starter library
  builder.md
  researcher.md
  reviewer.md
  tester.md
memory/                 # hooks + lib + tools (memory infrastructure)
docs/
dotagents.yaml          # minimal template (see below)
.goreleaser.yaml
.github/
go.work, go.mod, go.sum
README.md, LICENSE, AGENTS.md, CLAUDE.md
```

No `.claude-plugin/`, no `.codex-plugin/`, no `plugins:` section.

**B — private user repo** (becomes `~/.agents`):
```
skills/                 # all 22 personal skills
agents/                 # can override/extend public roles
hooks/                  # cmux, pr-triage, personal hooks
mcp/                    # personal MCP configs
memory/                 # can override hooks
dotagents.yaml          # full personal config
dotagents.lock          # personal lock file
external/               # external skill cache
```

## Decisions

### 1. No `init` command — `setup` handles everything

`dotagents setup` gains scaffold responsibility:
- If `~/.agents` does not exist → create it, detect harnesses, generate
  `dotagents.yaml`, run first sync
- If `~/.agents` exists → patch harness configs as today

### 2. Skills in public repo

Only two skills ship:
- `skills/dotagents/` — self-management (own)
- `skills/grilling/` — external from mattpocock/skills (pinned in lock,
  demonstrates external skill pinning)

All 20 other personal skills move to private repo.

### 3. Agent roles stay

All 5 generic roles (architect, builder, researcher, reviewer, tester)
stay in public repo. Users override by creating same-name files in their
`~/.agents/agents/`.

### 4. Plugins removed entirely

- No `plugins:` section in `dotagents.yaml`
- No `.claude-plugin/`, `.codex-plugin/` packaging
- No plugin delivery mode (`--delivery plugin`)
- No plugin projection (projecting native plugin skills to other harnesses)
- Remove `plugins.go`, `delivery.go`, related tests, doctor plugin checks
- ~400 lines of Go code removed

dotagents syncs exactly four things: **skills, MCP servers, hooks, agent roles**.

### 5. Config resolution

CLI always reads `~/.agents/dotagents.yaml`. Override via `DOTAGENTS_HOME`
env var or `--config` flag. No CWD walk, no project-level configs.
dotagents is user-level, not project-level — this is the key differentiator
from rulesync/ruler.

### 6. Template dotagents.yaml in public repo

Minimal working config, not a reference of all options:

```yaml
version: 1
agents: []  # populated by dotagents setup

external_skills:
  - url: https://github.com/mattpocock/skills
    branch: main
    skill_dirs: [skills/productivity/grilling]
    materialize: true

hooks:
  - name: memory-session-start
    enabled: true
    event: SessionStart
    command: ~/.agents/memory/hooks/session-start.sh
    timeout: 15
    agents: [claude-code]
  - name: memory-stop
    enabled: true
    event: Stop
    command: ~/.agents/memory/hooks/stop.sh
    timeout: 15
    agents: [claude-code]
  - name: memory-session-end
    enabled: true
    event: SessionEnd
    command: ~/.agents/memory/hooks/session-end.sh
    timeout: 30
    agents: [claude-code]
```

`agents:` section left empty — `setup` fills it with detected harnesses.
Memory hooks included as working example. Knowledge config via
`~/.agents/memsearch.conf`.

### 7. Setup pipeline with existing content import

When a user already has skills/agents/MCP in harness-native locations,
`setup` discovers and offers to import them:

```
Step 1: Detect harnesses
  ✓ claude-code
  ✓ codex

Step 2: Scan existing content
  Found 3 skills in ~/.claude/skills/
  Found 2 agents in ~/.codex/agents/
  Found 4 MCP servers in ~/.claude/settings.json

Step 3: Import?
  Import 3 skills into ~/.agents/skills/? [Y/n]
  Import 2 agent roles into ~/.agents/agents/? [Y/n]
  Import 4 MCP servers into ~/.agents/dotagents.yaml? [Y/n]

Step 4: Sync
  Done. Run `dotagents status` to verify.
```

Rules:
- Import = copy + convert, not move. Originals stay untouched.
- Codex `.toml` roles → `.md` format conversion
- MCP from `settings.json` → `dotagents.yaml` mcp_servers entries
- Name conflicts across harnesses: v1 — show both, ask "keep left /
  keep right / skip". No merge. Diff improvements = separate iteration.
- After import, `dotagents sync` overwrites harness-native copies from
  `~/.agents/` as canonical source.

### 8. Memory tiers

Three levels, selectable via `dotagents setup --memory <tier>`:

| Tier | Behavior | Dependencies |
|---|---|---|
| off | hooks not registered | none |
| basic (default) | session-end writes digest to `$KNOWLEDGE_DIR/sessions/YYYY-MM-DD.md`; session-start injects recent entries as context | Python 3 |
| memsearch | full-text index, search, vault sync (current behavior) | memsearch binary |

`basic` tier = new simple scripts that write markdown files without
external dependencies beyond Python. Works out of the box.

`dotagents setup` defaults to basic. Explains what memory does and how
to upgrade to memsearch or disable.

**Independent review of memory scheme required after implementation,
before e2e tests.**

### 9. Migration for us

Manual, not automated (we're the only existing user of the current layout):
1. Create private repo with personal content
2. Clean public repo to match spec above
3. Redirect `~/.agents` symlink to private repo
4. `dotagents sync` to verify

## Execution plan

Sequential — each step depends on the previous:

| # | Task | Who |
|---|---|---|
| 1 | Remove plugins (config, Go code, tests) | builder |
| 2 | Separate repo: remove personal content, keep tool + 2 skills + 5 roles + memory | builder |
| 3 | Setup pipeline: detect → scan → show → ask → import → sync | builder |
| 4 | Memory tiers: off/basic/memsearch | builder |
| 5 | Config resolution: always ~/.agents/, DOTAGENTS_HOME override | builder |
| 6 | README for new structure | builder |
| 7 | Independent review of memory scheme | reviewer |
| 8 | E2e test: setup on machine with existing skills → import → sync → status | tester |
| 9 | Update launch posts for new messaging | manual |

## What dotagents syncs (post-separation)

Four things, nothing else:
1. **Skills** — SKILL.md files, external pinning with lock file + audit
2. **MCP servers** — declared in dotagents.yaml, rendered to each harness
3. **Hooks** — lifecycle hooks registered per harness
4. **Agent roles** — .md role definitions rendered to each harness format
