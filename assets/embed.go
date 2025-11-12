package assets

import _ "embed"

//go:embed shell/zsh.sh
var ZshHook string

//go:embed shell/bash.sh
var BashHook string
