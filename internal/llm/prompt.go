package llm

import "fmt"

// BuildJudgePrompt asks the model to classify how memory A relates to memory B.
func BuildJudgePrompt(aTitle, aBody, bTitle, bBody string) string {
	return fmt.Sprintf(`Judge how memory A relates to memory B in a personal knowledge base.

Pick exactly one relation (from A's point of view):
- supersedes: A replaces/obsoletes B.
- conflicts_with: A and B contradict; they cannot both be true.
- refines: A is a more specific or updated take on B (not a full replacement).
- depends_on: A requires or builds on B.
- related: topically connected, no conflict.
- not_conflict: no meaningful relationship.

Return ONLY a JSON object, no prose:
{"relation":"<one of the above>","reason":"<short why, max 160 chars>","confidence":<0.0-1.0>}

# Memory A: %s
%s

# Memory B: %s
%s`, aTitle, truncate(aBody, 2000), bTitle, truncate(bBody, 2000))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
