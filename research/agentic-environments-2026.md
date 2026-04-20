# One-Click Agentic Coding Environments (April 2026)

Python/ML focused, macOS Apple Silicon. Ranked by install simplicity and agentic capability.

## Tier 1: Production-Ready CLI Agents

| Tool | Install | Model | Cost | Stars |
|------|---------|-------|------|-------|
| **Claude Code** | `brew install --cask claude-code` or `curl -fsSL https://claude.ai/install.sh \| bash` | Claude 4.x (cloud) | $20-200/mo subscription | N/A (proprietary) |
| **OpenAI Codex** | `npm install -g @openai/codex` | GPT-5/o3 (cloud) | Usage-based | ~15K |
| **Goose** (Block/AAIF) | Desktop .dmg or `brew install goose` | Any (15+ providers, local via Ollama) | Free (Apache 2.0) | 29K |
| **OpenCode** | `brew install opencode` or single binary | Any (75+ providers, local) | Free (MIT) | 140K |
| **Aider** | `uv tool install aider-chat` | Any (Claude, GPT, local) | Free (Apache 2.0) | 30K+ |

## Tier 2: IDE-Integrated

| Tool | Install | Notes |
|------|---------|-------|
| **Cursor** | .dmg download | Fork of VS Code with built-in agent. $20/mo. |
| **Windsurf** (Codeium) | .dmg download | VS Code fork, Cascade agent. Free tier exists. |
| **Kiro** (AWS) | .dmg download | Spec-driven development, free during preview. |

## Tier 3: Local-Only / Privacy-First

| Tool | Install | Notes |
|------|---------|-------|
| **Ollama + OpenCode** | `brew install ollama && ollama pull qwen3.5` | Fully offline. Qwen3.5 35B MoE works on 16GB. |
| **llama.cpp + Aider** | Build from source or brew | Maximum control, no cloud dependency. |

## Recommendations for M1 Pro 16GB

For a friend doing Python/ML coding with agents:

1. **Claude Code** (if they have a subscription) - best agentic capability, zero config.
2. **Goose** - best free option, desktop app with extension ecosystem, works with any API key.
3. **OpenCode** - single binary, great TUI, works with local models or cloud.
4. **Aider** - Python-native, excellent git integration, good for ML workflows.

For fully local (no API keys): Ollama + Qwen3.5-Coder (fits in 16GB unified memory) paired with OpenCode or Aider as the frontend.

## Sources

- [Goose by Block](https://github.com/block/goose) - 29K stars, Apache 2.0
- [OpenCode](https://opencode.ai/) - 140K stars, MIT
- [Aider](https://aider.chat/) - AI pair programming
- [Claude Code Homebrew](https://formulae.brew.sh/cask/claude-code)
- [Fazm: Open-Source AI Agents Mac 2026](https://fazm.ai/blog/open-source-ai-agents-mac-2026)
- [KDnuggets: Goose Review](https://www.kdnuggets.com/free-agentic-coding-with-goose)
- [Pinggy: Top 5 CLI Coding Agents 2026](https://pinggy.io/blog/top_cli_based_ai_coding_agents/)
