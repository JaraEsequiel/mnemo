package llm

import (
	"context"
	"errors"
	"testing"
)

func fakeCLI(out string, err error) cliFunc {
	return func(ctx context.Context, name string, args []string, stdin string) ([]byte, error) {
		return []byte(out), err
	}
}

func TestClaudeJudgeParsesEnvelope(t *testing.T) {
	env := `{"result":"{\"relation\":\"supersedes\",\"reason\":\"OAuth replaces JWT\",\"confidence\":0.9}"}`
	r := &ClaudeRunner{run: fakeCLI(env, nil)}
	v, err := r.Judge(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if v.Relation != "supersedes" || v.Confidence != 0.9 || v.Reason == "" {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestClaudeJudgeStripsFences(t *testing.T) {
	env := "{\"result\":\"```json\\n{\\\"relation\\\":\\\"conflicts_with\\\",\\\"reason\\\":\\\"both claim the store\\\",\\\"confidence\\\":0.8}\\n```\"}"
	r := &ClaudeRunner{run: fakeCLI(env, nil)}
	v, err := r.Judge(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if v.Relation != "conflicts_with" {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestJudgeRejectsUnknownRelation(t *testing.T) {
	env := `{"result":"{\"relation\":\"bogus\",\"reason\":\"x\",\"confidence\":1}"}`
	r := &ClaudeRunner{run: fakeCLI(env, nil)}
	if _, err := r.Judge(context.Background(), "p"); !errors.Is(err, ErrUnknownRelation) {
		t.Fatalf("err = %v", err)
	}
}

func TestNewRunner(t *testing.T) {
	if _, err := NewRunner("claude"); err != nil {
		t.Error(err)
	}
	if _, err := NewRunner(""); !errors.Is(err, ErrInvalidRunner) {
		t.Errorf("empty: %v", err)
	}
	if _, err := NewRunner("gpt"); !errors.Is(err, ErrInvalidRunner) {
		t.Errorf("unknown: %v", err)
	}
}
