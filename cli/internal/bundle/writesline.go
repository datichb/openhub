package bundle

import (
	"path"
	"strings"
	"unicode"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Lines of a `writes:` agent that the rules cannot read (p6-debts): git add
// or git commit of several declared paths, a commit message in several -m.
// oh reads them word by word, as the shell would, and lets through only:
//
//	git add [--] <path>…
//	git commit -m <message> [-m <message>]… -- <path>…
//
// every path declared (WritesCovered: a declared file, a declared folder or
// a path inside it), clean and relative (no `..`, no `.` part, no option,
// no pathspec magic), and each message one word or one quoted string
// without expansion. Anything else (another option, a redirection, a
// second command, a quote that does not close, an expansion) is refused.

// IsWritesLine reports whether a command is a git add or git commit line
// read by WritesLineProblem.
func IsWritesLine(cmd string) bool {
	return strings.HasPrefix(cmd, "git add ") || strings.HasPrefix(cmd, "git commit ")
}

// WritesLineProblem returns why a git add or git commit line of an agent
// whose `writes:` paths are paths is refused ("" when it only stages or
// commits declared paths).
func WritesLineProblem(paths []string, cmd string) string {
	if len(paths) == 0 {
		return "no writes: path"
	}
	words, why := writesWords(cmd)
	if why != "" {
		return why
	}
	if len(words) < 3 || words[0].text != "git" {
		return "not a git add or git commit line"
	}
	rest := words[2:]
	switch words[1].text {
	case "add":
		if len(rest) > 0 && rest[0].text == "--" && !rest[0].quoted {
			rest = rest[1:]
		}
	case "commit":
		messages := 0
		for len(rest) >= 2 && rest[0].text == "-m" && !rest[0].quoted {
			messages++
			rest = rest[2:]
		}
		if messages == 0 {
			return "a commit takes its message in -m"
		}
		if len(rest) == 0 || rest[0].text != "--" || rest[0].quoted {
			return "only -m messages before --, then the paths"
		}
		rest = rest[1:]
	default:
		return "not a git add or git commit line"
	}
	if len(rest) == 0 {
		return "no path"
	}
	for _, w := range rest {
		if w.quoted {
			return "quoted path " + w.text
		}
		if why := writesLinePath(paths, w.text); why != "" {
			return why
		}
	}
	return ""
}

// writesLinePath returns why p is not a declared path.
func writesLinePath(paths []string, p string) string {
	trimmed := strings.TrimSuffix(p, "/")
	switch {
	case trimmed == "" || strings.HasPrefix(p, "-") || strings.HasPrefix(p, ":") || path.IsAbs(p):
		return p + ": not a path"
	case path.Clean(trimmed) != trimmed || trimmed == ".":
		return p + ": not a clean relative path"
	}
	for _, part := range strings.Split(trimmed, "/") {
		if strings.HasPrefix(part, ".") {
			return p + ": hidden or relative part"
		}
	}
	for _, d := range paths {
		if dir, ok := strings.CutSuffix(d, workflow.WritesAll); ok && trimmed == dir {
			return "" // the declared folder itself
		}
	}
	if workflow.WritesCovered(trimmed, paths) && (trimmed == p || !isFile(paths, trimmed)) {
		return ""
	}
	return p + ": not a declared writes: path (" + strings.Join(paths, ", ") + ")"
}

func isFile(paths []string, p string) bool {
	for _, d := range paths {
		if d == p {
			return true
		}
	}
	return false
}

type writesWord struct {
	text   string
	quoted bool
}

// writesWords splits a line into its words: words of plain characters, or
// a whole word in single or double quotes. Anything the shell would
// expand, redirect or chain is refused.
func writesWords(cmd string) (words []writesWord, problem string) {
	var out []writesWord
	r := []rune(cmd)
	for i := 0; i < len(r); {
		c := r[i]
		switch c {
		case ' ':
			i++
		case '\'', '"':
			end := -1
			for j := i + 1; j < len(r); j++ {
				if r[j] == c {
					end = j
					break
				}
			}
			if end < 0 {
				return nil, "a quote that does not close"
			}
			body := string(r[i+1 : end])
			for _, b := range body {
				if unicode.IsControl(b) || (c == '"' && strings.ContainsRune("$`\\!", b)) {
					return nil, "an expansion or a control character in a quoted message"
				}
			}
			if end+1 < len(r) && r[end+1] != ' ' {
				return nil, "a quoted word glued to another"
			}
			out = append(out, writesWord{text: body, quoted: true})
			i = end + 1
		default:
			j := i
			for j < len(r) && r[j] != ' ' {
				if !plainRune(r[j]) {
					return nil, "a character the shell would read (" + string(r[j]) + ")"
				}
				j++
			}
			out = append(out, writesWord{text: string(r[i:j])})
			i = j
		}
	}
	return out, ""
}

// plainRune is a character the shell reads as itself in a word.
func plainRune(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || strings.ContainsRune("._/-+,@=%", c)
}
