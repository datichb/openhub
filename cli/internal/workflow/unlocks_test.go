package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// unlocks: (piste T, T1): operations locked until a checkpoint is passed.

func TestValidate_Unlocks(t *testing.T) {
	cases := []struct{ name, body, code, path string }{
		{"unknown", "risk: write\ncheckpoints:\n  cp-1: { unlocks: [merge], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", "enum_invalid", "checkpoints.cp-1.unlocks[0]"},
		{"duplicate", "risk: write\ncheckpoints:\n  cp-1: { unlocks: [commit], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n  cp-2: { unlocks: [close, commit], mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", "unlock_duplicate", "checkpoints.cp-2.unlocks[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertDiag(t, checkYAML(t, vHeader+c.body, testEnv()), c.code, c.path, SeverityError)
		})
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
