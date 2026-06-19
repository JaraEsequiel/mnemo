package llm

import "fmt"

// NewRunner returns a Runner for the given CLI name ("claude" | "opencode"),
// typically read from the MNEMO_AGENT_CLI environment variable.
func NewRunner(name string) (Runner, error) {
	switch name {
	case "claude":
		return NewClaudeRunner(), nil
	case "opencode":
		return NewOpenCodeRunner(), nil
	case "":
		return nil, fmt.Errorf("%w: MNEMO_AGENT_CLI is not set (use: claude, opencode)", ErrInvalidRunner)
	default:
		return nil, fmt.Errorf("%w: %q (use: claude, opencode)", ErrInvalidRunner, name)
	}
}
