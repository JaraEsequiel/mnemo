// Package llm runs an external agent CLI (claude / opencode) as a contradiction
// judge for headless flows (e.g. `mnemo lint --semantic`). When an interactive
// agent is present it should judge directly via the wiki_relate MCP tool; this
// package is the no-agent fallback. $0 on a Pro/Max subscription.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Verdict is the judge's classification of how memory A relates to memory B.
type Verdict struct {
	Relation   string  `json:"relation"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// Runner judges a prompt and returns a structured Verdict.
type Runner interface {
	Judge(ctx context.Context, prompt string) (Verdict, error)
}

// ValidRelations is the locked vocabulary; "not_conflict" means no edge.
var ValidRelations = map[string]bool{
	"supersedes":     true,
	"conflicts_with": true,
	"related":        true,
	"refines":        true,
	"depends_on":     true,
	"not_conflict":   true,
}

var (
	ErrInvalidRunner   = errors.New("invalid runner name")
	ErrInvalidJSON     = errors.New("invalid judge JSON")
	ErrUnknownRelation = errors.New("unknown relation")
	ErrCLINotInstalled = errors.New("agent CLI not installed")
)

var fenceRE = regexp.MustCompile("(?s)```[a-zA-Z]*\\n?(.+?)\\n?```")

// parseVerdict extracts a Verdict from a model's raw text output: it strips
// markdown fences, isolates the first JSON object, and validates the relation.
func parseVerdict(raw string) (Verdict, error) {
	s := strings.TrimSpace(raw)
	if m := fenceRE.FindStringSubmatch(s); len(m) == 2 {
		s = strings.TrimSpace(m[1])
	}
	// Isolate the outermost JSON object.
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := strings.LastIndexByte(s, '}'); j >= i {
			s = s[i : j+1]
		}
	}
	var v Verdict
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return Verdict{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	v.Relation = strings.TrimSpace(strings.ToLower(v.Relation))
	if !ValidRelations[v.Relation] {
		return Verdict{}, fmt.Errorf("%w: %q", ErrUnknownRelation, v.Relation)
	}
	return v, nil
}
