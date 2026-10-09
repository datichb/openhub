package cmd

import (
	"bytes"
	"strings"
	"testing"

	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
)

// S-05: the replay plan comes from the job; its control sequences cannot
// erase lines, move the cursor or retitle the terminal.
func TestPrintReplayPlanNeutralizesControls(t *testing.T) {
	useLocale(t, "en")
	plan := &remotesvc.ReplayPlan{
		Items: []remotesvc.ReplayItem{
			{Seq: 1, Argv: []string{"note", "bd-42", "ok\x1b[2K\x1b[1A\r"}, State: remotesvc.ItemPending},
			{Seq: 2, Argv: []string{"close", "bd-1"}, State: remotesvc.ItemRefused, Reason: "x\x1b]0;pwned\x07"},
		},
		Conflicts: []remotesvc.Conflict{{Ticket: "bd-42", Title: "t\x1b[8m", Notes: []string{"n\x9b31m\u202e"},
			Fields: []remotesvc.FieldChange{{Field: "status", Local: "open\r", Remote: "review\x1b[0m", Snapshot: "open"}}}},
	}
	var out bytes.Buffer
	printReplayPlan(&out, plan)
	s := out.String()
	for _, raw := range []string{"\x1b[2K", "\x1b[1A", "\r", "\x1b]0;", "\a", "\x1b[8m", "\u009b", "\u202e", "\x1b[0m"} {
		if strings.Contains(s, raw) {
			t.Fatalf("raw %q in the output:\n%q", raw, s)
		}
	}
	for _, want := range []string{`ok\x1b[2K\x1b[1A\x0d`, `x\x1b]0;pwned\x07`, `\u202e`} {
		if !strings.Contains(s, want) {
			t.Fatalf("%q not shown:\n%s", want, s)
		}
	}
}
