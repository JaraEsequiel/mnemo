package vault

import (
	"os"
	"strings"
	"testing"
)

func TestAddRelationRoundTrip(t *testing.T) {
	root := t.TempDir()
	abs := writeFile(t, root, "decisions/oauth.md", `---
slug: oauth-migration
title: OAuth migration
---

# OAuth migration

Body prose that must be preserved.
`)

	if err := AddRelation(abs, Relation{Type: "supersedes", Target: "jwt-auth", Reason: "moved to OAuth"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(abs)
	content := string(data)

	if !strings.Contains(content, "Body prose that must be preserved.") {
		t.Error("prose was clobbered")
	}
	if !strings.Contains(content, "slug: oauth-migration") {
		t.Error("frontmatter was clobbered")
	}
	rels := ParseRelations(content)
	if len(rels) != 1 || rels[0].Type != "supersedes" || rels[0].Target != "jwt-auth" || rels[0].Reason != "moved to OAuth" {
		t.Fatalf("relations = %+v", rels)
	}

	// Upsert same (type,target) with a new reason → still one relation, updated.
	if err := AddRelation(abs, Relation{Type: "supersedes", Target: "jwt-auth", Reason: "clearer reason"}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(abs)
	rels = ParseRelations(string(data))
	if len(rels) != 1 || rels[0].Reason != "clearer reason" {
		t.Fatalf("after update relations = %+v", rels)
	}

	// A second distinct relation appends.
	if err := AddRelation(abs, Relation{Type: "related", Target: "redis-sessions", Reason: "shares the session store"}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(abs)
	if got := ParseRelations(string(data)); len(got) != 2 {
		t.Fatalf("expected 2 relations, got %+v", got)
	}
}

func TestAddRelationRequiresReasonAndValidType(t *testing.T) {
	root := t.TempDir()
	abs := writeFile(t, root, "a.md", "# a\n")

	if err := AddRelation(abs, Relation{Type: "supersedes", Target: "x", Reason: ""}); err == nil {
		t.Error("expected error for missing reason")
	}
	if err := AddRelation(abs, Relation{Type: "bogus", Target: "x", Reason: "y"}); err == nil {
		t.Error("expected error for invalid type")
	}
}
