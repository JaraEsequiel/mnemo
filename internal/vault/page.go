// Package vault models a mnemo vault: markdown pages are the source of truth.
// Everything here reads from disk; nothing in this package writes the derived
// search index (that lives in package ftsindex).
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Page is a parsed markdown page: frontmatter metadata + body + derived fields.
type Page struct {
	RelPath     string   // path relative to the vault root, slash-separated
	Folder      string   // top-level folder (decisions, entities, ...) or "" at root
	Slug        string   // stable id; frontmatter slug or filename stem
	Title       string   // frontmatter title, else first H1, else slug
	Type        string   // page type (decision, entity, concept, ...)
	Tags        []string // context tags
	Description string   // one-liner; feeds the per-folder index.md
	Created     string
	Updated     string
	ReviewAfter string   // optional lifecycle hint
	Links       []string   // outbound slugs (frontmatter links + [[wikilinks]])
	Relations   []Relation // typed, reasoned edges from the managed block
	Body        string     // markdown body after frontmatter
	Hash        string     // sha256 of the raw file — drives incremental reindex
	ModTime     int64      // file mtime (unix seconds) — recency signal
}

type frontmatter struct {
	Slug        string   `yaml:"slug"`
	Title       string   `yaml:"title"`
	Type        string   `yaml:"type"`
	Tags        []string `yaml:"tags"`
	Description string   `yaml:"description"`
	Created     string   `yaml:"created"`
	Updated     string   `yaml:"updated"`
	ReviewAfter string   `yaml:"review_after"`
	Links       []string `yaml:"links"`
}

var (
	wikilinkRE = regexp.MustCompile(`\[\[([^\]|#]+)`)
	headingRE  = regexp.MustCompile(`(?m)^#\s+(.+)$`)
)

// ParsePage reads and parses a markdown file at absPath, relative to vaultRoot.
func ParsePage(absPath, vaultRoot string) (*Page, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)

	rel, err := filepath.Rel(vaultRoot, absPath)
	if err != nil {
		rel = absPath
	}
	rel = filepath.ToSlash(rel)

	fm, body := splitFrontmatter(string(raw))
	var meta frontmatter
	if fm != "" {
		_ = yaml.Unmarshal([]byte(fm), &meta)
	}

	slug := strings.TrimSpace(meta.Slug)
	if slug == "" {
		base := filepath.Base(absPath)
		slug = strings.TrimSuffix(base, filepath.Ext(base))
	}

	title := strings.TrimSpace(meta.Title)
	if title == "" {
		if h := firstHeading(body); h != "" {
			title = h
		} else {
			title = slug
		}
	}

	folder := ""
	if i := strings.Index(rel, "/"); i >= 0 {
		folder = rel[:i]
	}

	links := dedupe(append(append([]string{}, meta.Links...), extractWikilinks(body)...))
	relations := ParseRelations(string(raw))

	var mtime int64
	if fi, statErr := os.Stat(absPath); statErr == nil {
		mtime = fi.ModTime().Unix()
	}

	return &Page{
		RelPath:     rel,
		Folder:      folder,
		Slug:        slug,
		Title:       title,
		Type:        strings.TrimSpace(meta.Type),
		Tags:        meta.Tags,
		Description: strings.TrimSpace(meta.Description),
		Created:     meta.Created,
		Updated:     meta.Updated,
		ReviewAfter: meta.ReviewAfter,
		Links:       links,
		Relations:   relations,
		Body:        body,
		Hash:        hex.EncodeToString(sum[:]),
		ModTime:     mtime,
	}, nil
}

// splitFrontmatter separates a leading `---`-fenced YAML block from the body.
// Returns ("", original) when there is no frontmatter.
func splitFrontmatter(s string) (fm, body string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", s
	}
	rest := s[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", s
	}
	fm = rest[:idx]
	body = rest[idx+len("\n---"):]
	// Drop the remainder of the closing fence line, then leading blank lines.
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	} else {
		body = ""
	}
	return fm, strings.TrimLeft(body, "\n")
}

func firstHeading(body string) string {
	if m := headingRE.FindStringSubmatch(body); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractWikilinks(body string) []string {
	var out []string
	for _, m := range wikilinkRE.FindAllStringSubmatch(body, -1) {
		if len(m) == 2 {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// TagsString joins tags for FTS indexing.
func (p *Page) TagsString() string { return strings.Join(p.Tags, " ") }
