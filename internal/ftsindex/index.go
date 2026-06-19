// Package ftsindex maintains the derived SQLite + FTS5 search index over a
// mnemo vault. The index is disposable: it can always be rebuilt from the
// markdown, which is the source of truth.
package ftsindex

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JaraEsequiel/mnemo/internal/vault"

	_ "modernc.org/sqlite"
)

// Index wraps the derived SQLite database.
type Index struct{ db *sql.DB }

// Result is one search hit.
type Result struct {
	RelPath string
	Slug    string
	Type    string
	Title   string
	Snippet string
	Rank    float64
}

// schemaVersion is bumped whenever the derived schema (esp. FTS tokenizer)
// changes. On mismatch the derived tables are dropped and rebuilt from the
// markdown on the next reindex — safe because the index is disposable.
const schemaVersion = 4

const createMeta = `CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);`

const createRelations = `
CREATE TABLE IF NOT EXISTS relations (
  source_slug TEXT,
  type        TEXT,
  target_slug TEXT,
  reason      TEXT,
  PRIMARY KEY (source_slug, type, target_slug)
);
CREATE INDEX IF NOT EXISTS idx_rel_target ON relations(target_slug);`

const createPages = `
CREATE TABLE IF NOT EXISTS pages (
  rel_path    TEXT PRIMARY KEY,
  slug        TEXT,
  type        TEXT,
  tags        TEXT,
  title       TEXT,
  description TEXT,
  hash        TEXT,
  updated_at  TEXT,
  mtime       INTEGER DEFAULT 0
);`

const createLinks = `
CREATE TABLE IF NOT EXISTS links (
  source_slug TEXT,
  target_slug TEXT,
  PRIMARY KEY (source_slug, target_slug)
);
CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_slug);`

// The porter stemmer lets "distraction" match "distractions"; unicode61 folds
// case and diacritics.
const createFTS = `
CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5 (
  rel_path UNINDEXED,
  slug,
  type,
  tags,
  title,
  description,
  body,
  tokenize = 'porter unicode61'
);`

// Open opens (creating if needed) the index at dbPath.
func Open(dbPath string) (*Index, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("ftsindex: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ftsindex: open: %w", err)
	}
	db.SetMaxOpenConns(1) // single writer, like engram
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("ftsindex: pragma %q: %w", pragma, err)
		}
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ftsindex: schema: %w", err)
	}
	return &Index{db: db}, nil
}

// migrate ensures the derived tables exist at the current schemaVersion,
// dropping and rebuilding them when the stored version differs.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(createMeta); err != nil {
		return err
	}
	var stored int
	var raw string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&raw); err == nil {
		fmt.Sscanf(raw, "%d", &stored)
	}
	if stored != schemaVersion {
		for _, drop := range []string{
			`DROP TABLE IF EXISTS pages_fts`,
			`DROP TABLE IF EXISTS pages`,
			`DROP TABLE IF EXISTS relations`,
			`DROP TABLE IF EXISTS links`,
		} {
			if _, err := db.Exec(drop); err != nil {
				return err
			}
		}
	}
	for _, create := range []string{createPages, createFTS, createRelations, createLinks} {
		if _, err := db.Exec(create); err != nil {
			return err
		}
	}
	_, err := db.Exec(
		`INSERT INTO meta(key, value) VALUES('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		fmt.Sprintf("%d", schemaVersion))
	return err
}

// Close closes the underlying database.
func (i *Index) Close() error { return i.db.Close() }

// Hashes returns rel_path → hash for every indexed page (for incremental sync).
func (i *Index) Hashes() (map[string]string, error) {
	rows, err := i.db.Query(`SELECT rel_path, hash FROM pages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var rel, h string
		if err := rows.Scan(&rel, &h); err != nil {
			return nil, err
		}
		out[rel] = h
	}
	return out, rows.Err()
}

