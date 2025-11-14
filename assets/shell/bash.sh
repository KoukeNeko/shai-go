# SHAI bash integration
# Source from ~/.bashrc to enable `#` natural language queries.

if [[ $- != *i* ]]; then
  return
fi

if [[ -n "$_SHAI_BASH_LOADED" ]]; then
  return
fi
_SHAI_BASH_LOADED=1

_shai_command_bin() {
  if [[ -n "$SHAI_BIN" ]]; then
    printf '%s\n' "$SHAI_BIN"
  else
    printf '%s\n' "shai"
  fi
}

_shai_handle_debug() {
  local cmd="$BASH_COMMAND"
  if [[ "$cmd" == \#* ]]; then
    local query="${cmd#\#}"
    history -d $((HISTCMD)) 2>/dev/null || true

    # Generate command and capture output (requires verbose: false in config)
    # Spinner is shown by the shai binary itself (outputs to /dev/tty)
    local generated_cmd
    generated_cmd=$("$(_shai_command_bin)" query "$query")

    if [[ -n "$generated_cmd" ]]; then
      # Put generated command in readline buffer for user to review/execute
      history -s "$generated_cmd"
      READLINE_LINE="$generated_cmd"
      READLINE_POINT=${#READLINE_LINE}
    fi
    return 1
  fi
}

_shai_prompt_command() {
  local last="$(history 1 2>/dev/null)"
  if [[ "$last" == *"#"* ]]; then
    history -d $((HISTCMD-1)) 2>/dev/null || true
  fi
}

if [[ -z "$_SHAI_DEBUG_TRAP" ]]; then
  trap '_shai_handle_debug' DEBUG
  _SHAI_DEBUG_TRAP=1
fi

case "$PROMPT_COMMAND" in
  *_shai_prompt_command*) ;;
  "")
    PROMPT_COMMAND="_shai_prompt_command"
    ;;
  *)
    PROMPT_COMMAND="_shai_prompt_command;$PROMPT_COMMAND"
    ;;
esac
