#!/usr/bin/env bash
set -euo pipefail

# Setup script for a fresh macOS (Apple Silicon) machine with agentic coding tools.
# Installs: Homebrew, core dev tools, Python/ML stack, coding agents, dotagents.
# Usage: curl -fsSL <raw-url> | sudo bash -s -- --user <username>
#   or:  sudo ./setup-macbook.sh --user <username>

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[x]${NC} $*" >&2; exit 1; }

# --- Parse args ---
TARGET_USER=""
SKIP_CLAUDE=false
SKIP_CODEX=false
INSTALL_OLLAMA=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --user)       TARGET_USER="$2"; shift 2 ;;
    --skip-claude) SKIP_CLAUDE=true; shift ;;
    --skip-codex)  SKIP_CODEX=true; shift ;;
    --ollama)      INSTALL_OLLAMA=true; shift ;;
    -h|--help)
      echo "Usage: sudo $0 --user <username> [--skip-claude] [--skip-codex] [--ollama]"
      echo ""
      echo "Options:"
      echo "  --user <name>    macOS username to configure (required)"
      echo "  --skip-claude    Skip Claude Code installation"
      echo "  --skip-codex     Skip OpenAI Codex installation"
      echo "  --ollama         Install Ollama + Qwen3.5-Coder for local inference"
      exit 0
      ;;
    *) err "Unknown option: $1" ;;
  esac
done

[[ -z "$TARGET_USER" ]] && err "--user is required"
TARGET_HOME=$(eval echo "~$TARGET_USER")
[[ -d "$TARGET_HOME" ]] || err "Home directory $TARGET_HOME does not exist"

run_as_user() {
  sudo -u "$TARGET_USER" -- "$@"
}

# --- Homebrew ---
if ! command -v brew &>/dev/null; then
  log "Installing Homebrew..."
  NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  eval "$(/opt/homebrew/bin/brew shellenv)"
else
  log "Homebrew already installed"
fi

# Ensure brew is on PATH for the target user
BREW_SHELLENV='eval "$(/opt/homebrew/bin/brew shellenv)"'
for rc in "$TARGET_HOME/.zprofile" "$TARGET_HOME/.profile"; do
  if [[ -f "$rc" ]] && grep -q "brew shellenv" "$rc"; then
    break
  fi
  if [[ "$rc" == "$TARGET_HOME/.zprofile" ]]; then
    echo "$BREW_SHELLENV" >> "$rc"
    chown "$TARGET_USER" "$rc"
    break
  fi
done

export PATH="/opt/homebrew/bin:$PATH"

# --- Core tools ---
log "Installing core dev tools..."
brew install git gh node go python@3.12 uv ripgrep fd jq tmux

# --- Python / ML ---
log "Setting up Python environment..."
run_as_user uv python install 3.12
run_as_user uv tool install ipython
run_as_user uv tool install ruff
run_as_user uv tool install aider-chat

# --- Coding agents ---
if [[ "$SKIP_CLAUDE" == false ]]; then
  log "Installing Claude Code..."
  brew install --cask claude-code || true
fi

if [[ "$SKIP_CODEX" == false ]]; then
  log "Installing OpenAI Codex CLI..."
  run_as_user npm install -g @openai/codex || warn "Codex install failed (needs npm auth or API key later)"
fi

log "Installing Goose (Block)..."
brew install --cask goose || brew install goose || warn "Goose install failed - try manual download from https://github.com/block/goose/releases"

log "Installing OpenCode..."
brew install opencode || {
  warn "OpenCode not in brew, installing from binary..."
  curl -fsSL https://opencode.ai/install.sh | run_as_user bash
} || warn "OpenCode install failed"

# --- Local models (optional) ---
if [[ "$INSTALL_OLLAMA" == true ]]; then
  log "Installing Ollama..."
  brew install --cask ollama
  log "Pulling Qwen3.5-Coder (fits 16GB unified memory)..."
  run_as_user ollama pull qwen3.5-coder:latest || warn "Model pull failed - run 'ollama pull qwen3.5-coder' manually after Ollama starts"
fi

# --- dotagents ---
log "Cloning dotagents repo..."
DOTAGENTS_DIR="$TARGET_HOME/Workspace/dotagents"
if [[ -d "$DOTAGENTS_DIR" ]]; then
  log "dotagents already exists at $DOTAGENTS_DIR"
else
  run_as_user mkdir -p "$TARGET_HOME/Workspace"
  run_as_user git clone https://github.com/yourconscience/dotagents.git "$DOTAGENTS_DIR"
fi

log "Running dotagents setup..."
cd "$DOTAGENTS_DIR"
run_as_user go run ./skills/dotagents/tools/dotagents setup

log "Installing dotagents auto-pull cron (every 30m)..."
run_as_user go run ./skills/dotagents/tools/dotagents cron --interval 30m

# --- Shell config ---
log "Adding useful aliases..."
ALIASES_FILE="$TARGET_HOME/.aliases_agents"
cat > "$ALIASES_FILE" << 'ALIASES'
# Agentic coding shortcuts
alias cc='claude'
alias oc='opencode'
alias aid='aider'
alias goo='goose'

# Python/ML
alias py='python3'
alias ipy='ipython'
alias uvr='uv run'
alias uvp='uv pip'
ALIASES
chown "$TARGET_USER" "$ALIASES_FILE"

# Source aliases from .zshrc if not already there
ZSHRC="$TARGET_HOME/.zshrc"
if [[ -f "$ZSHRC" ]] && ! grep -q ".aliases_agents" "$ZSHRC"; then
  echo '[[ -f ~/.aliases_agents ]] && source ~/.aliases_agents' >> "$ZSHRC"
fi

# --- Summary ---
echo ""
log "Setup complete. Installed:"
echo "  Core:    git, gh, node, go, python 3.12, uv, ripgrep, tmux"
echo "  Python:  ipython, ruff, aider-chat (via uv tools)"
echo "  Agents:  $([ "$SKIP_CLAUDE" == false ] && echo 'Claude Code, ')$([ "$SKIP_CODEX" == false ] && echo 'Codex, ')Goose, OpenCode, Aider"
[[ "$INSTALL_OLLAMA" == true ]] && echo "  Local:   Ollama + Qwen3.5-Coder"
echo "  Skills:  dotagents (~/Workspace/dotagents -> ~/.agents)"
echo ""
warn "Next steps:"
echo "  1. Open a new terminal (or: source ~/.zshrc)"
echo "  2. Run 'claude' and authenticate with Anthropic (if using Claude Code)"
echo "  3. Run 'goose' and configure your preferred LLM provider"
echo "  4. For local models: open Ollama.app, then 'opencode' or 'aider --model ollama/qwen3.5-coder'"
echo ""