// Upsert inserts or replaces a page in both the metadata table and the FTS index.
func (i *Index) Upsert(p *vault.Page) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM pages_fts WHERE rel_path = ?`, p.RelPath); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO pages_fts (rel_path, slug, type, tags, title, description, body)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.RelPath, p.Slug, p.Type, p.TagsString(), p.Title, p.Description, p.Body,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO pages (rel_path, slug, type, tags, title, description, hash, updated_at, mtime)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(rel_path) DO UPDATE SET
		   slug=excluded.slug, type=excluded.type, tags=excluded.tags,
		   title=excluded.title, description=excluded.description,
		   hash=excluded.hash, updated_at=excluded.updated_at, mtime=excluded.mtime`,
		p.RelPath, p.Slug, p.Type, p.TagsString(), p.Title, p.Description, p.Hash, p.Updated, p.ModTime,
	); err != nil {
		return err
	}

	// Outbound links (for inbound-link signals), derived from the markdown.
	if _, err := tx.Exec(`DELETE FROM links WHERE source_slug = ?`, p.Slug); err != nil {
		return err
	}
	for _, target := range p.Links {
		if target == p.Slug {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO links (source_slug, target_slug) VALUES (?, ?)`,
			p.Slug, target,
		); err != nil {
			return err
		}
	}

	// Relations are derived from the page's managed block (markdown is truth).
	if _, err := tx.Exec(`DELETE FROM relations WHERE source_slug = ?`, p.Slug); err != nil {
		return err
	}
	for _, r := range p.Relations {
		if _, err := tx.Exec(
			`INSERT INTO relations (source_slug, type, target_slug, reason)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(source_slug, type, target_slug) DO UPDATE SET reason=excluded.reason`,
			p.Slug, r.Type, r.Target, r.Reason,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Relation is one annotated edge as stored in the index.
type Relation struct {
	Type   string // from the source's point of view (or reverse, for incoming)
	Slug   string // the other page's slug
	Reason string
}

// RelationsFor returns relations annotated from slug's point of view: outgoing
// relations as-is, and incoming relations rewritten with their reverse verb.
func (i *Index) RelationsFor(slug string) ([]Relation, error) {
	var out []Relation
	rows, err := i.db.Query(`SELECT type, target_slug, reason FROM relations WHERE source_slug = ?`, slug)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.Type, &r.Slug, &r.Reason); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	rows.Close()

	rows, err = i.db.Query(`SELECT type, source_slug, reason FROM relations WHERE target_slug = ?`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var verb, src, reason string
		if err := rows.Scan(&verb, &src, &reason); err != nil {
			return nil, err
		}
		out = append(out, Relation{Type: vault.ReverseVerb(verb), Slug: src, Reason: reason})
	}
	return out, rows.Err()
}

// Candidates returns pages lexically similar to slug (by its title/description/
// tags) that are not slug itself and not already related — for contradiction
// surfacing. The caller (an LLM agent) then judges and records verdicts.
func (i *Index) Candidates(slug string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}
	var title, desc, tags string
	if err := i.db.QueryRow(
		`SELECT title, description, tags FROM pages WHERE slug = ? LIMIT 1`, slug,
	).Scan(&title, &desc, &tags); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no page with slug %q", slug)
		}
		return nil, err
	}

	excluded := map[string]bool{slug: true}
	rels, err := i.RelationsFor(slug)
	if err != nil {
		return nil, err
	}
	for _, r := range rels {
		excluded[r.Slug] = true
	}

	// OR semantics: a candidate need only share *some* significant terms.
	hits, err := i.searchMatch(sanitizeFTSOr(strings.Join([]string{title, desc, tags}, " ")), "", limit+len(excluded)+5)
	if err != nil {
		return nil, err
	}
	var out []Result
	for _, h := range hits {
		if excluded[h.Slug] {
			continue
		}
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Delete removes a page and its derived edges (relations/links it sourced).
func (i *Index) Delete(relPath string) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var slug string
	_ = tx.QueryRow(`SELECT slug FROM pages WHERE rel_path = ?`, relPath).Scan(&slug)

	if _, err := tx.Exec(`DELETE FROM pages_fts WHERE rel_path = ?`, relPath); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM pages WHERE rel_path = ?`, relPath); err != nil {
		return err
	}
	if slug != "" {
		if _, err := tx.Exec(`DELETE FROM relations WHERE source_slug = ?`, slug); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM links WHERE source_slug = ?`, slug); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Signal carries the hot-cache promotion signals for one page.
type Signal struct {
	Slug      string
	Title     string
	Type      string
	Inbound   int   // inbound wikilinks + inbound relations
	MtimeUnix int64 // file recency
}

// PageSignals returns per-page signals ordered by inbound count desc.
func (i *Index) PageSignals() ([]Signal, error) {
	rows, err := i.db.Query(`
SELECT p.slug, p.title, p.type, p.mtime,
       (SELECT count(*) FROM links l WHERE l.target_slug = p.slug)
     + (SELECT count(*) FROM relations r WHERE r.target_slug = p.slug) AS inbound
FROM pages p
ORDER BY inbound DESC, p.mtime DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Signal
	for rows.Next() {
		var s Signal
		if err := rows.Scan(&s.Slug, &s.Title, &s.Type, &s.MtimeUnix, &s.Inbound); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Search runs an FTS5 query (AND of terms), optionally filtered by type, ranked by BM25.
func (i *Index) Search(query, typ string, limit int) ([]Result, error) {
	return i.searchMatch(sanitizeFTS(query), typ, limit)
}

// searchMatch runs a raw FTS5 MATCH expression.
func (i *Index) searchMatch(match, typ string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	if strings.TrimSpace(match) == "" {
		return nil, nil
	}

	// Note: FTS5 auxiliary functions (snippet/bm25) require the FTS table's real
	// name as their first argument — the table cannot be aliased when used this way.
	sb := strings.Builder{}
	sb.WriteString(`
SELECT pages_fts.rel_path, p.slug, p.type, p.title,
       snippet(pages_fts, 6, '[', ']', '…', 12) AS snip,
       bm25(pages_fts) AS rank
FROM pages_fts
JOIN pages p ON p.rel_path = pages_fts.rel_path
WHERE pages_fts MATCH ?`)
	args := []any{match}
	if strings.TrimSpace(typ) != "" {
		sb.WriteString(` AND p.type = ?`)
		args = append(args, typ)
	}
	sb.WriteString(` ORDER BY rank LIMIT ?`)
	args = append(args, limit)

	rows, err := i.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.RelPath, &r.Slug, &r.Type, &r.Title, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PageMeta is a row from the metadata table (no body).
type PageMeta struct {
	RelPath     string
	Slug        string
	Type        string
	Tags        string
	Title       string
	Description string
}

// PathForSlug returns the rel_path of the page with the given slug, or "" if
// none. When multiple pages share a slug, the lexically-first rel_path wins.
func (i *Index) PathForSlug(slug string) (string, error) {
	var rel string
	err := i.db.QueryRow(
		`SELECT rel_path FROM pages WHERE slug = ? ORDER BY rel_path LIMIT 1`, slug,
	).Scan(&rel)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return rel, err
}

// ListFolder returns the catalog (slug + description) for one folder, by slug.
func (i *Index) ListFolder(folder string) ([]PageMeta, error) {
	rows, err := i.db.Query(
		`SELECT rel_path, slug, type, tags, title, description
		 FROM pages WHERE rel_path LIKE ? ORDER BY slug`, folder+"/%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageMeta
	for rows.Next() {
		var m PageMeta
		if err := rows.Scan(&m.RelPath, &m.Slug, &m.Type, &m.Tags, &m.Title, &m.Description); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Folders returns each top-level folder and its page count.
func (i *Index) Folders() (map[string]int, error) {
	rows, err := i.db.Query(
		`SELECT substr(rel_path, 1, instr(rel_path, '/') - 1) AS folder, count(*)
		 FROM pages WHERE instr(rel_path, '/') > 0 GROUP BY folder ORDER BY folder`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var folder string
		var n int
		if err := rows.Scan(&folder, &n); err != nil {
			return nil, err
		}
		out[folder] = n
	}
	return out, rows.Err()
}

// sanitizeFTS wraps each whitespace-separated term in double quotes so that
// user input cannot trip FTS5's operator syntax (mirrors engram's approach).
func sanitizeFTS(q string) string {
	fields := strings.Fields(q)
	for i, f := range fields {
		fields[i] = `"` + strings.ReplaceAll(f, `"`, "") + `"`
	}
	return strings.Join(fields, " ")
}

// sanitizeFTSOr builds an OR expression over the distinct significant terms
// (length ≥ 3, deduped) — used for candidate detection where matching *some*
// terms is enough.
func sanitizeFTSOr(q string) string {
	seen := map[string]bool{}
	var terms []string
	for _, f := range strings.Fields(strings.ToLower(q)) {
		f = strings.Trim(f, `".,;:!?()[]{}/\`)
		if len(f) < 3 || seen[f] {
			continue
		}
		seen[f] = true
		terms = append(terms, `"`+strings.ReplaceAll(f, `"`, "")+`"`)
	}
	return strings.Join(terms, " OR ")
}
