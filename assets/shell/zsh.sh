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

function _shai_accept_line() {
  local buffer="$BUFFER"
  if [[ "$buffer" == \#* ]]; then
    local query="${buffer#\#}"

    # Show loading message
    zle -M "⠿ Generating command..."

    # Generate command and capture output (requires verbose: false in config)
    local generated_cmd
    generated_cmd=$(command "$(_shai_command_bin)" query "$query" 2>/dev/null)

    if [[ -n "$generated_cmd" ]]; then
      # Clear the input line and put generated command in buffer
      BUFFER="$generated_cmd"
      zle reset-prompt
      zle end-of-line
    else
      # Clear if failed
      BUFFER=""
      zle reset-prompt
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
