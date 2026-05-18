#!/usr/bin/env bash
set -euo pipefail

# Setup script for a fresh macOS (Apple Silicon) machine with agentic coding tools.
# Installs: Homebrew, shell tools, Python/ML stack, coding agents, dotagents.
# Usage: sudo ./setup-macbook.sh --user <username>

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
INSTALL_HERMES=false
INSTALL_OLLAMA=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --user)        TARGET_USER="$2"; shift 2 ;;
    --skip-claude) SKIP_CLAUDE=true; shift ;;
    --skip-codex)  SKIP_CODEX=true; shift ;;
    --hermes)      INSTALL_HERMES=true; shift ;;
    --ollama)      INSTALL_OLLAMA=true; shift ;;
    -h|--help)
      echo "Usage: sudo $0 --user <username> [--skip-claude] [--skip-codex] [--hermes] [--ollama]"
      echo ""
      echo "Options:"
      echo "  --user <name>    macOS username to configure (required)"
      echo "  --skip-claude    Skip Claude Code installation"
      echo "  --skip-codex     Skip OpenAI Codex installation"
      echo "  --hermes         Install Hermes Agent (Nous Research)"
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

# --- Core dev tools ---
log "Installing core dev tools..."
run_as_user /opt/homebrew/bin/brew install git gh node go python@3.12 uv ripgrep fd jq tmux neovim

# --- Modern CLI replacements ---
log "Installing modern CLI tools..."
run_as_user /opt/homebrew/bin/brew install fzf bat eza lazygit zoxide glow
run_as_user /opt/homebrew/bin/brew install --cask font-hack-nerd-font || true

# --- Oh-My-Zsh ---
OMZ_DIR="$TARGET_HOME/.oh-my-zsh"
if [[ ! -d "$OMZ_DIR" ]]; then
  log "Installing Oh-My-Zsh..."
  run_as_user sh -c 'RUNZSH=no KEEP_ZSHRC=yes sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"'
else
  log "Oh-My-Zsh already installed"
fi

# --- Python / ML ---
log "Setting up Python environment..."
run_as_user uv python install 3.12
run_as_user uv tool install ipython
run_as_user uv tool install ruff==0.15.12

# --- Coding agents ---
if [[ "$SKIP_CLAUDE" == false ]]; then
  log "Installing Claude Code..."
  run_as_user /opt/homebrew/bin/brew install --cask claude-code || true
fi

if [[ "$SKIP_CODEX" == false ]]; then
  log "Installing OpenAI Codex CLI..."
  run_as_user npm install -g @openai/codex || warn "Codex install failed (needs npm auth or API key later)"
fi

# --- Hermes Agent (optional) ---
if [[ "$INSTALL_HERMES" == true ]]; then
  log "Installing Hermes Agent (Nous Research)..."
  run_as_user bash -c 'curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash' || warn "Hermes install failed - try: pip install hermes-agent"
fi

# --- Local models (optional) ---
if [[ "$INSTALL_OLLAMA" == true ]]; then
  log "Installing Ollama..."
  run_as_user /opt/homebrew/bin/brew install --cask ollama
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
run_as_user go run ./cmd/dotagents setup

# --- Knowledge vault + memsearch ---
log "Setting up knowledge vault + memsearch..."
run_as_user uv tool install memsearch==0.4.2 || warn "memsearch install failed"
run_as_user go run ./cmd/dotagents memsearch setup --vault "$TARGET_HOME/Workspace/knowledge" || warn "memsearch setup failed (non-critical)"

log "Installing dotagents auto-pull cron (every 30m)..."
run_as_user go run ./cmd/dotagents cron --interval 30m

# --- Shell config ---
log "Configuring shell aliases and integrations..."
ZSHRC="$TARGET_HOME/.zshrc"

ALIASES_FILE="$TARGET_HOME/.aliases_agents"
cat > "$ALIASES_FILE" << 'ALIASES'
# Coding agents
alias cc='claude'
alias cdx='codex'

# Modern CLI replacements
alias ls='eza --icons --group-directories-first'
alias ll='eza -la --icons --git --time-style=relative'
alias lt='eza --tree --icons --level=2'
alias cat='bat --paging=never'

# Python/ML
alias py='python3'
alias ipy='ipython'
alias uvr='uv run'
alias uvp='uv pip'
ALIASES
chown "$TARGET_USER" "$ALIASES_FILE"

if [[ -f "$ZSHRC" ]]; then
  grep -q ".aliases_agents" "$ZSHRC" || printf '\n[[ -f ~/.aliases_agents ]] && source ~/.aliases_agents\n' >> "$ZSHRC"
  grep -q "zoxide init" "$ZSHRC" || printf 'eval "$(zoxide init zsh)"\n' >> "$ZSHRC"
  grep -q "fzf --zsh" "$ZSHRC" || printf 'source <(fzf --zsh)\n' >> "$ZSHRC"
fi

# --- Summary ---
echo ""
log "Setup complete. Installed:"
echo "  Core:    git, gh, node, go, python 3.12, uv, neovim, ripgrep, tmux"
echo "  CLI:     fzf, bat, eza, lazygit, zoxide, glow, nerd-font"
echo "  Shell:   oh-my-zsh, aliases, zoxide, fzf integration"
echo "  Python:  ipython, ruff (via uv tools)"
echo "  Agents:  $([ "$SKIP_CLAUDE" == false ] && echo 'Claude Code, ')$([ "$SKIP_CODEX" == false ] && echo 'Codex CLI')"
[[ "$INSTALL_HERMES" == true ]] && echo "  General: Hermes Agent"
[[ "$INSTALL_OLLAMA" == true ]] && echo "  Local:   Ollama + Qwen3.5-Coder"
echo "  Skills:  dotagents (~/Workspace/dotagents -> ~/.agents)"
echo ""
warn "Next steps:"
echo "  1. Open a new terminal (or: source ~/.zshrc)"
echo "  2. Run 'claude' and authenticate with Anthropic"
echo "  3. Run 'codex' and set OPENAI_API_KEY"
[[ "$INSTALL_HERMES" == true ]] && echo "  4. Run 'hermes setup' to configure model + messaging"
[[ "$INSTALL_OLLAMA" == true ]] && echo "  5. Open Ollama.app, then use local models from agents"
echo ""
