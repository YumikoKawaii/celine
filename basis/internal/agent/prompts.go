package agent

import _ "embed"

//go:embed prompts/celine.md
var basePersona string

func SystemPrompt() string { return basePersona }
