package wiki

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/services/report"
)

// Proposals of wiki_propose (LD-J): a proposal is a datum written by an
// agent, never an instruction. oh cleans it (control characters and
// terminal sequences removed), neutralises what would change the
// structure of a page or hide text (headings, frontmatter markers, HTML
// and comments such as the oh:when markers, an unclosed code fence), drops
// the tag lines it carries (oh writes the tag line, with the author it
// saw), and refuses any reference to the oh session data
// (~/.oh/sessions, the scratch folders of the agents among them).

// Limits of a proposal.
const (
	MaxContent      = 4 << 10 // bytes, after cleaning
	MaxContentLines = 60
	MaxSection      = 120 // characters
	MaxSource       = 200 // characters
)

// Errors of a proposal.
var (
	ErrEmpty       = errors.New("the proposal is empty")
	ErrTooLarge    = fmt.Errorf("the proposal is larger than %d KiB or %d lines", MaxContent>>10, MaxContentLines)
	ErrBadTag      = errors.New("the tag is CONFIRMÉ, DÉDUIT or INCERTAIN")
	ErrNoSource    = errors.New("a CONFIRMÉ proposal names its source (file:line)")
	ErrSessionData = errors.New("a proposal may not cite the oh session data (~/.oh/sessions, scratch folders): put the finding itself, from the code")
)

// Proposal is a proposal as given by an agent.
type Proposal struct {
	Page    string
	Section string
	Content string
	Tag     string
	Source  string
}

// sessionDataRe finds a reference to the oh session data whatever the
// home of oh (~/.oh/sessions, $OH_HOME/sessions is checked apart).
var sessionDataRe = regexp.MustCompile(`(?i)\.oh[\\/]+sessions`)

// NormalizeTag returns the confidence tag a text names (case, accents and
// backticks aside).
func NormalizeTag(tag string) (string, bool) {
	t := strings.ToUpper(strings.Trim(strings.TrimSpace(tag), "`"))
	t = strings.NewReplacer("É", "E", "È", "E").Replace(t)
	switch t {
	case "CONFIRME":
		return TagConfirmed, true
	case "DEDUIT":
		return TagDeduced, true
	case "INCERTAIN":
		return TagUncertain, true
	}
	return "", false
}

// CleanProposal checks a proposal and returns it cleaned. sessionDirs are
// the session folders of oh (their path is refused as well).
func CleanProposal(in Proposal, sessionDirs ...string) (Proposal, error) {
	var out Proposal
	for _, s := range []string{in.Page, in.Section, in.Content, in.Source} {
		if citesSessionData(s, sessionDirs) {
			return out, ErrSessionData
		}
	}
	page, err := PagePath(in.Page)
	if err != nil {
		return out, err
	}
	tag, ok := NormalizeTag(in.Tag)
	if !ok {
		return out, ErrBadTag
	}
	out = Proposal{Page: page, Tag: tag}
	out.Section = oneLine(strings.TrimLeft(strings.TrimSpace(in.Section), "# "), MaxSection)
	out.Source = strings.Trim(oneLine(in.Source, MaxSource), "`")
	if tag == TagConfirmed && out.Source == "" {
		return out, ErrNoSource
	}
	content, err := cleanContent(in.Content)
	if err != nil {
		return out, err
	}
	out.Content = content
	return out, nil
}

func citesSessionData(s string, dirs []string) bool {
	if sessionDataRe.MatchString(s) {
		return true
	}
	norm := strings.ReplaceAll(s, "\\", "/")
	for _, d := range dirs {
		d = strings.TrimRight(strings.ReplaceAll(d, "\\", "/"), "/")
		if len(d) > 1 && strings.Contains(norm, d) {
			return true
		}
	}
	return false
}

// oneLine returns a cleaned text on one line, bounded.
func oneLine(s string, limit int) string {
	s = strings.Join(strings.Fields(report.Clean(s)), " ")
	if utf8.RuneCountInString(s) > limit {
		s = string([]rune(s)[:limit-1]) + "…"
	}
	return s
}

var (
	breakRe   = regexp.MustCompile(`^\s{0,3}([-*_])(\s*[-*_]){2,}\s*$`)
	headRe    = regexp.MustCompile(`^\s{0,3}#`)
	htmlTagRe = regexp.MustCompile(`<([A-Za-z/!?])`)
)

// cleanContent cleans and neutralises the content of a proposal.
func cleanContent(text string) (string, error) {
	text = report.Clean(text)
	var lines []string
	fence := false
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, " \t")
		if strings.HasPrefix(strings.TrimSpace(l), "```") || strings.HasPrefix(strings.TrimSpace(l), "~~~") {
			fence = !fence
			lines = append(lines, l)
			continue
		}
		if fence {
			lines = append(lines, l)
			continue
		}
		if tagLineRe.MatchString(strings.TrimSpace(l)) && strings.HasPrefix(strings.TrimSpace(l), "—") {
			continue // oh writes the tag line
		}
		switch {
		case headRe.MatchString(l):
			l = `\` + strings.TrimLeft(l, " ")
		case breakRe.MatchString(l):
			l = `\` + strings.TrimLeft(l, " ")
		}
		lines = append(lines, escapeHTML(l))
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if fence {
		lines = append(lines, "```")
	}
	out := strings.Trim(strings.Join(lines, "\n"), "\n")
	if strings.TrimSpace(out) == "" {
		return "", ErrEmpty
	}
	if len(out) > MaxContent || strings.Count(out, "\n")+1 > MaxContentLines {
		return "", ErrTooLarge
	}
	return out, nil
}

// escapeHTML neutralises HTML tags and comments outside inline code.
func escapeHTML(line string) string {
	parts := strings.Split(line, "`")
	step := 2
	if len(parts)%2 == 0 {
		step = 1 // an unclosed code span: nothing is code
	}
	for i := 0; i < len(parts); i += step {
		parts[i] = htmlTagRe.ReplaceAllString(parts[i], "&lt;$1")
		parts[i] = strings.ReplaceAll(parts[i], "-->", "--&gt;")
	}
	return strings.Join(parts, "`")
}
