package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/bundle"
)

func TestPrintBundleReport(t *testing.T) {
	r := &bundle.Report{Hash: "0123456789abcdef", Dir: "/b", EntryAgent: "lead",
		Agents: []bundle.AgentReport{{ID: "lead", Entry: true, Calls: []string{"helper"}, Tokens: 120}, {ID: "helper", Tokens: 30}},
		Skills: []bundle.SkillReport{{ID: "s1", Tokens: 50}},
		Budget: bundle.Budget{EntryAgent: 120, Agents: 150, Skills: 50, SkillCatalog: 5, Initial: 125}}
	var out bytes.Buffer
	printBundleReport(&out, "pair", r, false)
	for _, want := range []string{"0123456789ab", "lead", "lead → helper", "125"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("report lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "s1") {
		t.Fatal("skills detailed without --budget")
	}
	out.Reset()
	printBundleReport(&out, "pair", r, true)
	for _, want := range []string{"s1", "150", "50"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("budget lacks %q:\n%s", want, out.String())
		}
	}
}
