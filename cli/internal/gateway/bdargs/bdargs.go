// Package bdargs parses the arguments of a bd command the way bd does
// (cobra and pflag), with the command and flag table of the bd version it
// was generated from (spec_bd.go, `go run ./gen/main.go`). The Beads
// gateway checks what bd will really run (attached short values `-sclosed`,
// `--name=value`, combined short flags `-qC/dir`, flags before or after the
// subcommand), then hands bd a canonical argument list (Cmd.Argv) so that
// bd cannot read the command differently (audit v5, S-07, S-08).
//
// Unknown commands and flags are errors: bd refuses them too, and a flag of
// a newer bd cannot be checked.
package bdargs

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type kind uint8

const (
	boolFlag     kind = iota // no value (NoOptDefVal "true")
	valueFlag                // takes a value (attached, after =, or the next argument)
	optionalFlag             // value only after = (NoOptDefVal)
)

type flagDef struct {
	name  string
	short byte
	kind  kind
}

type cmdDef struct {
	path   []string
	flags  map[string]flagDef
	shorts map[byte]flagDef
	subs   map[string]*cmdDef // by name and alias
}

var root = build()

func parseSpec(spec string, into *cmdDef) {
	for _, tok := range strings.Fields(spec) {
		d := flagDef{kind: boolFlag}
		switch {
		case strings.HasSuffix(tok, "="):
			d.kind, tok = valueFlag, strings.TrimSuffix(tok, "=")
		case strings.HasSuffix(tok, "?"):
			d.kind, tok = optionalFlag, strings.TrimSuffix(tok, "?")
		}
		name, short, _ := strings.Cut(tok, "/")
		d.name = name
		if short != "" {
			d.short = short[0]
			into.shorts[d.short] = d
		}
		into.flags[name] = d
	}
}

func newDef(path []string) *cmdDef {
	d := &cmdDef{path: path, flags: map[string]flagDef{}, shorts: map[byte]flagDef{}, subs: map[string]*cmdDef{}}
	parseSpec(globalSpec, d)
	return d
}

func build() *cmdDef {
	r := newDef(nil)
	r.subs["help"] = newDef([]string{"help"}) // cobra's own command
	keys := make([]string, 0, len(commandSpecs))
	for k := range commandSpecs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { // parents first
		return strings.Count(keys[i], " ") < strings.Count(keys[j], " ") || (strings.Count(keys[i], " ") == strings.Count(keys[j], " ") && keys[i] < keys[j])
	})
	for _, k := range keys {
		path := strings.Fields(k)
		parent := r
		for _, p := range path[:len(path)-1] {
			if parent = parent.subs[p]; parent == nil {
				break
			}
		}
		if parent == nil {
			continue
		}
		d := newDef(path)
		parseSpec(commandSpecs[k], d)
		name := path[len(path)-1]
		parent.subs[name] = d
		for _, a := range strings.Fields(commandAliases[k]) {
			if _, taken := parent.subs[a]; !taken {
				parent.subs[a] = d
			}
		}
	}
	return r
}

// Flag is one flag of a parsed command.
type Flag struct {
	Name     string // long name ("status" for -s)
	Value    string // "true" for a boolean given without value
	Explicit bool   // the value was written (not a boolean or optional default)
}

// Cmd is a parsed bd command.
type Cmd struct {
	Path  []string // canonical command path ("dep", "add"); empty for `bd`, `bd --version`
	Flags []Flag   // in order, global flags included
	Args  []string // positional arguments
}

// Name is the subcommand ("" for none).
func (c Cmd) Name() string {
	if len(c.Path) == 0 {
		return ""
	}
	return c.Path[0]
}

// Key is the command path joined by spaces ("dep add").
func (c Cmd) Key() string { return strings.Join(c.Path, " ") }

