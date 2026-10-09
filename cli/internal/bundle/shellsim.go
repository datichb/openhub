package bundle

import (
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Simulator of the shell permission matching of the tool (lot 0 of the v5
// critical fixes, probes U1–U5, TestE2EShellMatcherSemantics). It mirrors
// what opencode 2.0.20 does, read from its source (core/src/shell/parse.ts,
// core/src/permission.ts, core/src/util/wildcard.ts) and verified by the e2e
// probe:
//
//   - a line is cut into its commands with a shell parser: every simple
//     command is checked on its own, including those of pipelines, lists
//     (`&&`, `||`, `;`, `&`), subshells and command substitutions (`$(…)`,
//     backquotes, also inside double quotes);
//   - the text checked is the raw source text of the command: environment
//     prefixes (`X=y git diff`) are part of it (U1), quotes and spaces are
//     not normalized (`git pu''sh`, `git  push`, U2), a redirection is part of
//     it (`echo x > f`);
//   - `cd` (and pushd…) and declarations (`export X=…`, `declare`, `local`,
//     `readonly`, `typeset`, `unset`) are not checked as commands: `cd`
//     arguments are checked as directories (external directory), the
//     substitutions inside declarations are checked;
//   - the strings given to an interpreter (`sh -c '…'`) are not parsed;
//   - a line is refused when one of its commands is refused, asks when one
//     asks, runs otherwise (U5); the rules are the agent rules then the
//     session rules, the last matching rule wins, nothing matching = ask;
//   - `*` matches any run (newlines included), `?` one character, matching
//     is case sensitive (except on Windows) and a pattern ending in " *" also
//     matches the bare command (`git status *` matches `git status`).
//
// Redirections are not checked against the external directory rules (U4):
// `echo x > ~/f` writes outside the project when `echo *` is allowed.

// ShellEffect returns the effect the tool gives to a shell line under rules
// (agent rules then session rules), with the commands it checks.
func ShellEffect(rules []sessionspec.PermissionRule, line string) sessionspec.Effect {
	cmds := ShellCommands(line)
	if len(cmds) == 0 {
		return sessionspec.EffectAllow
	}
	effects := make([]sessionspec.Effect, 0, len(cmds))
	for _, c := range cmds {
		e := toolEvaluate(rules, sessionspec.ActionShell, c)
		if e == sessionspec.EffectDeny {
			return sessionspec.EffectDeny
		}
		effects = append(effects, e)
	}
	for _, e := range effects {
		if e == sessionspec.EffectAsk {
			return sessionspec.EffectAsk
		}
	}
	return sessionspec.EffectAllow
}

// ToolEffect returns the effect of the rules for one resource of an action
// (edit paths, external directories…), with the matching of the tool.
func ToolEffect(rules []sessionspec.PermissionRule, action, resource string) sessionspec.Effect {
	return toolEvaluate(rules, action, resource)
}

func toolEvaluate(rules []sessionspec.PermissionRule, action, resource string) sessionspec.Effect {
	eff := sessionspec.EffectAsk
	for _, r := range rules {
		if toolMatch(action, r.Action) && toolMatch(resource, r.Resource) {
			eff = r.Effect
		}
	}
	return eff
}

// toolMatch is the wildcard of the tool: whole value, '*' any run, '?' one
// character, and a trailing " *" that also matches nothing.
func toolMatch(s, pattern string) bool {
	s = strings.ReplaceAll(s, `\`, "/")
	var b strings.Builder
	for _, r := range strings.ReplaceAll(pattern, `\`, "/") {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	re := b.String()
	if strings.HasSuffix(re, " .*") {
		re = strings.TrimSuffix(re, " .*") + "( .*)?"
	}
	flags := "(?s)"
	if runtime.GOOS == "windows" {
		flags = "(?si)"
	}
	re = flags + "^" + re + "$"
	if c, ok := patternCache.Load(re); ok {
		return c.(*regexp.Regexp).MatchString(s)
	}
	c := regexp.MustCompile(re)
	patternCache.Store(re, c)
	return c.MatchString(s)
}

var patternCache sync.Map

// notCommands are the first words of the statements the tool does not check
// as commands (directory changes are checked as directories, declarations
// are not commands for the parser).
var notCommands = map[string]bool{
	"cd": true, "chdir": true, "pushd": true, "popd": true,
	"export": true, "declare": true, "local": true, "readonly": true, "typeset": true, "unset": true,
}

// shellKeywords open or close a compound command: they are dropped from the
// head of a segment, the command after them is checked.
var shellKeywords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true, "do": true, "done": true,
	"while": true, "until": true, "!": true, "{": true, "}": true, "esac": true, "time": true,
}

// ShellCommands returns the commands of a line the tool checks one by one
// (raw text, trimmed).
func ShellCommands(line string) []string {
	var out []string
	collect(line, &out)
	return out
}

func collect(line string, out *[]string) {
	for _, seg := range splitTop(line) {
		inner, rest := substitutions(seg)
		for _, in := range inner {
			collect(in, out)
		}
		cmd := strings.TrimSpace(rest)
		for {
			w := firstWord(cmd)
			if !shellKeywords[w] {
				break
			}
			cmd = strings.TrimSpace(cmd[len(w):])
		}
		if cmd == "" || cmd[0] == '<' || cmd[0] == '>' {
			continue // nothing, or the redirection of a compound command
		}
		if strings.HasPrefix(cmd, "(") && strings.HasSuffix(cmd, ")") {
			collect(cmd[1:len(cmd)-1], out)
			continue
		}
		w := firstWord(cmd)
		if w == "for" || w == "case" || w == "in" || notCommands[w] || onlyAssignments(cmd) {
			continue
		}
		*out = append(*out, cmd)
	}
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t\n"); i >= 0 {
		return s[:i]
	}
	return s
}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// onlyAssignments reports a statement made of variable assignments only.
func onlyAssignments(cmd string) bool {
	for _, w := range strings.Fields(cmd) {
		if !assignment.MatchString(w) {
			return false
		}
	}
	return true
}

// splitTop cuts a line on the list and pipeline operators outside quotes,
// substitutions and subshells. The original text of each part is kept.
func splitTop(line string) []string {
	var parts []string
	start, depth := 0, 0
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			}
			continue
		case c == '\\':
			i++
			continue
		case quote == '"':
			switch {
			case c == '"':
				quote = 0
			case c == '$' && i+1 < len(line) && line[i+1] == '(':
				i = matchParen(line, i+1)
			case c == '`':
				i = matchBackquote(line, i)
			}
			continue
		case quote == '`':
			if c == '`' {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';', '\n':
			if depth == 0 {
				parts = append(parts, line[start:i])
				start = i + 1
			}
		case '|', '&':
			if depth != 0 {
				continue
			}
			if c == '&' && ((i > 0 && (line[i-1] == '>' || line[i-1] == '<')) || (i+1 < len(line) && line[i+1] == '>')) {
				continue // redirections: 2>&1, &>, <&
			}
			parts = append(parts, line[start:i])
			if i+1 < len(line) && (line[i+1] == c || (c == '|' && line[i+1] == '&')) {
				i++
			}
			start = i + 1
		}
	}
	parts = append(parts, line[start:])
	return parts
}

// substitutions returns the command substitutions of a segment ($(…),
// backquotes, process substitutions; also in double quotes) and the segment.
func substitutions(seg string) (inner []string, rest string) {
	var quote byte
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			}
			continue
		}
		switch {
		case c == '\\':
			i++
		case c == '\'' && quote == 0:
			quote = c
		case c == '"':
			if quote == '"' {
				quote = 0
			} else {
				quote = '"'
			}
		case (c == '$' || c == '<' || c == '>') && i+1 < len(seg) && seg[i+1] == '(':
			end := matchParen(seg, i+1)
			if end > i+1 {
				inner = append(inner, seg[i+2:end])
			}
			i = end
		case c == '`':
			end := matchBackquote(seg, i)
			if end > i {
				inner = append(inner, seg[i+1:end])
			}
			i = end
		}
	}
	return inner, seg
}

// matchParen returns the index of the parenthesis closing the one at open.
func matchParen(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' {
				i++
			}
			continue
		}
		switch c {
		case '\\':
			i++
		case '\'', '"':
			quote = c
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s) - 1
}

func matchBackquote(s string, open int) int {
	for i := open + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '`' {
			return i
		}
	}
	return len(s) - 1
}
