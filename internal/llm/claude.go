package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// cliFunc shells out to an external command, feeding stdin, returning stdout.
// Swappable in tests so no real process is spawned.
type cliFunc func(ctx context.Context, name string, args []string, stdin string) ([]byte, error)

// ClaudeRunner judges via the `claude` CLI:
//
//	claude -p --output-format json --model haiku --max-turns 1
type ClaudeRunner struct{ run cliFunc }

// NewClaudeRunner returns a runner using the real exec implementation.
func NewClaudeRunner() *ClaudeRunner { return &ClaudeRunner{run: defaultRunCLI} }

// claudeEnvelope is the wrapper `claude --output-format json` returns.
type claudeEnvelope struct {
	Result string `json:"result"`
}

// Judge sends prompt to the claude CLI and parses the inner Verdict JSON.
func (r *ClaudeRunner) Judge(ctx context.Context, prompt string) (Verdict, error) {
	args := []string{"-p", "--output-format", "json", "--model", "haiku", "--max-turns", "1"}
	out, err := r.run(ctx, "claude", args, prompt)
	if err != nil {
		return Verdict{}, err
	}
	var env claudeEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		// Some setups print the raw result without the envelope — try direct parse.
		return parseVerdict(string(out))
	}
	return parseVerdict(env.Result)
}

// OpenCodeRunner judges via the `opencode` CLI (best-effort: expects the model
// to print the JSON verdict to stdout).
type OpenCodeRunner struct{ run cliFunc }

// NewOpenCodeRunner returns a runner using the real exec implementation.
func NewOpenCodeRunner() *OpenCodeRunner { return &OpenCodeRunner{run: defaultRunCLI} }

// Judge sends prompt to the opencode CLI and parses a Verdict from stdout.
func (r *OpenCodeRunner) Judge(ctx context.Context, prompt string) (Verdict, error) {
	out, err := r.run(ctx, "opencode", []string{"run"}, prompt)
	if err != nil {
		return Verdict{}, err
	}
	return parseVerdict(string(out))
}

func defaultRunCLI(ctx context.Context, name string, args []string, stdin string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("CLI %q exited %d: %s", name, ee.ExitCode(), string(ee.Stderr))
		}
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("%w: %q", ErrCLINotInstalled, name)
		}
		return nil, err
	}
	return out, nil
}
