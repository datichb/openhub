package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// unlocks: (piste T, T1): operations locked until a checkpoint is passed.
// The refusals and warnings are rows of TestValidate_Rules.

// An operation unknown to this version (written by a newer oh) is a warning,
// not an error: a team on mixed versions keeps working (DC9).
func TestValidate_UnlockUnknownIsWarning(t *testing.T) {
	t.Parallel()
	body := "risk: write\ncheckpoints:\n  cp-1: { unlocks: [deploy, commit], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n"
	diags := checkYAML(t, vHeader+body, testEnv())
	assertDiag(t, diags, "unlock_unknown", "checkpoints.cp-1.unlocks[0]", SeverityWarning)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error: %v", diagList(diags))
		}
	}
}

// What the unlocks accept: no diagnostic of the kind the case refuses.
func TestValidate_UnlocksAccepted(t *testing.T) {
	t.Parallel()
	pause := "mode: { manuel: pause, semi-auto: pause, auto: pause }"
	isError := func(d Diagnostic) bool { return d.Severity == SeverityError }
	cases := []struct {
		name, body string
		refused    func(Diagnostic) bool
	}{
		{"create and apply are valid operations",
			"risk: write\ncheckpoints:\n  cp-tickets: { unlocks: [create], " + pause + " }\n  cp-apply: { unlocks: [apply], " + pause + " }\n",
			func(d Diagnostic) bool { return strings.HasPrefix(d.Code, "unlock_") || d.Code == "enum_invalid" }},
		{"push in a publish workflow",
			"risk: publish\ncheckpoints:\n  cp-1: { unlocks: [commit, push], " + pause + " }\n",
			func(d Diagnostic) bool { return d.Code == "unlock_push_without_publish" }},
		{"wiki: sync on a checkpoint that unlocks the commit",
			"risk: write\ncheckpoints:\n  cp-sync: { wiki: sync, unlocks: [commit], " + pause + " }\n", isError},
		{"a second commit checkpoint after the first",
			"risk: write\ncheckpoints:\n  cp-a: { unlocks: [commit], " + pause + " }\n  cp-b: { " + pause + " }\n  cp-c: { unlocks: [commit], unlock_again: true, " + pause + " }\n", isError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, d := range checkYAML(t, vHeader+c.body, testEnv()) {
				if c.refused(d) {
					t.Fatalf("refused: %v", d)
				}
			}
		})
	}
}

// p5-fixes: the earlier commit checkpoint knows the later (CommitAgain); the
// repeated commit adds no irreversible pause to the budget.
func TestSpec_UnlockAgainCommitAgain(t *testing.T) {
	t.Parallel()
	pause := "mode: { manuel: pause, semi-auto: pause, auto: pause }"
	ok := "risk: write\ncheckpoints:\n  cp-a: { unlocks: [commit], " + pause + " }\n  cp-b: { " + pause + " }\n  cp-c: { unlocks: [commit], unlock_again: true, " + pause + " }\n"
	doc, pd := Parse([]byte(vHeader+ok), Source{Layer: LayerHub, Path: "x.yaml"})
	if pd.HasErrors() {
		t.Fatal(pd)
	}
	s := doc.Spec
	if got := s.CommitAgain("cp-a"); got != "cp-c" {
		t.Fatalf("CommitAgain(cp-a) = %q", got)
	}
	if s.CommitAgain("cp-c") != "" || s.CommitAgain("cp-b") != "" {
		t.Fatal("only the earlier commit checkpoint has a later one")
	}
	if b := s.PauseBudget(); !reflect.DeepEqual(b.Irreversible, []string{"cp-a"}) || b.Budget != BudgetDecisionPauses+1 {
		t.Fatalf("budget = %+v", b)
	}
}

var hubTicketUnlocks = strings.Replace(hubTicket,
	"cp-2: { label: Commit, mandatory: true,",
	"cp-3: { label: Next, unlocks: [push], mode: { manuel: pause, semi-auto: auto, auto: auto } }\n  cp-2: { label: Commit, mandatory: true, unlocks: [commit, close],", 1)

func TestResolve_UnlocksOnlyAddUp(t *testing.T) {
	t.Parallel()
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicketUnlocks,
		"team:ticket": "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-1: { unlocks: [] }\n  cp-2: { unlocks: [close] }\n",
	})
	r := resolveOK(t, cat, "team:ticket")
	cp2, _ := r.Spec.Checkpoints.Get("cp-2")
	if !reflect.DeepEqual(cp2.Unlocks, []UnlockOp{UnlockCommit, UnlockClose}) {
		t.Fatalf("unlocks = %v (an inherited lock is kept)", cp2.Unlocks)
	}
}

func TestResolve_UnlockMovedIsDuplicate(t *testing.T) {
	t.Parallel()
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicketUnlocks,
		"team:ticket": "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-1: { unlocks: [commit] }\n",
	})
	_, diags := Check(cat, Ref{LayerTeam, "ticket"}, nil, testEnv())
	for _, d := range diags {
		if d.Code == "unlock_duplicate" {
			return
		}
	}
	t.Fatalf("unlocking commit earlier must be refused: %v", diagList(diags))
}

// p5-fixes: only the hub layer sets unlock_again (a commit unlocked by one
// more checkpoint): a team layer that sets it, on an inherited checkpoint
// or a new one, is a loosening.
func TestPatch_UnlockAgainHubOnly(t *testing.T) {
	t.Parallel()
	for _, patch := range []string{
		"checkpoints:\n  cp-1: { unlocks: [commit], unlock_again: true }\n",
		"checkpoints:\n  cp-9: { unlocks: [commit], unlock_again: true, mode: { manuel: pause, semi-auto: pause, auto: pause } }\n",
	} {
		_, diags := layeredDiags(t, patch)
		found := false
		for _, d := range diags {
			found = found || (d.Code == "loosening" && strings.HasSuffix(d.Path, ".unlock_again"))
		}
		if !found {
			t.Fatalf("%q: diags = %v", patch, diagList(diags))
		}
	}
}
