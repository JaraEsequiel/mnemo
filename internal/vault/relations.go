package vault

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Relation is a typed, reasoned edge from one page to another.
// A relation ALWAYS carries a Reason — a wikilink without a "why" is not allowed.
type Relation struct {
	Type   string // one of relationVerbs
	Target string // target slug
	Reason string // why they are related (required)
}

// relationVerbs is the locked vocabulary of relation types.
var relationVerbs = map[string]bool{
	"supersedes":     true,
	"conflicts_with": true,
	"related":        true,
	"refines":        true,
	"depends_on":     true,
}

// ReverseVerb returns how a relation reads from the target's point of view,
// used to annotate the other page (e.g. X supersedes Y ⇒ Y is superseded_by X).
func ReverseVerb(verb string) string {
	switch verb {
	case "supersedes":
		return "superseded_by"
	case "refines":
		return "refined_by"
	case "depends_on":
		return "required_by"
	default:
		return verb // conflicts_with and related are symmetric
	}
}

// ValidRelationType reports whether verb is a known relation.
func ValidRelationType(verb string) bool { return relationVerbs[verb] }

const (
	relStart = "<!-- mnemo:relations -->"
	relEnd   = "<!-- /mnemo:relations -->"
)

// bullet matches "- <type> [[<slug>]] — <reason>" (— / -- / - accepted as the dash).
var relBulletRE = regexp.MustCompile(`(?m)^-\s+([a-z_]+)\s+\[\[([^\]]+)\]\]\s*(?:—|--|-)\s*(.+?)\s*$`)

var blockRE = regexp.MustCompile(`(?s)\n*` + regexp.QuoteMeta(relStart) + `.*?` + regexp.QuoteMeta(relEnd) + `\n*`)

// ParseRelations extracts relations from a page's managed block.
func ParseRelations(content string) []Relation {
	start := strings.Index(content, relStart)
	end := strings.Index(content, relEnd)
	if start < 0 || end < 0 || end < start {
		return nil
	}
	block := content[start:end]
	var out []Relation
	for _, m := range relBulletRE.FindAllStringSubmatch(block, -1) {
		if len(m) == 4 && relationVerbs[m[1]] {
			out = append(out, Relation{Type: m[1], Target: strings.TrimSpace(m[2]), Reason: strings.TrimSpace(m[3])})
		}
	}
	return out
}

// AddRelation upserts a relation into the managed block of the file at absPath,
// preserving frontmatter and prose. The reason is mandatory.
func AddRelation(absPath string, rel Relation) error {
	if strings.TrimSpace(rel.Reason) == "" {
		return errors.New("a relation must include a reason (the 'why')")
	}
	if !relationVerbs[rel.Type] {
		return fmt.Errorf("unknown relation type %q (want: supersedes, conflicts_with, related, refines, depends_on)", rel.Type)
	}
	rel.Target = strings.TrimSpace(rel.Target)
	if rel.Target == "" {
		return errors.New("a relation must have a target slug")
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")

	merged := mergeRelation(ParseRelations(content), rel)
	content = blockRE.ReplaceAllString(content, "\n")
	content = strings.TrimRight(content, "\n") + "\n\n" + renderBlock(merged) + "\n"

	return os.WriteFile(absPath, []byte(content), 0o644)
}

// mergeRelation replaces a same-(type,target) entry or appends a new one.
func mergeRelation(existing []Relation, rel Relation) []Relation {
	for i, e := range existing {
		if e.Type == rel.Type && e.Target == rel.Target {
			existing[i].Reason = rel.Reason
			return existing
		}
	}
	return append(existing, rel)
}

func renderBlock(rels []Relation) string {
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].Type != rels[j].Type {
			return rels[i].Type < rels[j].Type
		}
		return rels[i].Target < rels[j].Target
	})
	var sb strings.Builder
	sb.WriteString(relStart + "\n")
	sb.WriteString("## Related\n")
	for _, r := range rels {
		fmt.Fprintf(&sb, "- %s [[%s]] — %s\n", r.Type, r.Target, r.Reason)
	}
	sb.WriteString(relEnd)
	return sb.String()
}
