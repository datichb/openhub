package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// unlocks: (piste T, T1): operations locked until a checkpoint is passed.

func TestValidate_Unlocks(t *testing.T) {
	cases := []struct{ name, body, code, path string }{
		{"duplicate", "risk: write\ncheckpoints:\n  cp-1: { unlocks: [commit], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n  cp-2: { unlocks: [close, commit], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", "unlock_duplicate", "checkpoints.cp-2.unlocks[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertDiag(t, checkYAML(t, vHeader+c.body, testEnv()), c.code, c.path, SeverityError)
		})
	}
}

// An operation unknown to this version (written by a newer oh) is a warning,
// not an error: a team on mixed versions keeps working (DC9).
func TestValidate_UnlockUnknownIsWarning(t *testing.T) {
	body := "risk: write\ncheckpoints:\n  cp-1: { unlocks: [deploy, commit], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n"
	diags := checkYAML(t, vHeader+body, testEnv())
	assertDiag(t, diags, "unlock_unknown", "checkpoints.cp-1.unlocks[0]", SeverityWarning)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error: %v", diagList(diags))
		}
	}
}

func TestValidate_UnlockCreateAndApply(t *testing.T) {
	body := "risk: write\ncheckpoints:\n  cp-tickets: { unlocks: [create], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n  cp-apply: { unlocks: [apply], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n"
	for _, d := range checkYAML(t, vHeader+body, testEnv()) {
		if strings.HasPrefix(d.Code, "unlock_") || d.Code == "enum_invalid" {
			t.Fatalf("create and apply are valid operations: %v", d)
		}
	}
}

var hubTicketUnlocks = strings.Replace(hubTicket,
	"cp-2: { label: Commit, mandatory: true,",
	"cp-3: { label: Next, unlocks: [push], mode: { manuel: pause, semi-auto: auto, auto: auto } }\n  cp-2: { label: Commit, mandatory: true, unlocks: [commit, close],", 1)

func TestResolve_UnlocksOnlyAddUp(t *testing.T) {
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

func TestResolve_UnlockingCheckpointCannotBeRemoved(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicketUnlocks,
		"team:ticket": "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-3: { disabled: true }\n",
	})
	_, diags := ResolveSpec(cat, Ref{LayerTeam, "ticket"}, nil)
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"unlocking_checkpoint_removed"}) {
		t.Fatalf("codes = %v", got)
	}
}

func TestResolve_UnlockMovedIsDuplicate(t *testing.T) {
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

// N10 (DC7): push in unlocks opens nothing outside a publishing workflow.
func TestValidate_UnlockPushWithoutPublish(t *testing.T) {
	body := "checkpoints:\n  cp-1: { unlocks: [commit, push], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n"
	assertDiag(t, checkYAML(t, vHeader+"risk: write\n"+body, testEnv()), "unlock_push_without_publish", "checkpoints.cp-1.unlocks[1]", SeverityWarning)
	for _, d := range checkYAML(t, vHeader+"risk: publish\n"+body, testEnv()) {
		if d.Code == "unlock_push_without_publish" {
			t.Fatalf("publish workflow: %v", d)
		}
	}
}

// LD-D: `wiki: sync` (doc-sync) on a checkpoint that unlocks the commit.
func TestValidate_CheckpointWiki(t *testing.T) {
	mode := "mode: { manuel: pause, semi-auto: pause, auto: pause }"
	ok := "risk: write\ncheckpoints:\n  cp-sync: { wiki: sync, unlocks: [commit], " + mode + " }\n"
	for _, d := range checkYAML(t, vHeader+ok, testEnv()) {
		if d.Severity == SeverityError {
			t.Fatalf("valid: %v", d)
		}
	}
	assertDiag(t, checkYAML(t, vHeader+"risk: write\ncheckpoints:\n  cp-sync: { wiki: sync, "+mode+" }\n", testEnv()),
		"checkpoint_wiki_commit", "checkpoints.cp-sync.wiki", SeverityError)
	assertDiag(t, checkYAML(t, vHeader+"risk: write\ncheckpoints:\n  cp-sync: { wiki: all, unlocks: [commit], "+mode+" }\n", testEnv()),
		"enum_invalid", "checkpoints.cp-sync.wiki", SeverityError)
}
