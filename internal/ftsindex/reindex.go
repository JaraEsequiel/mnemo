package ftsindex

import (
	"github.com/JaraEsequiel/mnemo/internal/vault"
)

// Stats summarizes a reindex pass.
type Stats struct {
	Created   int
	Updated   int
	Deleted   int
	Unchanged int
	Errors    int
}

// Reindex brings the derived index in sync with the markdown on disk.
// It is incremental: a page is only re-parsed and re-indexed when its file hash
// has changed; pages whose files have disappeared are removed.
func Reindex(idx *Index, root string) (Stats, error) {
	var st Stats

	files, err := vault.WalkPages(root)
	if err != nil {
		return st, err
	}
	existing, err := idx.Hashes()
	if err != nil {
		return st, err
	}

	seen := make(map[string]bool, len(files))
	for _, abs := range files {
		p, err := vault.ParsePage(abs, root)
		if err != nil {
			st.Errors++
			continue
		}
		seen[p.RelPath] = true

		if h, ok := existing[p.RelPath]; ok && h == p.Hash {
			st.Unchanged++
			continue
		}
		_, already := existing[p.RelPath]
		if err := idx.Upsert(p); err != nil {
			st.Errors++
			continue
		}
		if already {
			st.Updated++
		} else {
			st.Created++
		}
	}

	for rel := range existing {
		if !seen[rel] {
			if err := idx.Delete(rel); err != nil {
				st.Errors++
				continue
			}
			st.Deleted++
		}
	}

	return st, nil
}
