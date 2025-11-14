# SHAI zsh integration
# Sources this file from ~/.zshrc to enable `#` natural language queries.

if [[ -z "$ZSH_VERSION" || ! -o interactive ]]; then
  return
fi

if [[ -n "$_SHAI_ZSH_LOADED" ]]; then
  return
fi
readonly _SHAI_ZSH_LOADED=1

_shai_warn_conflicts() {
  if [[ -n "$ZSH" && "$ZSH" == *oh-my-zsh* && -z "$_SHAI_ZSH_CONFLICT_WARNED" ]]; then
    echo "SHAI: oh-my-zsh detected. Ensure SHAI loads after your framework plugins." >&2
    export _SHAI_ZSH_CONFLICT_WARNED=1
  fi
}

_shai_command_bin() {
  if [[ -n "$SHAI_BIN" ]]; then
    echo "$SHAI_BIN"
  else
    echo "shai"
  fi
}

if (( ! $+functions[_shai_accept_line_original] )); then
  zle -A accept-line _shai_accept_line_original 2>/dev/null
fi

function _shai_spinner_update() {
  local frame="$1"
  zle -M "$frame"
}

function _shai_accept_line() {
  local buffer="$BUFFER"
  if [[ "$buffer" == \#* ]]; then
    local query="${buffer#\#}"
    BUFFER=""
    zle reset-prompt

    # Show initial spinner
    local frames=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
    local frame_idx=0

    # Start background animation
    local animation_running=1
    (
      while ((animation_running)); do
        print -n "\r${frames[$((frame_idx % 10))]}" > /dev/tty
        sleep 0.08
        ((frame_idx++))
      done
      print -n "\r\033[K" > /dev/tty  # Clear line
    ) &
    local animation_pid=$!

    # Generate command and capture output (requires verbose: false in config)
    local generated_cmd
    generated_cmd=$(command "$(_shai_command_bin)" query "$query" 2>/dev/null)

    # Stop animation
    animation_running=0
    kill $animation_pid 2>/dev/null
    wait $animation_pid 2>/dev/null

    if [[ -n "$generated_cmd" ]]; then
      # Put generated command in buffer for user to review/execute
      BUFFER="$generated_cmd"
      zle reset-prompt
      zle end-of-line
    fi
    return 0
  fi
  if (( $+functions[_shai_accept_line_original] )); then
    zle _shai_accept_line_original
  else
    zle .accept-line
  fi
}

_shai_warn_conflicts
zle -N _shai_accept_line
bindkey "^M" _shai_accept_line
bindkey -M viins "^M" _shai_accept_line
bindkey -M vicmd "^M" _shai_accept_line
