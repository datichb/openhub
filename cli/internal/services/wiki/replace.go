package wiki

import (
	"errors"
	"fmt"
	"strings"
)

// Replacement of a line (p5-fixes, wiki_propose `replaces`): a proposal
// may replace one line of a page instead of adding an entry, so that a
// re-onboarding corrects an entry rather than add a corrected copy next
// to it. The old line is given exactly (spaces around it aside); it must
// be in the page once (in its section, when the proposal names one),
// outside the frontmatter and the code fences, and must not be a heading,
// a separator or a tag line. oh checks it when the proposal is filed and
// again when it is applied: a line no longer in the page (or found twice)
// makes the proposal stale, not applied. The new content is one line,
// cleaned and neutralised like an addition; oh writes it in place of the
// old line, keeping its indentation and its list marker, with its tag
// line (the tag line of the old entry, right after it, goes too).

// MaxReplaceLines is the size of the content of a replacement.
const MaxReplaceLines = 1

// Errors of a replacement.
var (
	ErrReplaceLine      = errors.New("the replaced line is one line of the page, given exactly")
	ErrReplaceStructure = errors.New("a heading, a separator, a code fence or a tag line cannot be replaced")
	ErrReplaceContent   = fmt.Errorf("the new content of a replacement is %d line", MaxReplaceLines)
	ErrReplaceSame      = errors.New("the new content is the replaced line")
	// ErrStale: the replaced line is not in the page (or is there twice).
	ErrStale = errors.New("the replaced line is no longer in the page once")
)

// cleanReplaces checks the line a proposal replaces and returns it cleaned
// (spaces around it removed).
func cleanReplaces(line string) (string, error) {
	line = strings.TrimSpace(strings.ReplaceAll(line, "\r\n", "\n"))
	if line == "" || strings.Contains(line, "\n") || len(line) > MaxContent {
		return "", ErrReplaceLine
	}
	if structural(line) {
		return "", ErrReplaceStructure
	}
	return line, nil
}

// structural reports a line whose replacement would change the structure
// of the page.
func structural(line string) bool {
	t := strings.TrimSpace(line)
	return headRe.MatchString(t) || breakRe.MatchString(t) || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") ||
		(strings.HasPrefix(t, "—") && tagLineRe.MatchString(t))
}

// findLine returns the index of the only line of text equal to want
// (spaces around it aside), in the section when one is given, outside the
// frontmatter and the code fences: ErrStale when it is not there once,
// ErrNoSection when the section is missing.
func findLine(text, section, want string) (int, error) {
	lo, hi := 0, -1
	if section != "" {
		var ok bool
		if lo, hi, ok = sectionRange(text, section); !ok {
			return -1, fmt.Errorf("%w %q", ErrNoSection, section)
		}
	}
	at, n := -1, 0
	EachLine(text, func(i int, line string) {
		if i < lo || (hi >= 0 && i >= hi) {
			return
		}
		if strings.TrimSpace(line) == want {
			at, n = i, n+1
		}
	})
	if n != 1 {
		return -1, ErrStale
	}
	return at, nil
}

// sectionRange returns the lines of the first section of text whose
// heading reads section (level 2 or more): from its heading to the next
// heading of the same level or above.
func sectionRange(text, section string) (start, end int, ok bool) {
	want := normHeading(section)
	start, level := -1, 0
	end = -1
	EachLine(text, func(i int, line string) {
		m := headingRe.FindStringSubmatch(line)
		if len(m) != 3 {
			return
		}
		switch {
		case start < 0 && len(m[1]) >= 2 && normHeading(m[2]) == want:
			start, level = i, len(m[1])
		case start >= 0 && end < 0 && len(m[1]) <= level:
			end = i
		}
	})
	return start, end, start >= 0
}

// CheckReplaces checks that the line a proposal replaces is in its page
// once (wiki_propose, before it is queued).
func CheckReplaces(root string, p Proposal) error {
	if p.Replaces == "" {
		return nil
	}
	text, err := ReadPage(root, p.Page, "")
	if err != nil {
		return err
	}
	_, err = findLine(text, p.Section, p.Replaces)
	return err
}

// replaceEntry replaces the line of e.Replaces with the entry and its tag
// line.
func replaceEntry(text string, e Entry) (string, error) {
	at, err := findLine(text, e.Section, e.Replaces)
	if err != nil {
		return "", err
	}
	lines := strings.Split(text, "\n")
	old := lines[at]
	indent := old[:len(old)-len(strings.TrimLeft(old, " \t"))]
	trimmed := strings.TrimSpace(old)
	content := strings.TrimSpace(e.Content)
	var block []string
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		content = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(content, "- "), "* "))
		block = []string{indent + trimmed[:2] + content, indent + "  " + e.TagLine()}
	} else {
		block = []string{indent + content, indent + e.TagLine()}
	}
	end := at + 1
	// The tag line of the old entry, right after it, goes with it.
	if !anyTagRe.MatchString(old) && end < len(lines) {
		if next := strings.TrimSpace(lines[end]); strings.HasPrefix(next, "—") && tagLineRe.MatchString(next) {
			end++
		}
	}
	out := append(append(append([]string{}, lines[:at]...), block...), lines[end:]...)
	return strings.Join(out, "\n"), nil
}
