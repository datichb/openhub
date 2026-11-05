package tracker

import (
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
)

// ticketIDFromIssue derives the hub ticket ID from a tracker IssueState: the
// key of the tracker when it has one (Jira, "SRU-42"), otherwise the IID
// dressed the way the ticket pattern of the project writes it (pattern
// `^SRU-(\d+)$` gives "SRU-42"; no pattern, or one that cannot be written
// back, gives "42"). The ID is the one resolveIID reads the IID back from.
func ticketIDFromIssue(issue IssueState, pattern string) string {
	if issue.Key != "" {
		return issue.Key
	}
	return ticketIDFromIID(issue.IID, pattern)
}

// ticketIDFromIID is the ticket ID of the issue number iid under pattern.
func ticketIDFromIID(iid int, pattern string) string {
	n := strconv.Itoa(iid)
	prefix, suffix, ok := patternAffixes(pattern)
	if !ok {
		return n
	}
	id := prefix + n + suffix
	if back, found := resolveIID(id, 0, pattern); !found || back != iid {
		return n
	}
	return id
}

// patternAffixes is the literal text a ticket pattern puts before and after
// the number it captures: `^SRU-(\d+)$` gives ("SRU-", ""). ok is false when
// the pattern is anything but literal text around one captured number (a
// wildcard, an alternative), which cannot be written back.
func patternAffixes(pattern string) (prefix, suffix string, ok bool) {
	if pattern == "" {
		return "", "", false
	}
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", "", false
	}
	re = re.Simplify()
	subs := []*syntax.Regexp{re}
	if re.Op == syntax.OpConcat {
		subs = re.Sub
	}
	var before, after strings.Builder
	captured := false
	for _, s := range subs {
		switch s.Op {
		case syntax.OpBeginText, syntax.OpBeginLine, syntax.OpEndText, syntax.OpEndLine, syntax.OpEmptyMatch:
		case syntax.OpLiteral:
			if s.Flags&syntax.FoldCase != 0 {
				return "", "", false
			}
			if captured {
				after.WriteString(string(s.Rune))
			} else {
				before.WriteString(string(s.Rune))
			}
		case syntax.OpCapture:
			if captured || !isNumber(s.Sub[0]) {
				return "", "", false
			}
			captured = true
		default:
			return "", "", false
		}
	}
	if !captured {
		return "", "", false
	}
	return before.String(), after.String(), true
}

// isNumber: the expression matches digits only (\d+, [0-9]+, \d*).
func isNumber(re *syntax.Regexp) bool {
	if re.Op != syntax.OpPlus && re.Op != syntax.OpStar {
		return false
	}
	cc := re.Sub[0]
	if cc.Op != syntax.OpCharClass || len(cc.Rune) == 0 || len(cc.Rune)%2 != 0 {
		return false
	}
	for i := 0; i < len(cc.Rune); i += 2 {
		if cc.Rune[i] < '0' || cc.Rune[i+1] > '9' {
			return false
		}
	}
	return true
}

// iidKey identifies an issue of a tracker: its tracker project and number.
type iidKey struct {
	trackerProject string
	iid            int
}

// claimIID is the issue number of a claim: the one recorded on it, else the
// one its ID carries under the pattern of its project, else the ID itself
// when it is a number (a claim made by hand for an issue "42").
func claimIID(storedIID int, ticketID, pattern string) (int, bool) {
	if iid, ok := resolveIID(ticketID, storedIID, pattern); ok {
		return iid, true
	}
	if n, err := strconv.Atoi(ticketID); err == nil && n > 0 && strconv.Itoa(n) == ticketID {
		return n, true
	}
	return 0, false
}

// issueIndex remembers which hub project holds the claim of each issue of the
// tracker, so that an issue gets a claim once: two hub projects linked to the
// same tracker project do not both take it, and the same issue does not come
// as "42" and as "SRU-42".
type issueIndex struct {
	owner map[iidKey]string
}

func newIssueIndex() *issueIndex { return &issueIndex{owner: map[iidKey]string{}} }

// note records that hubProject holds the claim of the issue; the first owner
// stays.
func (x *issueIndex) note(trackerProject string, iid int, hubProject string) {
	k := iidKey{trackerProject, iid}
	if _, ok := x.owner[k]; !ok {
		x.owner[k] = hubProject
	}
}

// taken reports whether a claim already exists for the issue, in any hub
// project linked to its tracker project.
func (x *issueIndex) taken(trackerProject string, iid int) bool {
	_, ok := x.owner[iidKey{trackerProject, iid}]
	return ok
}

// count is the number of issues of the tracker project that have a claim.
func (x *issueIndex) count(trackerProject string) int {
	n := 0
	for k := range x.owner {
		if k.trackerProject == trackerProject {
			n++
		}
	}
	return n
}

// sortedKeys are the keys of m in alphabetical order (a run goes through the
// projects in a fixed order, so that the first owner of an issue is always the
// same).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
