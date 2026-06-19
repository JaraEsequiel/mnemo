// Package watcher re-indexes a mnemo vault whenever its markdown changes.
package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JaraEsequiel/mnemo/internal/ftsindex"
	"github.com/JaraEsequiel/mnemo/internal/vault"

	"github.com/fsnotify/fsnotify"
)

var ignoredDirs = map[string]bool{
	".mnemo": true, ".obsidian": true, ".git": true, "node_modules": true,
}

// Run watches root and reindexes (FTS + folder index.md) on markdown changes,
// debounced by the given interval, until ctx is cancelled. It runs one sync
// immediately on start.
func Run(ctx context.Context, root string, idx *ftsindex.Index, debounce time.Duration) error {
	if debounce <= 0 {
		debounce = 500 * time.Millisecond
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watcher: %w", err)
	}
	defer w.Close()

	if err := addDirs(w, root); err != nil {
		return err
	}

	sync(root, idx) // first cycle runs immediately

	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			// Track newly created directories so their files get watched too.
			if event.Op&fsnotify.Create != 0 {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					_ = addDirs(w, event.Name)
				}
			}
			if !strings.HasSuffix(strings.ToLower(event.Name), ".md") {
				continue
			}
			// Debounce: (re)start the timer on each relevant event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(debounce)
			timerC = timer.C

		case <-timerC:
			sync(root, idx)
			timerC = nil

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Printf("mnemo watch: %v", err)
		}
	}
}

func sync(root string, idx *ftsindex.Index) {
	fts, err := ftsindex.Reindex(idx, root)
	if err != nil {
		log.Printf("mnemo watch: reindex: %v", err)
		return
	}
	im, err := vault.GenerateIndexes(root)
	if err != nil {
		log.Printf("mnemo watch: indexes: %v", err)
	}
	log.Printf("[%s] sync: created=%d updated=%d deleted=%d unchanged=%d  indexes_written=%d",
		time.Now().Format(time.RFC3339), fts.Created, fts.Updated, fts.Deleted, fts.Unchanged, im.Written)
}

func addDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if ignoredDirs[d.Name()] {
			return fs.SkipDir
		}
		_ = w.Add(path)
		return nil
	})
}
