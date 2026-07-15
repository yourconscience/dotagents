# Agent roles

`subagents.yaml` is the single source of truth for agent roles. The `.md` files
next to it are generated Claude plugin role files (`dotagents sync`); do not
edit them directly.

## Format

```yaml
version: 1
agents:
  - name: reviewer
    description: Reviews code changes against specs and best practices.
    model: opus
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
  `model` is the Claude Code model; use Claude aliases such as `opus` (the
  current Opus 4.6 family) rather than pinning a dated full model ID.
- `codex` (`model`, `model_reasoning_effort`) and `droid` (`model`,
  `reasoning_effort`, `tools`) - optional per-harness overrides.
- `instructions_file` - optional path relative to this directory whose contents
  become the prompt. Exactly one of `instructions` or `instructions_file` must
  be set.

After editing, run `dotagents sync` to regenerate the committed `.md` role
files and deliver native formats to each supported harness.

## Provider routing

The canonical roles support both subscription-backed providers without making
one silently fall back to the other:

- Claude Code receives the top-level `model` (currently `opus`, resolving to
  Opus 4.6).
- Codex receives `codex.model`: GPT-5.6 Sol for architecture, implementation,
  and review; GPT-5.6 Luna for research. Each role also carries its explicit
  reasoning effort.
- Droid receives `droid.model` when set; otherwise the renderer maps the
  top-level Claude family aliases to the configured GPT-5.5 custom models.

GPT-5.6 Sol and Luna are explicit role routes, not automatic fallback-chain
entries. This keeps model selection predictable and prevents silent provider
fallbacks.

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
