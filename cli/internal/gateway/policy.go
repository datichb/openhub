package gateway

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// DefaultBeadsAllow applies when the workflow declares no `beads:` block:
// read-only commands. An explicit empty list refuses every command.
var DefaultBeadsAllow = []string{"show", "list", "ready", "search", "children", "comments", "count", "status", "graph", "history"}

// forbiddenFlags would let a command escape the project database or write
// files on the machine; they are refused wherever they appear (bd global
// flags are accepted after the subcommand too).
var forbiddenFlags = []string{"--db", "--database", "-C", "--directory", "--global", "--cpu-profile", "--mem-profile", "--dolt-auto-commit"}

// Global flags accepted before the subcommand.
var (
	globalBoolFlags = map[string]bool{
		"--json": true, "-q": true, "--quiet": true, "-v": true, "--verbose": true, "--no-color": true,
		"--readonly": true, "--sandbox": true, "--ignore-schema-skew": true, "-h": true, "--help": true,
		"-V": true, "--version": true,
	}
	globalValueFlags = map[string]bool{"--actor": true}
)

// Refusal is a command refused by the gateway, with a localized message.
type Refusal struct{ Msg string }

func (r *Refusal) Error() string { return r.Msg }

func refuse(key string, args ...any) *Refusal { return &Refusal{Msg: i18n.Tf(key, args...)} }

// command is a parsed bd invocation.
type command struct {
	Name string // subcommand ("" = none: help/version)
	Sub  string // first positional word after the subcommand
	At   int    // index of the subcommand in argv (-1 = none)
}

// checkBeads parses argv and checks it against the allow-list (nil = the
// default read-only list). Entries are subcommands ("show") or a subcommand
// and its first word ("dep tree").
func checkBeads(argv, allow []string) (command, error) {
	if allow == nil {
		allow = DefaultBeadsAllow
	}
	for _, a := range argv {
		if a == "--" {
			break
		}
		name, _, _ := strings.Cut(a, "=")
		for _, f := range forbiddenFlags {
			if name == f || (f == "-C" && strings.HasPrefix(a, "-C")) {
				return command{}, refuse("cmd.gateway.beads.forbidden_flag", f)
			}
		}
	}
	c := command{At: -1}
	i := 0
	for i < len(argv) && strings.HasPrefix(argv[i], "-") {
		name, _, hasValue := strings.Cut(argv[i], "=")
		switch {
		case globalBoolFlags[name]:
			i++
		case globalValueFlags[name] && hasValue:
			i++
		case globalValueFlags[name]:
			i += 2
		default:
			return command{}, refuse("cmd.gateway.beads.unknown_option", argv[i])
		}
	}
	if i >= len(argv) {
		return c, nil // bd, bd --version, bd --help
	}
	c.Name, c.At = argv[i], i
	for _, a := range argv[i+1:] {
		if !strings.HasPrefix(a, "-") {
			c.Sub = a
			break
		}
	}
	if c.Name == "help" {
		return c, nil
	}
	for _, e := range allow {
		w := strings.Fields(e)
		switch {
		case len(w) == 1 && w[0] == c.Name:
			return c, nil
		case len(w) == 2 && w[0] == c.Name && w[1] == c.Sub:
			return c, nil
		}
	}
	if len(allow) == 0 {
		return c, refuse("cmd.gateway.beads.none_allowed")
	}
	shown := c.Name
	if c.Sub != "" {
		shown += " " + c.Sub
	}
	return c, refuse("cmd.gateway.beads.not_allowed", shown, strings.Join(allow, ", "))
}
