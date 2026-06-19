package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestParsePage(t *testing.T) {
	root := t.TempDir()
	abs := writeFile(t, root, "decisions/jwt-auth.md", `---
slug: jwt-auth-model
title: JWT auth model
type: decision
tags: [code, auth]
description: JWT + refresh tokens
links: [redis-sessions]
---

# JWT auth model

Body referencing [[user-store]] and [[redis-sessions]].
`)

	p, err := ParsePage(abs, root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "jwt-auth-model" {
		t.Errorf("slug = %q", p.Slug)
	}
	if p.Title != "JWT auth model" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Type != "decision" {
		t.Errorf("type = %q", p.Type)
	}
	if p.Folder != "decisions" {
		t.Errorf("folder = %q", p.Folder)
	}
	if p.RelPath != "decisions/jwt-auth.md" {
		t.Errorf("relpath = %q", p.RelPath)
	}
	if len(p.Tags) != 2 || p.Tags[0] != "code" {
		t.Errorf("tags = %v", p.Tags)
	}
	// links = frontmatter (redis-sessions) ∪ wikilinks (user-store, redis-sessions), deduped
	want := map[string]bool{"redis-sessions": true, "user-store": true}
	if len(p.Links) != len(want) {
		t.Errorf("links = %v", p.Links)
	}
	for _, l := range p.Links {
		if !want[l] {
			t.Errorf("unexpected link %q", l)
		}
	}
	if p.Hash == "" {
		t.Error("hash empty")
	}
}

func TestSlugFallbackToFilename(t *testing.T) {
	root := t.TempDir()
	abs := writeFile(t, root, "notes/some-note.md", "# A note\n\nNo frontmatter here.\n")
	p, err := ParsePage(abs, root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "some-note" {
		t.Errorf("slug = %q, want some-note", p.Slug)
	}
	if p.Title != "A note" {
		t.Errorf("title = %q, want 'A note' (first H1)", p.Title)
	}
}

func TestWalkPagesSkipsIndexAndIgnored(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "decisions/a.md", "# a")
	writeFile(t, root, "decisions/index.md", "# index")
	writeFile(t, root, ".mnemo/note.md", "# hidden")
	writeFile(t, root, "log.md", "# log")

	files, err := WalkPages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "a.md" {
		t.Errorf("unexpected file %q", files[0])
	}
}
