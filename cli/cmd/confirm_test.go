package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// U-11: one function reads a confirmation, with the same words everywhere.
func TestConfirmed(t *testing.T) {
	for _, yes := range []string{"o", "O", "oui", "Oui", "OUI", "y", "Y", "yes", "Yes", "YES", " oui \n", "\to"} {
		if !confirmed(yes, false) || !confirmed(yes, true) {
			t.Errorf("%q must be a yes", yes)
		}
	}
	for _, no := range []string{"n", "N", "non", "no", "x", "peut-être", "yy", "ouiii", "0"} {
		if confirmed(no, false) || confirmed(no, true) {
			t.Errorf("%q must be a no, even with yes by default", no)
		}
	}
	if !confirmed("", true) || !confirmed("  ", true) || confirmed("", false) {
		t.Error("an empty answer takes the default of the question")
	}
}

// The hint shown with a question follows the language of the hub.
func TestYesNoHint(t *testing.T) {
	defer i18n.SetLocale("en")
	i18n.SetLocale("en")
	if yesNoHint(true) != "[Y/n]" || yesNoHint(false) != "[y/N]" {
		t.Errorf("en: %s %s", yesNoHint(true), yesNoHint(false))
	}
	i18n.SetLocale("fr")
	if yesNoHint(true) != "[O/n]" || yesNoHint(false) != "[o/N]" {
		t.Errorf("fr: %s %s", yesNoHint(true), yesNoHint(false))
	}
}

// askYes (oh mcp trust) goes through the same reader.
func TestPromptsAcceptEveryYes(t *testing.T) {
	for _, answer := range []string{"o", "oui", "y", "yes", "OUI", "Yes"} {
		if !askYes(strings.NewReader(answer+"\n"), &bytes.Buffer{}, "ok ?") {
			t.Errorf("askYes(%q) = false", answer)
		}
	}
	if askYes(strings.NewReader("non\n"), &bytes.Buffer{}, "ok ?") || askYes(strings.NewReader("\n"), &bytes.Buffer{}, "ok ?") {
		t.Error("askYes: no, and empty = no")
	}
}

// U-11: the prompts of the CLI do not compare an answer with "y" on their own.
func TestNoPrivateConfirmationReader(t *testing.T) {
	for _, f := range []string{"init.go", "claim.go", "secrets.go", "review_feedback.go", "team_config.go", "mcp_trust.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{`== "y"`, `!= "y"`, `"Y"`, `== "oui"`, `== "non"`} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s reads an answer by itself (%s): use confirmed()", f, bad)
			}
		}
	}
}