// Has reports whether the flag was given.
func (c Cmd) Has(name string) bool {
	for _, f := range c.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

// Value is the last value of a flag (bd keeps the last one).
func (c Cmd) Value(name string) (string, bool) {
	v, ok := "", false
	for _, f := range c.Flags {
		if f.Name == name {
			v, ok = f.Value, true
		}
	}
	return v, ok
}

// Values are all the values of a repeatable flag.
func (c Cmd) Values(name string) []string {
	var out []string
	for _, f := range c.Flags {
		if f.Name == name {
			out = append(out, f.Value)
		}
	}
	return out
}

// Bool reports whether a boolean flag is set to true.
func (c Cmd) Bool(name string) bool {
	v, ok := c.Value(name)
	if !ok {
		return false
	}
	switch v {
	case "1", "t", "T", "true", "TRUE", "True":
		return true
	}
	return false
}

// Help reports a help request (bd only prints the help).
func (c Cmd) Help() bool { return c.Bool("help") || c.Name() == "help" }

// Argv is the canonical argument list of the command: the command path,
// every flag as --name or --name=value, then "--" and the positional
// arguments. bd reads it exactly as it was checked.
func (c Cmd) Argv() []string {
	out := append([]string{}, c.Path...)
	for _, f := range c.Flags {
		if f.Explicit {
			out = append(out, "--"+f.Name+"="+f.Value)
		} else {
			out = append(out, "--"+f.Name)
		}
	}
	if len(c.Args) > 0 {
		out = append(out, "--")
		out = append(out, c.Args...)
	}
	return out
}

// ErrorKind classifies a parse error.
type ErrorKind int

const (
	UnknownFlag ErrorKind = iota + 1
	MissingValue
	UnknownCommand
	BadSyntax
)

// Error is an argument list bd would refuse, or that cannot be checked.
type Error struct {
	Kind  ErrorKind
	Token string
}

func (e *Error) Error() string {
	switch e.Kind {
	case UnknownFlag:
		return fmt.Sprintf("unknown flag %q", e.Token)
	case MissingValue:
		return fmt.Sprintf("flag needs an argument: %q", e.Token)
	case UnknownCommand:
		return fmt.Sprintf("unknown command %q", e.Token)
	default:
		return fmt.Sprintf("bad flag syntax: %q", e.Token)
	}
}

// Known reports whether path (canonical or alias names) is a bd command,
// and returns its canonical path.
func Known(path ...string) ([]string, bool) {
	d := root
	for _, p := range path {
		if d = d.subs[p]; d == nil {
			return nil, false
		}
	}
	return d.path, true
}

// Parse reads argv (the arguments after "bd") as bd does: the command is
// found first (cobra Find), then every flag is parsed with the flags of
// that command (pflag, flags and arguments interspersed).
func Parse(argv []string) (Cmd, error) {
	d, rest, err := find(argv)
	if err != nil {
		return Cmd{}, err
	}
	c := Cmd{Path: append([]string{}, d.path...)}
	for i := 0; i < len(rest); i++ {
		s := rest[i]
		switch {
		case len(s) < 2 || s[0] != '-':
			c.Args = append(c.Args, s)
		case s == "--":
			c.Args = append(c.Args, rest[i+1:]...)
			return c, nil
		case s[1] == '-':
			name, val, has := strings.Cut(s[2:], "=")
			if name == "" || name[0] == '-' || name[0] == '=' {
				return Cmd{}, &Error{Kind: BadSyntax, Token: s}
			}
			f, ok := d.flags[name]
			if !ok {
				return Cmd{}, &Error{Kind: UnknownFlag, Token: s}
			}
			switch {
			case has:
				c.Flags = append(c.Flags, Flag{Name: name, Value: val, Explicit: true})
			case f.kind != valueFlag:
				c.Flags = append(c.Flags, Flag{Name: name, Value: "true"})
			case i+1 < len(rest):
				i++
				c.Flags = append(c.Flags, Flag{Name: name, Value: rest[i], Explicit: true})
			default:
				return Cmd{}, &Error{Kind: MissingValue, Token: s}
			}
		default:
			shorts := s[1:]
			for shorts != "" {
				f, ok := d.shorts[shorts[0]]
				if !ok {
					return Cmd{}, &Error{Kind: UnknownFlag, Token: "-" + shorts[:1] + " (" + s + ")"}
				}
				switch {
				case len(shorts) > 2 && shorts[1] == '=':
					c.Flags = append(c.Flags, Flag{Name: f.name, Value: shorts[2:], Explicit: true})
					shorts = ""
				case f.kind != valueFlag:
					c.Flags = append(c.Flags, Flag{Name: f.name, Value: "true"})
					shorts = shorts[1:]
				case len(shorts) > 1:
					c.Flags = append(c.Flags, Flag{Name: f.name, Value: shorts[1:], Explicit: true})
					shorts = ""
				case i+1 < len(rest):
					i++
					c.Flags = append(c.Flags, Flag{Name: f.name, Value: rest[i], Explicit: true})
					shorts = ""
				default:
					return Cmd{}, &Error{Kind: MissingValue, Token: s}
				}
			}
		}
	}
	return c, nil
}

// find walks the command tree like cobra (stripFlags, argsMinusFirstX):
// the first word that is not a flag or a flag value names a subcommand.
func find(args []string) (*cmdDef, []string, error) {
	d := root
	for {
		words := stripFlags(args, d)
		if len(words) == 0 {
			break
		}
		next := d.subs[words[0]]
		if next == nil {
			if d == root {
				return nil, nil, &Error{Kind: UnknownCommand, Token: words[0]}
			}
			break
		}
		args = minusFirst(args, words[0], d)
		d = next
	}
	return d, args, nil
}

// takesNext reports whether a flag token without "=" consumes the next
// argument, as cobra guesses it while looking for the subcommand (an
// unknown flag is assumed to take a value).
func takesNext(s string, d *cmdDef) bool {
	if strings.Contains(s, "=") {
		return false
	}
	if strings.HasPrefix(s, "--") {
		f, ok := d.flags[s[2:]]
		return !ok || f.kind == valueFlag
	}
	if len(s) == 2 && s[0] == '-' {
		f, ok := d.shorts[s[1]]
		return !ok || f.kind == valueFlag
	}
	return false
}

func stripFlags(args []string, d *cmdDef) []string {
	var words []string
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		switch {
		case s == "--":
			return words
		case strings.HasPrefix(s, "-") && takesNext(s, d):
			if len(args) <= 1 {
				return words
			}
			args = args[1:]
		case s != "" && !strings.HasPrefix(s, "-"):
			words = append(words, s)
		}
	}
	return words
}

func minusFirst(args []string, x string, d *cmdDef) []string {
	for pos := 0; pos < len(args); pos++ {
		s := args[pos]
		switch {
		case s == "--":
			return args
		case strings.HasPrefix(s, "-") && takesNext(s, d):
			pos++
		case !strings.HasPrefix(s, "-") && s == x:
			out := append([]string{}, args[:pos]...)
			return append(out, args[pos+1:]...)
		}
	}
	return args
}

var versionRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?(?:[-+][0-9A-Za-z.-]+)?`)

// VersionNumber extracts the version number of the output of `bd version`
// ("bd version 1.3.1 (Homebrew)" → "1.3.1"); "" when there is none.
func VersionNumber(out string) string {
	return versionRe.FindString(out)
}

// SameVersion reports whether the output of `bd version` names the bd
// version the table was generated from (GeneratedFor), and that version.
// The build (Homebrew…) does not count.
func SameVersion(out string) (expected string, same bool) {
	expected = VersionNumber(GeneratedFor)
	return expected, VersionNumber(out) == expected
}
