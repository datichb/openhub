package bundle

import (
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Method of the session (R7): the bundle holds the checkpoints of its
// method only (cp-seams and cp-red for tdd), so that a session of another
// method gets the same bundle as a workflow without them.

// applyMethod keeps in req.Spec the checkpoints of req.Method. A method no
// checkpoint is declared for changes nothing: it is dropped (same bundle).
func (req *Request) applyMethod() {
	if req.Method.Standard() || !req.Spec.HasMethod(req.Method) {
		req.Method = ""
	}
	req.Spec = req.Spec.ForMethod(req.Method)
}

// contractMethod is the method line of the session contract: the method,
// what it asks (methodText) and its checkpoints with the agents that pass
// them, in order.
type contractMethod struct {
	ID, Text string
	Steps    []contractMethodStep
}

type contractMethodStep struct {
	ID string
	By []string
	// Detail: what the step asks and what oh checks, for the agents that
	// pass it ("" for the others).
	Detail string
}

// methodText says what a method asks (generated text, generatedLang).
var methodText = map[workflow.Method]string{
	workflow.MethodTDD: "les tests s'écrivent avant le code : points d'injection et tests prévus présentés, puis tests qui échouent vérifiés par oh, puis implémentation",
}

// methodContract returns the method line of the contract of agent id: for
// the agents that pass a checkpoint of the method and those that launch
// them (zero for the others, and when the session has no method).
func methodContract(req Request, wf *sessionspec.WorkflowRuntime, id string, callees []string) contractMethod {
	if req.Method == "" || req.Spec == nil || wf == nil {
		return contractMethod{}
	}
	concerned := false
	m := contractMethod{ID: string(req.Method), Text: methodText[req.Method]}
	for _, c := range wf.Checkpoints {
		cp, ok := req.Spec.Checkpoints.Get(c.ID)
		if !ok || cp.Method != req.Method {
			continue
		}
		step := contractMethodStep{ID: c.ID, By: slices.Clone(c.By)}
		for _, a := range c.By {
			concerned = concerned || slices.Contains(callees, a)
		}
		if slices.Contains(c.By, id) {
			concerned = true
			var parts []string
			if d := strings.TrimSpace(cp.Description.Text(generatedLang)); d != "" {
				parts = append(parts, d)
			}
			if t := testCheckText(cp.Verify); t != "" {
				parts = append(parts, t)
			}
			step.Detail = strings.Join(parts, " ")
		}
		m.Steps = append(m.Steps, step)
	}
	if !concerned {
		return contractMethod{}
	}
	return m
}

// testCheckText is what oh checks at a checkpoint verified for a test
// written first (expect: fail, must_touch_tests), for the workflow map
// (generated text, generatedLang); "" for the other verifications, which
// the map does not describe.
func testCheckText(v *workflow.VerifySpec) string {
	if v == nil || (v.ExpectOrPass() != workflow.VerifyFail && !v.MustTouchTests) {
		return ""
	}
	var parts []string
	if v.ExpectOrPass() == workflow.VerifyFail {
		parts = append(parts, "les tests doivent échouer, à l'exécution et non à la compilation (le code doit construire)")
	}
	if v.MustTouchTests {
		t := "les changements"
		if v.Since != "" {
			t += " depuis **" + v.Since + "**"
		}
		t += " doivent toucher au moins un fichier de test"
		switch v.OutsideTests {
		case workflow.OutsideTestsRefuse:
			t += ", et aucun autre fichier"
		case workflow.OutsideTestsWarn:
			t += " (les autres fichiers modifiés sont signalés : n'y mets que les amorces nécessaires, sans implémentation)"
		}
		parts = append(parts, t)
	}
	return "oh lance lui-même la vérification avant de le laisser passer : " + strings.Join(parts, " ; ") + ". Refusé : corrige, puis rappelle-le."
}
