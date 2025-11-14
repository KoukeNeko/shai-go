package assets

import (
	_ "embed"
)

// DefaultConfigYAML contains the embedded default configuration.
//
//go:embed defaults/config.yaml
var DefaultConfigYAML []byte

// DefaultGuardrailYAML contains the embedded default guardrail rules.
//
//go:embed defaults/guardrail.yaml
var DefaultGuardrailYAML []byte
