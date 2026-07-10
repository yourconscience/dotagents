# Agent roles

`subagents.yaml` is the single source of truth for agent roles. The `.md` files
next to it are generated Claude-format renders (`dotagents render`); do not
edit them directly.

## Format

```yaml
version: 1
agents:
  - name: reviewer
    description: Reviews code changes against specs and best practices.
    effort: high
    tools: [Read, Glob, Grep, Bash]
    color: purple
    codex:
      model: gpt-5.6-sol
      model_reasoning_effort: high
    instructions: |-
      You are a senior code reviewer. ...
```

Fields per agent:

- `name`, `description`, `instructions` - required.
- `model`, `effort`, `tools`, `color` - optional; used by the harness renders.
- `codex` (`model`, `model_reasoning_effort`) and `droid` (`model`,
  `reasoning_effort`, `tools`) - optional per-harness overrides.
- `instructions_file` - optional path relative to this directory whose contents
  become the prompt. Exactly one of `instructions` or `instructions_file` must
  be set.

After editing, run `dotagents render` to regenerate the committed `.md` renders
and `dotagents sync` to push them to harnesses.

## Inline vs external prompts

Inline `instructions` (default): single source of truth, atomic diffs when a
role changes, greppable alongside its metadata. Preferred while prompts stay
short.

External `instructions_file`: better editor and linting support for long
prompts, and the file can be reused outside dotagents. Costs: two-file diffs
and drift risk between metadata and prompt. Use only when a prompt outgrows a
comfortable inline block.

## Compatibility

Legacy per-agent `agents/<name>.yaml` files are still loaded when
`subagents.yaml` is absent (with a deprecation note). Having both forms at once
is an error.
