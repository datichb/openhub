package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// L4: the session data of oh (reports of workflow_report, transcripts) are
// refused to every agent (layer 0, sessionDataDeny), before the scratch
// exceptions: the owner of a scratch folder still reads and writes it.
func TestSessionDataRefusedScratchKept(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, ohHome, sessions string }{
		{"~/.oh", "", scratchSessions},
		{"custom oh home", "/data/oh", "/data/oh/sessions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := scratchBundle(t, "debug", tc.ohHome, scratchOther)
			scratch := ScratchDir(tc.sessions, "ses_a", "debugger") + "/repro_test.go"
			data := []string{tc.sessions + "/ses_a/reports/rp-1.md", tc.sessions + "/ses_a/transcript.json", tc.sessions + "/ses_b/reports/rp-2.md"}
			for _, plugin := range []bool{true, false} {
				owner := scratchAgentOf(t, b, "debugger", tc.sessions, "ses_a", plugin)
				assert.Equal(t, sessionspec.EffectAllow, owner.file(sessionspec.ActionEdit, scratch), "the owner writes its scratch (plugin %v)", plugin)
				assert.Equal(t, sessionspec.EffectAllow, owner.file(sessionspec.ActionRead, scratch), "the owner reads its scratch (plugin %v)", plugin)
				reader := scratchAgentOf(t, b, "developer", tc.sessions, "ses_a", plugin)
				assert.Equal(t, sessionspec.EffectAllow, reader.file(sessionspec.ActionRead, scratch), "the reader reads (plugin %v)", plugin)
				for _, agent := range []string{"debugger", "developer", "documentarian"} {
					a := scratchAgentOf(t, b, agent, tc.sessions, "ses_a", plugin)
					for _, p := range data {
						assert.Equal(t, sessionspec.EffectDeny, a.file(sessionspec.ActionRead, p), "%s reads %s (plugin %v)", agent, p, plugin)
						assert.Equal(t, sessionspec.EffectDeny, a.file(sessionspec.ActionEdit, p), "%s writes %s (plugin %v)", agent, p, plugin)
						assert.Equal(t, sessionspec.EffectDeny, a.shell("cat "+p), "%s: cat %s", agent, p)
					}
					assert.Equal(t, sessionspec.EffectDeny, a.search(actionGrep, tc.sessions+"/ses_a/reports", "Verdict"), "%s searches the reports", agent)
				}
			}

			// A workflow without scratch folder: the global guard refuses them.
			tb := scratchBundle(t, "ticket", tc.ohHome)
			for _, plugin := range []bool{true, false} {
				dev := scratchAgentOf(t, tb, "reviewer", tc.sessions, "ses_a", plugin)
				for _, p := range data {
					assert.Equal(t, sessionspec.EffectDeny, dev.file(sessionspec.ActionRead, p), "reviewer reads %s (plugin %v)", p, plugin)
					assert.Equal(t, sessionspec.EffectDeny, dev.file(sessionspec.ActionEdit, p), "reviewer writes %s (plugin %v)", p, plugin)
				}
			}
		})
	}
}
