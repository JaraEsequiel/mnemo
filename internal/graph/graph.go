// Package graph writes an opinionated Obsidian graph-view config for a mnemo
// vault: color groups by page-type folder so the knowledge graph reads at a glance.
package graph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Mode controls how Write handles an existing graph.json.
type Mode string

const (
	Preserve Mode = "preserve" // write only if absent (default)
	Force    Mode = "force"    // always overwrite
	Skip     Mode = "skip"     // never touch
)

// ParseMode validates a mode string.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case Preserve, Force, Skip:
		return Mode(s), nil
	default:
		return "", fmt.Errorf("invalid graph mode %q (want preserve|force|skip)", s)
	}
}

// Color groups by folder, tuned for the mnemo flat-by-type layout.
const template = `{
  "collapse-filter": true,
  "search": "",
  "showTags": false,
  "showAttachments": false,
  "hideUnresolved": false,
  "showOrphans": true,
  "collapse-color-groups": false,
  "colorGroups": [
    { "query": "path:decisions", "color": { "a": 1, "rgb": 31 } },
    { "query": "path:ideas",     "color": { "a": 1, "rgb": 16745728 } },
    { "query": "path:concepts",  "color": { "a": 1, "rgb": 43818 } },
    { "query": "path:entities",  "color": { "a": 1, "rgb": 10233776 } },
    { "query": "path:projects",  "color": { "a": 1, "rgb": 16766720 } },
    { "query": "path:sources",   "color": { "a": 1, "rgb": 8421504 } }
  ],
  "collapse-display": false,
  "showArrow": false,
  "textFadeMultiplier": 0,
  "nodeSizeMultiplier": 1,
  "lineSizeMultiplier": 1,
  "collapse-forces": false,
  "centerStrength": 0.5,
  "repelStrength": 12,
  "linkStrength": 0.7,
  "linkDistance": 200,
  "scale": 1
}
`

// Write places the graph config at {vaultPath}/.obsidian/graph.json per mode.
func Write(vaultPath string, mode Mode) error {
	if mode == Skip {
		return nil
	}
	dir := filepath.Join(vaultPath, ".obsidian")
	path := filepath.Join(dir, "graph.json")

	if mode == Preserve {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(template), 0o644)
}
