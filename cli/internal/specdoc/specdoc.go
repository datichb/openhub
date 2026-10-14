// Package specdoc checks the specs written in the wiki of a project
// (docs/wiki/specs/<slug>.md, modernisation R4): what oh verifies at the
// checkpoints of the workflow `spec` instead of trusting the agent.
//
// A spec has a glossary, acceptance criteria that each carry an id (CA-1)
// and name their test point, and, once sliced, a breakdown whose slices
// each cite the criteria they cover:
//
//	## Glossaire
//	- **Réservation** : file d'attente sur un livre déjà prêté (`Reservation`)
//	## Critères d'acceptation
//	- **CA-1** — total nul : erreur · Test : `TestPercent_ZeroTotal`
//	## Découpage
//	- **T1** — Percent et ses erreurs · Couvre : CA-1
//
// Lists and tables are read; headings in French or English. Fenced code
// blocks and the frontmatter are left out.
package specdoc

import (
	"regexp"
	"strconv"
	"strings"
)

// Problem codes.
const (
	NoGlossary            = "no_glossary"
	EmptyGlossary         = "empty_glossary"
	NoCriteria            = "no_criteria"
	CriterionNoID         = "criterion_no_id"
	CriterionDuplicate    = "criterion_duplicate"
	CriterionNoTest       = "criterion_no_test"
	NoBreakdown           = "no_breakdown"
	SliceNoCriterion      = "slice_no_criterion"
	SliceUnknownCriterion = "slice_unknown_criterion"
	CriterionUncovered    = "criterion_uncovered"
)

// Problem is a defect of a spec. Item names what it is about: a criterion
// id, or the start of the line of an item.
type Problem struct {
	Code string
	Item string
}

var (
	reHeading   = regexp.MustCompile(`^(#{1,6})[ \t]+(.+?)[ \t#]*$`)
	reListItem  = regexp.MustCompile(`^([ \t]*)(?:[-*+]|\d+[.)])[ \t]+(.*)$`)
	reTableRow  = regexp.MustCompile(`^[ \t]*\|.*\|[ \t]*$`)
	reTableSep  = regexp.MustCompile(`^[ \t]*\|?[ \t]*:?-{2,}`)
	reID        = regexp.MustCompile(`(?i)\b(CA|AC)[ \t]*-[ \t]*(\d+)\b`)
	reTest      = regexp.MustCompile(`(?i)\b(?:tests?|points? de test|test point)[\s\x{00A0}\x{202F}]*:[\s\x{00A0}\x{202F}]*(.*)$`)
	reGlossary  = regexp.MustCompile(`(?i)glossaire|glossary|vocabulaire|lexique`)
	reCriteria  = regexp.MustCompile(`(?i)crit[eèé]res?|acceptance criteria`)
	reBreakdown = regexp.MustCompile(`(?i)d[ée]coupage|tranches?|breakdown|slices?`)
	reTestCol   = regexp.MustCompile(`(?i)test`)
	rePlacehold = regexp.MustCompile(`(?i)^(?:[?….—–-]+$|(?:tbd|todo|à définir|a definir|à préciser|a preciser|à venir|aucun|none|n/?a)\b)`)
)

// Check returns the problems of the spec src; breakdown also checks the
// breakdown against the criteria (convergence of the tickets and the spec).
func Check(src []byte, breakdown bool) []Problem {
	secs := sections(lines(string(src)))
	var out []Problem
	gl := find(secs, reGlossary)
	switch {
	case gl == nil:
		out = append(out, Problem{Code: NoGlossary})
	case len(units(gl.body, false)) == 0:
		out = append(out, Problem{Code: EmptyGlossary})
	}
	crit := find(secs, reCriteria)
	ids, probs := criteria(crit)
	out = append(out, probs...)
	if !breakdown {
		return out
	}
	br := find(secs, reBreakdown)
	if br == nil {
		return append(out, Problem{Code: NoBreakdown})
	}
	return append(out, checkBreakdown(br, ids)...)
}

// IDs returns the criterion ids of the spec, in order.
func IDs(src []byte) []string {
	ids, _ := criteria(find(sections(lines(string(src))), reCriteria))
	return ids
}

