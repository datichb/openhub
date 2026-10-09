package gateway

import (
	"errors"
	"strings"

	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// DefaultBeadsAllow applies when the workflow declares no `beads:` block:
// read-only commands. An explicit empty list refuses every command.
var DefaultBeadsAllow = []string{"show", "list", "ready", "search", "children", "comments", "count", "status", "graph", "history"}

// forbiddenFlags would let a command escape the project database or write
// files on the machine; they are refused wherever and however they are
// written (bd reads `-C/x`, `-qC/x`, `--db=x` as well as `--db x`).
var forbiddenFlags = map[string]bool{
	"db": true, "database": true, "directory": true, "global": true, "cpu-profile": true,
	"mem-profile": true, "dolt-auto-commit": true, "repo": true,
}

// Refusal is a command refused by the gateway, with a localized message.
type Refusal struct{ Msg string }

func (r *Refusal) Error() string { return r.Msg }

func refuse(key string, args ...any) *Refusal { return &Refusal{Msg: i18n.Tf(key, args...)} }

// command is a bd invocation as bd reads it (bdargs).
type command struct {
	bdargs.Cmd
}

// parseBeads reads argv as bd does; arguments bd would refuse, or that oh
// cannot check, are refused.
func parseBeads(argv []string) (command, error) {
	c, err := bdargs.Parse(argv)
	var pe *bdargs.Error
	switch {
	case err == nil:
	case errors.As(err, &pe) && pe.Kind == bdargs.UnknownFlag:
		return command{}, refuse("cmd.gateway.beads.unknown_option", pe.Token, bdargs.GeneratedFor)
	case errors.As(err, &pe) && pe.Kind == bdargs.UnknownCommand:
		return command{}, refuse("cmd.gateway.beads.unknown_command", pe.Token, bdargs.GeneratedFor)
	default:
		return command{}, refuse("cmd.gateway.beads.bad_args", err.Error())
	}
	for _, f := range c.Flags {
		if forbiddenFlags[f.Name] {
			return command{}, refuse("cmd.gateway.beads.forbidden_flag", "--"+f.Name)
		}
	}
	return command{Cmd: c}, nil
}

// checkBeads parses argv and checks it against the allow-list (nil = the
// default read-only list). Entries are commands ("show", alias names
// accepted) or a command and its subcommand or first word ("dep tree").
func checkBeads(argv, allow []string) (command, error) {
	if allow == nil {
		allow = DefaultBeadsAllow
	}
	c, err := parseBeads(argv)
	if err != nil {
		return command{}, err
	}
	if c.Name() == "" || c.Help() {
		return c, nil // bd, bd --version, bd help, bd <cmd> --help
	}
	second := ""
	switch {
	case len(c.Path) > 1:
		second = c.Path[1]
	case len(c.Args) > 0:
		second = c.Args[0]
	}
	for _, e := range allow {
		if allowed(e, c.Name(), second) {
			return c, nil
		}
	}
	if len(allow) == 0 {
		return c, refuse("cmd.gateway.beads.none_allowed")
	}
	shown := c.Key()
	if len(c.Path) == 1 && second != "" {
		shown += " " + second
	}
	return c, refuse("cmd.gateway.beads.not_allowed", shown, strings.Join(allow, ", "))
}

// allowed matches one allow-list entry: its first word may be an alias.
func allowed(entry, name, second string) bool {
	w := strings.Fields(entry)
	if len(w) == 0 || len(w) > 2 {
		return false
	}
	first := w[0]
	if p, ok := bdargs.Known(first); ok {
		first = p[0]
	}
	if first != name {
		return false
	}
	if len(w) == 1 {
		return true
	}
	if p, ok := bdargs.Known(name, w[1]); ok {
		return p[1] == second
	}
	return w[1] == second
}
