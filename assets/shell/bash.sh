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

    # Start background process for animation
    local animation_running=1
    local animation_pid
    (
      local frames=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
      local i=0
      while ((animation_running)); do
        printf "\r${frames[$((i % 10))]}" > /dev/tty
        sleep 0.08
        ((i++))
      done
      printf "\r\033[K" > /dev/tty  # Clear line
    ) &
    animation_pid=$!

    # Generate command and capture output (requires verbose: false in config)
    local generated_cmd
    generated_cmd=$("$(_shai_command_bin)" query "$query" 2>/dev/null)

    # Stop animation
    animation_running=0
    kill $animation_pid 2>/dev/null
    wait $animation_pid 2>/dev/null

    if [[ -n "$generated_cmd" ]]; then
      # Put generated command in readline buffer for user to review/execute
      history -s "$generated_cmd"
      # Use bind to insert the command into the current line
      bind '"\er": redraw-current-line'
      bind '"\e^": magic-space'
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
