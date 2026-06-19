package ftsindex

import (
	"path/filepath"
	"testing"

	"github.com/JaraEsequiel/mnemo/internal/vault"
)

func newTestIndex(t *testing.T) *Index {
	t.Helper()
	idx, err := Open(filepath.Join(t.TempDir(), "wiki.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func mustUpsert(t *testing.T, idx *Index, p *vault.Page) {
	t.Helper()
	if err := idx.Upsert(p); err != nil {
		t.Fatal(err)
	}
}

func TestSearchRanksAndFilters(t *testing.T) {
	idx := newTestIndex(t)
	mustUpsert(t, idx, &vault.Page{
		RelPath: "decisions/jwt.md", Slug: "jwt-auth", Type: "decision",
		Title: "JWT auth", Description: "JWT with redis sessions",
		Body: "Use JWT access tokens stored in redis.", Hash: "h1",
	})
	mustUpsert(t, idx, &vault.Page{
		RelPath: "ideas/dash.md", Slug: "focus-dashboard", Type: "idea",
		Title: "Focus dashboard", Description: "A dashboard for focus",
		Body: "A morning dashboard app.", Hash: "h2",
	})

	res, err := idx.Search("redis", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Slug != "jwt-auth" {
		t.Fatalf("search redis = %+v", res)
	}

	// type filter
	res, err = idx.Search("dashboard", "idea", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Slug != "focus-dashboard" {
		t.Fatalf("typed search = %+v", res)
	}
	res, _ = idx.Search("dashboard", "decision", 10)
	if len(res) != 0 {
		t.Fatalf("type filter should exclude: %+v", res)
	}
}

func TestPorterStemming(t *testing.T) {
	idx := newTestIndex(t)
	mustUpsert(t, idx, &vault.Page{
		RelPath: "concepts/focus.md", Slug: "focus", Type: "concept",
		Title: "Focus", Description: "blocks distractions",
		Body: "A method that blocks distractions during deep work.", Hash: "h",
	})
	// singular query should match the plural in the body via the porter stemmer
	res, err := idx.Search("distraction", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Slug != "focus" {
		t.Fatalf("stemming search = %+v", res)
	}
}

func TestIncrementalHashesAndDelete(t *testing.T) {
	idx := newTestIndex(t)
	p := &vault.Page{RelPath: "a/x.md", Slug: "x", Title: "X", Body: "hello", Hash: "h1"}
	mustUpsert(t, idx, p)

	h, err := idx.Hashes()
	if err != nil {
		t.Fatal(err)
	}
	if h["a/x.md"] != "h1" {
		t.Fatalf("hashes = %v", h)
	}

	if got, _ := idx.PathForSlug("x"); got != "a/x.md" {
		t.Fatalf("PathForSlug = %q", got)
	}

	if err := idx.Delete("a/x.md"); err != nil {
		t.Fatal(err)
	}
	res, _ := idx.Search("hello", "", 10)
	if len(res) != 0 {
		t.Fatalf("expected no results after delete: %+v", res)
	}
}
