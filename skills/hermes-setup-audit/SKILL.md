---
name: hermes-setup-audit
description: Systematic health audit of Hermes Agent integration with dotagents, knowledge vault, memsearch, and memory providers.
---

# Hermes Setup Audit

Use when the user asks to "check my hermes setup," "audit hermes," "review dotagents integration," or anything involving the health of Hermes + dotagents + knowledge vault + memsearch together.

## Prerequisites

- `hermes` on PATH
- `dotagents` skill/tool available in the dotagents repo
- `memsearch` installed (`uv tool list | grep memsearch`)

## Audit steps

Run these in order. Do not skip steps even if earlier ones look clean; failures cascade.

### 1. Locate the active Hermes installation

Hermes may be installed from source (e.g. `~/Public/hermes-agent`) rather than the current working directory. Find it before running Python checks.

```bash
which hermes
hermes --version
```

If you need to run Python against the Hermes source tree, `cd` to the directory shown by `which hermes` and `source venv/bin/activate`.

### 2. Dotagents sync state

```bash
cd ~/Workspace/dotagents && go run ./skills/dotagents/tools/dotagents status
```

**Watch for:** `drifted` on any agent. Run `sync` if drifted.

### 3. Knowledge vault structure

```bash
ls ~/Workspace/knowledge/{notes,profile,ai}/ 2>/dev/null || echo "MISSING VAULT DIRS"
```

Expected: `notes/`, `profile/`, `ai/` all exist.

### 4. Memsearch config and index state

```bash
cat ~/.agents/memsearch.conf
ls -la ~/.memsearch/state/
```

**Critical finding:** If `~/.memsearch/state/` is empty, the semantic index was never built. `memsearch recall` will return nothing.

Fix:
```bash
memsearch index
```

### 5. Hermes config external_dirs

```bash
grep -A3 "external_dirs" ~/.hermes/config.yaml
```

Expected: `skills.external_dirs` includes `~/.agents/skills`.

### 6. Hooks

```bash
hermes hooks list
```

**Watch for:** `script modified since approval` warning. Fix with `hermes hooks doctor`.

### 7. Memory provider availability

```bash
cd <hermes-source-dir>
source venv/bin/activate
python3 -c "
from plugins.memory import discover_memory_providers
for name, desc, avail in discover_memory_providers():
    print(f'{name}: available={avail}')
"
```

**Common state:** Only `holographic` is available without API keys. All others (honcho, supermemory, hindsight, byterover, mem0, openviking, retaindb) require env vars or services.

To enable a provider, either:
- Set the required env vars in `~/.hermes/.env`, or
- Set `memory.provider: <name>` in `~/.hermes/config.yaml`

### 8. Skill name overlap detection

When a dotagents skill has the same frontmatter `name:` as a Hermes builtin, both load and may duplicate in `/skills` output.

Check:
```bash
python3 -c "
from agent.skill_utils import get_all_skills_dirs
for d in get_all_skills_dirs():
    print(d)
"
```

Known overlap: dotagents `skills/gws/` (name `google-workspace`) vs Hermes builtin `google-workspace`. Prefer the Hermes builtin.

## Remediation checklist

If any of the above failed, suggest these in order:

1. `go run ./skills/dotagents/tools/dotagents sync` (if drifted)
2. `hermes hooks doctor` (if hook warning)
3. `memsearch index` (if state dir empty)
4. `git add skills/dotagents/SKILL.md && git commit -m "..."` (if uncommitted doc changes in dotagents repo)
5. Enable holographic memory or set provider env vars (if no memory provider active)
6. Rename or disable overlapping skill via `hermes skills config` (if duplicates detected)

## Pitfalls

- Do not assume Hermes source is in the current directory. Always `which hermes` first.
- `discover_memory_providers()` requires the venv activated and must be run from the Hermes source tree.
- An empty `~/.memsearch/state/` is silent failure: `memsearch recall` returns empty results without error.
- The built-in memory provider (`provider: ""`) works but has no semantic search; holographic is the cheapest upgrade.
