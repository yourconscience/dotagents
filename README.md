# dotagents

Dotfiles for your AI agents. `dotagents` keeps one user-owned `~/.agents` repository as the canonical source for skills, MCP servers, hooks, and agent roles, then syncs each supported surface into the harness's native format.

The public tool and private user configuration are separate repositories. Installing `yourconscience/dotagents` does not install the maintainer's personal prompts, hooks, or MCP catalog.

**Landing page:** [yourconscience.github.io/dotagents](https://yourconscience.github.io/dotagents/) · [**Releases**](https://github.com/yourconscience/dotagents/releases)

![dotagents harness map](./docs/harness-map.png)

## Install

Install the CLI on macOS or Linux:

```bash
# Prebuilt release binary
curl -fsSL https://raw.githubusercontent.com/yourconscience/dotagents/main/scripts/install.sh | sh

# or via mise
mise use -g github:yourconscience/dotagents

# or with Go
go install github.com/yourconscience/dotagents/cmd/dotagents@latest
```

Then create your private canonical configuration:

```bash
dotagents setup
```

`setup` creates `~/.agents` when needed, detects installed harnesses, scans their existing skills, roles, and MCP servers, offers to copy or convert that content without modifying the originals, writes `dotagents.yaml`, and runs the first sync. Name conflicts are resolved explicitly; content is never merged silently.

Version the result in a private repository you control:

```bash
cd ~/.agents
git init
git add .
git commit -m "initialize agent configuration"
git remote add origin <your-private-repository>
git push -u origin main
```

Use `DOTAGENTS_HOME=/path/to/config` to replace `~/.agents`, or pass `--config /path/to/dotagents.yaml` for one command. dotagents never discovers configuration by walking the current project.

## What it syncs

Exactly four surfaces:

1. **Skills** — canonical `SKILL.md` directories, including commit-pinned external Git skills.
2. **MCP servers** — canonical YAML entries rendered into each supported native config.
3. **Hooks** — lifecycle hooks registered only where the harness exposes a verified hook surface.
4. **Agent roles** — canonical Markdown role definitions rendered into each supported native format.

| Harness | Skills | Roles | MCP | Hooks |
|---|---|---|---|---|
| Claude Code | yes | yes | yes | yes |
| Codex | yes | yes | yes | yes |
| Factory Droid | yes | yes | yes | yes |
| Hermes | yes | -- | yes | yes |
| Pi | yes | -- | -- | -- |
| OMP | yes | yes | yes | -- |

Unsupported surfaces are skipped; dotagents does not invent compatibility files a harness cannot consume.

## Public starter content

<!-- BEGIN GENERATED SKILLS -->
2 skills ship with this repo:

`dotagents` `grilling`
<!-- END GENERATED SKILLS -->

`dotagents` provides self-management guidance. `grilling` is a materialized,
commit-pinned example from [`mattpocock/skills`](https://github.com/mattpocock/skills).

It also ships five generic Markdown roles:

`architect` `builder` `researcher` `reviewer` `tester`

First-run setup copies missing starter assets into the private canonical root. Files already owned by the user are not overwritten. A same-name role in `~/.agents/agents/` is the canonical user version.

## Memory tiers

Choose a tier during setup:

```bash
dotagents setup --memory basic      # default
dotagents setup --memory off
dotagents setup --memory memsearch
```

| Tier | Behavior | Dependency |
|---|---|---|
| `off` | registers no managed memory hooks | none |
| `basic` | appends bounded session digests under `$KNOWLEDGE_DIR/sessions/` and injects recent entries at session start | Python 3 |
| `memsearch` | indexed search and vault synchronization through the full memory hook pipeline | `memsearch` |

Memory data belongs in the user's knowledge directory, not in the public tool repository.

## Key commands

```bash
dotagents setup [--memory off|basic|memsearch] [--agents ...]
dotagents status [--agents ...]
dotagents sync [--pull] [--agents ...]
dotagents doctor [--e2e] [--agents ...]
dotagents skill new <name> [--description ...]
dotagents skill update [name ...]
dotagents skill promote <name-or-path> [--dry-run]
dotagents mcp <list|add|import|remove> [options]
```

Run `dotagents help --all` for maintenance commands and explicitly labeled compatibility aliases.

## Configuration

The generated `~/.agents/dotagents.yaml` is the source of truth. The public template is intentionally small:

```yaml
version: 1
agents: [] # populated by dotagents setup

external_skills:
  - url: https://github.com/mattpocock/skills
    branch: main
    skill_dirs: [skills/productivity/grilling]
    materialize: true
```

Agent entries record detected harness paths. MCP servers and hooks are managed entries; unrelated native config remains untouched.

## External skills

Pull and pin a Git skill library once, selecting paths from any upstream subdirectory:

```yaml
external_skills:
  - url: https://github.com/example/shared-skills
    branch: main
    skill_dirs: [engineering/alpha, productivity/beta]
    materialize: true
```

`materialize: true` makes `dotagents sync` copy each selected tree exactly into canonical `skills/<basename>` and prune stale owned files. `dotagents.lock` pins the source commit and records ownership so `doctor` can detect drift. `dotagents skill update [name ...]` explicitly advances pins. `doctor` also scans external sources for risky patterns before delivery.

## How it differs

[rulesync](https://github.com/dyoshikawa/rulesync) and [ruler](https://github.com/intellectronica/ruler) are project-level tools: they generate per-tool configuration inside a project and cover a broad tool set. dotagents is user-level: one private configuration follows you across projects, machines, and supported harnesses. [openskills](https://github.com/numman-ali/openskills) focuses on installing skills; dotagents manages the four user-level surfaces together and pins external Git skill sources.
