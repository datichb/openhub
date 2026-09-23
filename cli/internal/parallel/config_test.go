package parallel

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxSessions != 5 {
		t.Errorf("expected MaxSessions=5, got %d", cfg.MaxSessions)
	}
	if cfg.MaxBudgetMinutes != 180 {
		t.Errorf("expected MaxBudgetMinutes=180, got %d", cfg.MaxBudgetMinutes)
	}
	if cfg.DefaultTicketWeightMin != 60 {
		t.Errorf("expected DefaultTicketWeightMin=60, got %d", cfg.DefaultTicketWeightMin)
	}
	if cfg.PortRangeStart != 4100 {
		t.Errorf("expected PortRangeStart=4100, got %d", cfg.PortRangeStart)
	}
	if !cfg.AutoMergeBeads {
		t.Error("expected AutoMergeBeads=true")
	}
	if cfg.AutoMergeExt {
		t.Error("expected AutoMergeExt=false")
	}
}

func TestValidate_Clamps(t *testing.T) {
	cfg := Config{MaxSessions: 0, PortRangeStart: 0, DefaultTicketWeightMin: 0}
	cfg.Validate()

	if cfg.MaxSessions != 5 {
		t.Errorf("expected MaxSessions clamped to 5, got %d", cfg.MaxSessions)
	}
	if cfg.PortRangeStart != 4100 {
		t.Errorf("expected PortRangeStart clamped to 4100, got %d", cfg.PortRangeStart)
	}
	if cfg.DefaultTicketWeightMin != 60 {
		t.Errorf("expected DefaultTicketWeightMin clamped to 60, got %d", cfg.DefaultTicketWeightMin)
	}
}

func TestValidate_MaxCap(t *testing.T) {
	cfg := Config{MaxSessions: 20, DefaultTicketWeightMin: 999}
	cfg.Validate()

	if cfg.MaxSessions != 10 {
		t.Errorf("expected MaxSessions capped at 10, got %d", cfg.MaxSessions)
	}
	if cfg.DefaultTicketWeightMin != 480 {
		t.Errorf("expected DefaultTicketWeightMin capped at 480, got %d", cfg.DefaultTicketWeightMin)
	}
}

func TestValidate_NeverAllowExternalMerge(t *testing.T) {
	cfg := Config{AutoMergeExt: true}
	cfg.Validate()

	if cfg.AutoMergeExt {
		t.Error("AutoMergeExt should always be forced to false")
	}
}

func TestValidate_NegativeBudgetClamped(t *testing.T) {
	cfg := Config{MaxBudgetMinutes: -10}
	cfg.Validate()

	if cfg.MaxBudgetMinutes != 0 {
		t.Errorf("expected MaxBudgetMinutes clamped to 0, got %d", cfg.MaxBudgetMinutes)
	}
}

func TestTotalBudget(t *testing.T) {
	cfg := Config{DefaultTicketWeightMin: 60}

	estimates := map[string]int{
		"bd-42": 30,  // XS/S
		"bd-43": 120, // L
		"bd-44": 0,   // unknown -> 60
	}
	tickets := []string{"bd-42", "bd-43", "bd-44"}

	total := cfg.TotalBudget(estimates, tickets)
	if total != 210 { // 30 + 120 + 60
		t.Errorf("expected total=210, got %d", total)
	}
}

func TestTotalBudget_AllUnknown(t *testing.T) {
	cfg := Config{DefaultTicketWeightMin: 60}

	estimates := map[string]int{}
	tickets := []string{"bd-42", "bd-43", "bd-44"}

	total := cfg.TotalBudget(estimates, tickets)
	if total != 180 { // 3 * 60
		t.Errorf("expected total=180, got %d", total)
	}
}

func TestTotalBudget_NilEstimates(t *testing.T) {
	cfg := Config{DefaultTicketWeightMin: 60}

	total := cfg.TotalBudget(nil, []string{"bd-42"})
	if total != 60 {
		t.Errorf("expected total=60, got %d", total)
	}
}
