package task

import "testing"

func TestDisplayName_Label(t *testing.T) {
	tk := Task{ID: "bd-42", Label: "Fix auth bug"}
	if tk.DisplayName() != "Fix auth bug" {
		t.Errorf("expected label, got %s", tk.DisplayName())
	}
}

func TestDisplayName_FallbackID(t *testing.T) {
	tk := Task{ID: "bd-42"}
	if tk.DisplayName() != "bd-42" {
		t.Errorf("expected ID fallback, got %s", tk.DisplayName())
	}
}

func TestIsBeads(t *testing.T) {
	tk := Task{Kind: KindTicket, Metadata: map[string]string{"beads": "true"}}
	if !tk.IsBeads() {
		t.Error("expected IsBeads=true for ticket with beads metadata")
	}

	tk2 := Task{Kind: KindTicket}
	if tk2.IsBeads() {
		t.Error("expected IsBeads=false without metadata")
	}

	tk3 := Task{Kind: KindSweep, Metadata: map[string]string{"beads": "true"}}
	if tk3.IsBeads() {
		t.Error("expected IsBeads=false for sweep kind")
	}
}

func TestIsMergeable(t *testing.T) {
	beads := Task{Kind: KindTicket, Metadata: map[string]string{"beads": "true"}}
	if !beads.IsMergeable() {
		t.Error("beads ticket should be mergeable")
	}

	sweep := Task{Kind: KindSweep}
	if !sweep.IsMergeable() {
		t.Error("sweep task should be mergeable")
	}

	custom := Task{Kind: KindCustom}
	if custom.IsMergeable() {
		t.Error("custom task should not be mergeable")
	}

	nonBeadsTicket := Task{Kind: KindTicket}
	if nonBeadsTicket.IsMergeable() {
		t.Error("non-beads ticket should not be mergeable")
	}
}

func TestTicketsToTasks(t *testing.T) {
	estimates := map[string]int{"bd-42": 30, "bd-43": 120}
	tasks := TicketsToTasks([]string{"bd-42", "bd-43", "bd-44"}, estimates, "bd-42")

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// bd-42: priority, estimate 30, beads
	if tasks[0].ID != "bd-42" || !tasks[0].Priority || tasks[0].EstimateMinutes != 30 || !tasks[0].IsBeads() {
		t.Errorf("bd-42: wrong fields: priority=%v est=%d beads=%v", tasks[0].Priority, tasks[0].EstimateMinutes, tasks[0].IsBeads())
	}

	// bd-43: not priority, estimate 120
	if tasks[1].Priority || tasks[1].EstimateMinutes != 120 {
		t.Errorf("bd-43: wrong fields: priority=%v est=%d", tasks[1].Priority, tasks[1].EstimateMinutes)
	}

	// bd-44: not priority, no estimate (0)
	if tasks[2].Priority || tasks[2].EstimateMinutes != 0 {
		t.Errorf("bd-44: wrong fields: priority=%v est=%d", tasks[2].Priority, tasks[2].EstimateMinutes)
	}
}