// lines returns the lines of src without the frontmatter and the fenced
// code blocks.
func lines(src string) []string {
	all := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	start := 0
	if len(all) > 0 && strings.TrimSpace(all[0]) == "---" {
		for i := 1; i < len(all); i++ {
			if strings.TrimSpace(all[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	var out []string
	fence := ""
	for _, l := range all[start:] {
		t := strings.TrimSpace(l)
		switch {
		case fence != "":
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		case strings.HasPrefix(t, "```"), strings.HasPrefix(t, "~~~"):
			fence = t[:3]
			continue
		}
		out = append(out, l)
	}
	return out
}

type section struct {
	title string
	body  []string
}

// sections cuts lines at the headings: the body of a section runs to the
// next heading of the same or a higher level (its subsections included).
func sections(ls []string) []section {
	type head struct{ at, level int }
	var heads []head
	for i, l := range ls {
		if m := reHeading.FindStringSubmatch(l); m != nil {
			heads = append(heads, head{i, len(m[1])})
		}
	}
	out := make([]section, 0, len(heads))
	for k, h := range heads {
		end := len(ls)
		for _, n := range heads[k+1:] {
			if n.level <= h.level {
				end = n.at
				break
			}
		}
		title := reHeading.FindStringSubmatch(ls[h.at])[2]
		var body []string
		for _, l := range ls[h.at+1 : end] {
			if !reHeading.MatchString(l) {
				body = append(body, l)
			}
		}
		out = append(out, section{title: title, body: body})
	}
	return out
}

// find returns the first section whose title matches re.
func find(secs []section, re *regexp.Regexp) *section {
	for i := range secs {
		if re.MatchString(secs[i].title) {
			return &secs[i]
		}
	}
	return nil
}

// unit is an entry of a section: a list item (its continuation lines and
// sub-items in text) or a table row. test is the cell of the test column
// of a table row (testCol false: no such column).
type unit struct {
	line     string // its own line, trimmed
	text     string // the line, continuation lines and sub-items
	children int
	nested   bool // a sub-item
	testCol  bool
	test     string
}

// units returns the entries of a section body: its list items (topOnly:
// the outermost ones only) and the rows of its tables.
func units(body []string, topOnly bool) []unit {
	type item struct {
		indent int
		start  int
	}
	var items []item
	top := -1
	for i, l := range body {
		if m := reListItem.FindStringSubmatch(l); m != nil {
			ind := len(strings.ReplaceAll(m[1], "\t", "    "))
			items = append(items, item{ind, i})
			if top < 0 || ind < top {
				top = ind
			}
		}
	}
	var out []unit
	for k, it := range items {
		if topOnly && it.indent > top {
			continue
		}
		end := len(body)
		children := 0
		for _, n := range items[k+1:] {
			if n.indent <= it.indent {
				end = n.start
				break
			}
			children++
		}
		var b strings.Builder
		for _, l := range body[it.start:end] {
			if reTableRow.MatchString(l) {
				break
			}
			b.WriteString(strings.TrimSpace(l))
			b.WriteByte('\n')
		}
		out = append(out, unit{line: strings.TrimSpace(body[it.start]), text: b.String(), children: children, nested: it.indent > top})
	}
	return append(out, tableRows(body)...)
}

// tableRows returns the body rows of the tables of a section.
func tableRows(body []string) []unit {
	var out []unit
	var header []string
	inTable := false
	for _, l := range body {
		if !reTableRow.MatchString(l) {
			inTable, header = false, nil
			continue
		}
		cells := splitRow(l)
		switch {
		case !inTable:
			inTable, header = true, cells
			continue
		case reTableSep.MatchString(l) && strings.Trim(strings.Join(cells, ""), "-: ") == "":
			continue
		}
		u := unit{line: strings.TrimSpace(l), text: strings.Join(cells, " | ")}
		for i, h := range header {
			if reTestCol.MatchString(h) && !reCriteria.MatchString(h) {
				u.testCol = true
				if i < len(cells) {
					u.test = cells[i]
				}
				break
			}
		}
		out = append(out, u)
	}
	return out
}

func splitRow(l string) []string {
	t := strings.TrimSpace(l)
	t = strings.TrimSuffix(strings.TrimPrefix(t, "|"), "|")
	cells := strings.Split(t, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// criteria returns the ids of the criteria of the section and the
// problems of its entries.
func criteria(sec *section) ([]string, []Problem) {
	if sec == nil {
		return nil, []Problem{{Code: NoCriteria}}
	}
	var ids []string
	var out []Problem
	seen := map[string]bool{}
	for _, u := range units(sec.body, false) {
		id := firstID(u.line)
		if id == "" {
			if !u.nested && u.children == 0 {
				out = append(out, Problem{Code: CriterionNoID, Item: excerpt(u.line)})
			}
			continue
		}
		if seen[id] {
			out = append(out, Problem{Code: CriterionDuplicate, Item: id})
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if !hasTest(u) {
			out = append(out, Problem{Code: CriterionNoTest, Item: id})
		}
	}
	if len(ids) == 0 && len(out) == 0 {
		out = append(out, Problem{Code: NoCriteria})
	}
	return ids, out
}

// hasTest reports whether an entry names its test point: a `Test :` with a
// value, or a filled test cell.
func hasTest(u unit) bool {
	if u.testCol {
		return filled(u.test)
	}
	for _, l := range strings.Split(u.text, "\n") {
		if m := reTest.FindStringSubmatch(l); len(m) > 1 && filled(m[1]) {
			return true
		}
	}
	return false
}

func filled(v string) bool {
	v = strings.TrimSpace(strings.Trim(strings.TrimSpace(v), "`*_"))
	return v != "" && !rePlacehold.MatchString(v)
}

// checkBreakdown checks the breakdown: each slice cites known criteria, each
// criterion is cited.
func checkBreakdown(sec *section, ids []string) []Problem {
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	covered := map[string]bool{}
	var out []Problem
	us := units(sec.body, true)
	for _, u := range us {
		cited := allIDs(u.text)
		if len(cited) == 0 {
			out = append(out, Problem{Code: SliceNoCriterion, Item: excerpt(u.line)})
			continue
		}
		for _, id := range cited {
			if !known[id] {
				out = append(out, Problem{Code: SliceUnknownCriterion, Item: id})
				continue
			}
			covered[id] = true
		}
	}
	if len(us) == 0 {
		return append(out, Problem{Code: NoBreakdown})
	}
	for _, id := range ids {
		if !covered[id] {
			out = append(out, Problem{Code: CriterionUncovered, Item: id})
		}
	}
	return out
}

func normID(m []string) string {
	n, _ := strconv.Atoi(m[2])
	return strings.ToUpper(m[1]) + "-" + strconv.Itoa(n)
}

func firstID(s string) string {
	if m := reID.FindStringSubmatch(s); m != nil {
		return normID(m)
	}
	return ""
}

func allIDs(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range reID.FindAllStringSubmatch(s, -1) {
		if id := normID(m); !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func excerpt(s string) string {
	s = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), "-*+|0123456789.) "))
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
