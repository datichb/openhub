package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/domain"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

func TestEndedSessions(t *testing.T) {
	ts := &tuiSessions{ends: map[string]string{}}
	view := func(id string, state domain.RunState, outputs map[string]any, typ domain.SessionType) sessionsvc.View {
		return sessionsvc.View{Session: domain.Session{ID: id, WorkflowID: "ticket", State: state, Outputs: outputs, Type: typ}}
	}
	// First refresh: what already ended before the TUI is not announced.
	if got := ts.endedSessions([]sessionsvc.View{view("a", domain.RunStopped, nil, ""), view("b", domain.RunActive, nil, "")}); len(got) != 0 {
		t.Fatalf("first refresh: %v", got)
	}
	ts.primed = true
	got := ts.endedSessions([]sessionsvc.View{view("a", domain.RunStopped, nil, ""), view("b", domain.RunStopped, nil, "")})
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("b just ended: %v", got)
	}
	got = ts.endedSessions([]sessionsvc.View{view("b", domain.RunStopped, map[string]any{"branch": "feat/x"}, "")})
	if len(got) != 1 {
		t.Fatalf("new outputs: %v", got)
	}
	if got = ts.endedSessions([]sessionsvc.View{view("b", domain.RunStopped, map[string]any{"branch": "feat/x"}, "")}); len(got) != 0 {
		t.Fatalf("nothing new: %v", got)
	}
	ts.endedSessions([]sessionsvc.View{view("h", domain.RunActive, nil, domain.SessionTypeHeadless)})
	if got = ts.endedSessions([]sessionsvc.View{view("h", domain.RunStopped, nil, domain.SessionTypeHeadless)}); len(got) != 0 {
		t.Fatalf("headless sessions are not announced: %v", got)
	}
}
