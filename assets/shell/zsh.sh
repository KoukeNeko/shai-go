# SHAI zsh integration
# Sources this file from ~/.zshrc to enable `#` natural language queries.

if [[ -z "$ZSH_VERSION" ]]; then
  return
fi

if [[ ! -o interactive ]]; then
  return
fi

_shai_command_bin() {
  if [[ -n "$SHAI_BIN" ]]; then
    echo "$SHAI_BIN"
  else
    echo "shai"
  fi
}

function _shai_accept_line() {
  local buffer="$BUFFER"
  if [[ "$buffer" == \#* ]]; then
    local query="${buffer#\#}"
    BUFFER=""
    zle reset-prompt
    command "$(_shai_command_bin)" query "$query"
    return 0
  fi
  zle .accept-line
}

if [[ "${widgets[accept-line]}" != "_shai_accept_line" ]]; then
  zle -N accept-line _shai_accept_line
fi
