package parallel

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxSessions != 3 {
		t.Errorf("expected MaxSessions=3, got %d", cfg.MaxSessions)
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
	cfg := Config{MaxSessions: 0, PortRangeStart: 0}
	cfg.Validate()

	if cfg.MaxSessions != 3 {
		t.Errorf("expected MaxSessions clamped to 3, got %d", cfg.MaxSessions)
	}
	if cfg.PortRangeStart != 4100 {
		t.Errorf("expected PortRangeStart clamped to 4100, got %d", cfg.PortRangeStart)
	}
}

func TestValidate_MaxCap(t *testing.T) {
	cfg := Config{MaxSessions: 20}
	cfg.Validate()

	if cfg.MaxSessions != 10 {
		t.Errorf("expected MaxSessions capped at 10, got %d", cfg.MaxSessions)
	}
}

func TestValidate_NeverAllowExternalMerge(t *testing.T) {
	cfg := Config{AutoMergeExt: true}
	cfg.Validate()

	if cfg.AutoMergeExt {
		t.Error("AutoMergeExt should always be forced to false")
	}
}
