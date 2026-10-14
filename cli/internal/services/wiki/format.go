package wiki

import (
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Format of one page (p4-debts, verify.spec of the workflow `spec`): the
// checks of wiki_check that bear on a page alone — its frontmatter
// (checkFrontmatter: present, readable, `updated` a date, `confidence` one
// of confirmed | mixed | uncertain and consistent with its tags, `agents`
// set) and its confidence tags (a line that reads like a tag but is not
// one) —, and, stricter than wiki_check, the keys of the frontmatter: only
// those of doc-wiki-protocol (`synced` belongs to index.md, written by
// oh).

// IssueFrontmatterKey is a frontmatter key the format does not know.
const IssueFrontmatterKey = "frontmatter_key"

// FrontmatterKeys are the keys of the frontmatter of a page
// (doc-wiki-protocol).
var FrontmatterKeys = []string{"updated", "confidence", "agents"}

// FormatIssues returns the format issues of the page rel (relative to
// docs/wiki/) whose text is text, errors and warnings alike.
func FormatIssues(rel, text string) []Issue {
	p := NewPage(rel, text)
	var out []Issue
	add := func(i Issue) { out = append(out, i) }
	(&Wiki{}).checkFrontmatter(p, add)
	checkFrontmatterKeys(p, SeverityError, add)
	checkTags(p, func(i Issue) {
		if i.Kind == IssueTag {
			add(i)
		}
	})
	return out
}

// checkFrontmatterKeys reports the keys of the frontmatter of p that the
// format does not know, with severity (p5-fixes: wiki_check warns about
// them on every page but the specs, verify.spec refuses them).
func checkFrontmatterKeys(p Page, severity string, add func(Issue)) {
	for _, k := range p.Frontmatter.Keys {
		if !slices.Contains(FrontmatterKeys, k) && (k != "synced" || p.Path != IndexPage) {
			add(Issue{Page: p.Path, Line: 1, Kind: IssueFrontmatterKey, Severity: severity, Message: i18n.Tf("wiki.check.frontmatter_key", k)})
		}
	}
}

// frontmatterKeys returns the top-level keys of a YAML mapping, in order.
func frontmatterKeys(src []byte) []string {
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	m := doc.Content[0]
	keys := make([]string, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, m.Content[i].Value)
	}
	return keys
}
