package parallel

import "testing"

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		input    []string
		expected int
	}{
		{[]string{"a", "b", "c"}, 3},
		{[]string{"a", "a", "b"}, 2},
		{[]string{"a", "a", "a"}, 1},
		{nil, 0},
		{[]string{}, 0},
	}

	for i, tc := range tests {
		result := uniqueStrings(tc.input)
		if len(result) != tc.expected {
			t.Errorf("test %d: expected %d unique, got %d", i, tc.expected, len(result))
		}
	}
}

func TestClassifyConflictSeverity(t *testing.T) {
	tests := []struct {
		file     string
		expected string
	}{
		{"src/auth/service.ts", "high"},
		{"package-lock.json", "low"},      // contains "lock"
		{"config/settings.toml", "medium"},
		{"yarn.lock", "low"},
		{"Gemfile.lock", "low"},
		{"tsconfig.json", "medium"},
		{"src/main.go", "high"},
	}

	for _, tc := range tests {
		result := classifyConflictSeverity(tc.file)
		if result != tc.expected {
			t.Errorf("file=%s: expected %s, got %s", tc.file, tc.expected, result)
		}
	}
}

func TestSeverityRank(t *testing.T) {
	if severityRank("high") <= severityRank("medium") {
		t.Error("high should rank above medium")
	}
	if severityRank("medium") <= severityRank("low") {
		t.Error("medium should rank above low")
	}
	if severityRank("low") <= severityRank("unknown") {
		t.Error("low should rank above unknown")
	}
}

func TestDetectConflicts(t *testing.T) {
	state := NewState("/tmp", 3)
	state.AddSession(SessionInfo{
		TicketID:      "bd-42",
		Status:        StatusRunning,
		FilesModified: []string{"src/config/index.ts", "src/auth/service.ts"},
	})
	state.AddSession(SessionInfo{
		TicketID:      "bd-43",
		Status:        StatusRunning,
		FilesModified: []string{"src/config/index.ts", "src/api/routes.ts"},
	})
	state.AddSession(SessionInfo{
		TicketID:      "bd-44",
		Status:        StatusRunning,
		FilesModified: []string{"src/utils/helpers.ts"},
	})

	ctx := NewSharedContext(state)
	ctx.detectConflicts()

	conflicts := ctx.GetConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].File != "src/config/index.ts" {
		t.Errorf("expected conflict on src/config/index.ts, got %s", conflicts[0].File)
	}
	if len(conflicts[0].Sessions) != 2 {
		t.Errorf("expected 2 sessions in conflict, got %d", len(conflicts[0].Sessions))
	}
}

func TestDetectConflicts_NoConflict(t *testing.T) {
	state := NewState("/tmp", 3)
	state.AddSession(SessionInfo{
		TicketID:      "bd-42",
		Status:        StatusRunning,
		FilesModified: []string{"src/auth/service.ts"},
	})
	state.AddSession(SessionInfo{
		TicketID:      "bd-43",
		Status:        StatusRunning,
		FilesModified: []string{"src/api/routes.ts"},
	})

	ctx := NewSharedContext(state)
	ctx.detectConflicts()

	conflicts := ctx.GetConflicts()
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflicts_IgnoreNonRunning(t *testing.T) {
	state := NewState("/tmp", 3)
	state.AddSession(SessionInfo{
		TicketID:      "bd-42",
		Status:        StatusRunning,
		FilesModified: []string{"src/shared.ts"},
	})
	state.AddSession(SessionInfo{
		TicketID:      "bd-43",
		Status:        StatusPending, // not running — should be ignored
		FilesModified: []string{"src/shared.ts"},
	})

	ctx := NewSharedContext(state)
	ctx.detectConflicts()

	conflicts := ctx.GetConflicts()
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts (pending sessions ignored), got %d", len(conflicts))
	}
}
