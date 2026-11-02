package cmd

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// Labels and prompts that stayed in English in French: the menu entries of the
// team mode, the board hints and the « Choice [1]: » prompts of the rejoin
// wizard.
func TestFrenchLabelsAreNotEnglish(t *testing.T) {
	keys := []string{
		"cmd.team.init.step_policies", "cmd.team.init.recap.policies", "tui.team.policies", "tui.tm.item.policies",
		"tui.tm.item.sync_tracker", "tui.cmd.sync_tracker", "cmd.board.status_hints", "cmd.init.wizard_choice_prompt",
	}
	i18ntest.Set(t, "en")
	en := map[string]string{}
	for _, k := range keys {
		en[k] = i18n.Tf(k, 1)
	}
	i18ntest.Set(t, "fr")
	for _, k := range keys {
		if fr := i18n.Tf(k, 1); fr == en[k] {
			t.Errorf("%s is English in French: %q", k, fr)
		}
	}
	if p := i18n.Tf("cmd.init.wizard_choice_prompt", 2); !strings.Contains(p, "[2]") {
		t.Errorf("the prompt shows the default: %q", p)
	}
}
