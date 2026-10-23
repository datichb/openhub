package gitlabapi

import (
	"regexp"
	"strconv"
	"strings"
)

// RefKind says what a GitLab reference points to.
type RefKind string

const (
	// RefIssue is an issue (#623, …/-/issues/623).
	RefIssue RefKind = "issue"
	// RefMR is a merge request (!12, …/-/merge_requests/12).
	RefMR RefKind = "mr"
)

// Ref is a GitLab issue or merge request named by the user: #<iid>, !<iid>
// or the URL of one. #<iid> and !<iid> are GitLab's own syntax: a Beads
// ticket is never written that way (proj-a1b, bd-12).
type Ref struct {
	Kind RefKind
	IID  int
	// Project and Host come from a URL (the project path and the server);
	// empty for #<iid> and !<iid>, which belong to the project of the session.
	Project string
	Host    string
	// Text is the reference as written (the URL without its query and
	// fragment).
	Text string
}

// maxRefDigits bounds the digits of a short reference: #123456789 is not an
// issue number, and an all-digit hex color (#123456) stays within six.
const maxRefDigits = 6

var (
	// reRefURL matches the URL of an issue or merge request of a GitLab
	// server: host, project path (groups included), `/-/`, kind, IID. The
	// `/-/` is required: it tells GitLab from GitHub (…/issues/12, …/pull/12).
	reRefURL = regexp.MustCompile(`https?://([A-Za-z0-9.-]+(?::\d+)?)/((?:[A-Za-z0-9._~-]+/)*[A-Za-z0-9._~-]+)/-/(issues|work_items|merge_requests)/(\d+)`)
	// reRefShort matches #<iid> and !<iid> at the start of a word: after the
	// start of the text, a space or an opening punctuation mark.
	reRefShort = regexp.MustCompile(`(?:^|[\s(\[{,;:"'` + "`" + `])([#!])(\d{1,` + strconv.Itoa(maxRefDigits) + `})\b`)
)

// ParseRef reads s as a single reference: #623, !12 or a URL.
func ParseRef(s string) (Ref, bool) {
	s = strings.TrimSpace(s)
	refs := FindRefs(s)
	if len(refs) != 1 {
		return Ref{}, false
	}
	// The whole string must be the reference (a sentence is not one), but a
	// URL may carry a query or a fragment.
	r := refs[0]
	if s != r.Text && !strings.HasPrefix(s, r.Text+"#") && !strings.HasPrefix(s, r.Text+"?") {
		return Ref{}, false
	}
	return r, true
}

// FindRefs returns the references of a text, in order of appearance, once
// each.
func FindRefs(text string) []Ref {
	type hit struct {
		at  int
		ref Ref
	}
	var hits []hit
	seen := map[string]bool{}
	add := func(at int, r Ref) {
		if r.IID <= 0 || seen[r.Text] {
			return
		}
		seen[r.Text] = true
		hits = append(hits, hit{at, r})
	}

	rest := []byte(text)
	for _, m := range reRefURL.FindAllStringSubmatchIndex(text, -1) {
		iid, err := strconv.Atoi(text[m[8]:m[9]])
		if err != nil {
			continue
		}
		kind := RefIssue
		if text[m[6]:m[7]] == "merge_requests" {
			kind = RefMR
		}
		add(m[0], Ref{Kind: kind, IID: iid, Host: text[m[2]:m[3]], Project: text[m[4]:m[5]], Text: text[m[0]:m[1]]})
		// Blank the URL and what sticks to it (#note_5, ?x=1) so that it is
		// not read again.
		for i := m[0]; i < len(rest) && (i < m[1] || (rest[i] != ' ' && rest[i] != '\n' && rest[i] != '\t' && rest[i] != '\r')); i++ {
			rest[i] = ' '
		}
	}
	for _, m := range reRefShort.FindAllStringSubmatchIndex(string(rest), -1) {
		iid, err := strconv.Atoi(text[m[4]:m[5]])
		if err != nil {
			continue
		}
		kind := RefIssue
		if text[m[2]:m[3]] == "!" {
			kind = RefMR
		}
		add(m[2], Ref{Kind: kind, IID: iid, Text: text[m[2]:m[5]]})
	}
	// Order of appearance.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].at < hits[j-1].at; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	out := make([]Ref, len(hits))
	for i, h := range hits {
		out[i] = h.ref
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
