package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// CP-R5 (R3): the card shows the verification run by oh, the evidence and
// the unresolved ticket; a failed verification offers the override.
func TestCheckpointCardVerify(t *testing.T) {
	failed := CheckpointVerifyView{Status: verifyFail, Summary: "échouée — go test ./... ✗ (1)", Excerpt: "--- FAIL: TestSub\nFAIL calc",
		Evidence: []CheckpointEvidenceView{
			{Cmd: "go test ./...", Observed: true},
			{Cmd: "npm test"},
			{Cmd: "go vet ./...", Observed: true, Contradicted: true},
		}, Unresolved: "pt-1"}
	body := checkpointBody(CheckpointCardView{Summary: "prêt", Verify: failed})
	for _, want := range []string{"go test ./... ✗ (1)", "--- FAIL: TestSub", "npm test", "pt-1"} {
		assert.Contains(t, body, want)
	}
	assert.Equal(t, checkpointFix, checkpointDefaultChoice(failed))
	extra := checkpointExtraChoices(failed)
	assert.Len(t, extra, 1)
	assert.Equal(t, checkpointOverride, extra[0].Value)

	passed := CheckpointVerifyView{Status: verifyPass, Summary: "réussie", Excerpt: "noise"}
	body = checkpointBody(CheckpointCardView{Verify: passed})
	assert.Contains(t, body, "réussie")
	assert.NotContains(t, body, "noise", "no excerpt when it passed")
	assert.Empty(t, checkpointExtraChoices(passed))
	assert.Equal(t, checkpointApprove, checkpointDefaultChoice(passed))
	assert.Equal(t, checkpointBody(CheckpointCardView{}), checkpointBody(CheckpointCardView{Verify: CheckpointVerifyView{}}), "nothing without a verification")

	// R3-remote: a verification that cannot run is passed only overridden.
	blocked := CheckpointVerifyView{Status: verifyUnavailable, Summary: "impossible ici — emplacement de travail introuvable"}
	body = checkpointBody(CheckpointCardView{Verify: blocked})
	assert.Contains(t, body, "emplacement de travail introuvable")
	assert.True(t, blocked.Failed())
	assert.Equal(t, checkpointFix, checkpointDefaultChoice(blocked), "never passed by default")
	extra = checkpointExtraChoices(blocked)
	assert.Len(t, extra, 1)
	assert.Equal(t, checkpointOverride, extra[0].Value)
}
