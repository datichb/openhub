package cmd

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	"github.com/datichb/openhub/cli/internal/runsvc"
)

// Preconditions of a launch with a suggested workflow (`suggest`, piste E):
// the user may run the suggested workflow first; with `resume: true` the
// original launch is remembered for the session that runs first and offered
// again when it ends (« Enchaîner avec… », end-of-session toast).

// preconditionSuggestion is a failed `suggest` precondition of a launch.
type preconditionSuggestion struct {
	Label    string
	Workflow string
	Resume   bool
}

// resumeIntent is a launch to propose again after a session (stored in the
// preferences of the project, key resumeKey(session)).
type resumeIntent struct {
	Workflow string            `json:"workflow"`
	Inputs   map[string]string `json:"inputs,omitempty"`
	Tickets  []string          `json:"tickets,omitempty"`
	Mode     string            `json:"mode,omitempty"`
	Runtime  string            `json:"runtime,omitempty"`
	Location string            `json:"location,omitempty"`
}

func resumeKey(sessionID string) string { return "resume:" + sessionID }

// runSuggestedFirst starts the suggested workflow of a launch (same project,
// runtime, location and opening) and, when s.Resume, remembers the original
// launch for the sessions it started.
func runSuggestedFirst(ctx context.Context, a *app.App, orig runOptions, s preconditionSuggestion, ui launcher.LaunchUI) ([]*runsvc.StartResult, error) {
	first := runOptions{Workflow: s.Workflow, Project: orig.Project, Provider: orig.Provider, Runtime: orig.Runtime,
		Location: orig.Location, Attach: orig.Attach, Progress: orig.Progress}
	p, err := prepareWorkflowRun(ctx, a, first, io.Discard)
	if err != nil {
		return nil, err
	}
	results, err := p.start(ctx, a, ui)
	if err != nil || !s.Resume {
		return results, err
	}
	intent := resumeIntent{Workflow: orig.Workflow, Inputs: orig.Inputs, Tickets: orig.Tickets, Mode: orig.Mode,
		Runtime: orig.Runtime, Location: orig.Location}
	for _, r := range results {
		saveResumeIntent(ctx, a, orig.Project.ID, r.SessionID, intent)
	}
	return results, nil
}

func saveResumeIntent(ctx context.Context, a *app.App, projectID, sessionID string, intent resumeIntent) {
	if a.Preferences == nil {
		return
	}
	scope := domain.ProjectPreferenceScope(projectID)
	if err := prefsvc.New(a.Preferences, nil).Set(ctx, scope, resumeKey(sessionID), intent); err != nil {
		slog.Warn("resume intent not saved", "session", sessionID, "error", err)
	}
}

// loadResumeIntent returns the launch to propose again after a session.
func loadResumeIntent(ctx context.Context, a *app.App, projectID, sessionID string) (*resumeIntent, bool) {
	if a.Preferences == nil {
		return nil, false
	}
	var intent resumeIntent
	ok, err := prefsvc.New(a.Preferences, nil).Get(ctx, domain.ProjectPreferenceScope(projectID), resumeKey(sessionID), &intent)
	if err != nil || !ok || intent.Workflow == "" {
		return nil, false
	}
	return &intent, true
}

// forgetResumeIntent removes a consumed intent.
func forgetResumeIntent(ctx context.Context, a *app.App, projectID, sessionID string) {
	if a.Preferences != nil {
		_ = a.Preferences.Delete(ctx, domain.ProjectPreferenceScope(projectID), resumeKey(sessionID))
	}
}

// resumeCommand is the command line of an intent (CLI hint).
func resumeCommand(i resumeIntent) string {
	s := "oh run " + i.Workflow
	for _, k := range sortedStringKeys(i.Inputs) {
		s += " -i " + k + "=" + shellQuote(i.Inputs[k])
	}
	if len(i.Tickets) > 0 {
		s += " --tickets " + strings.Join(i.Tickets, ",")
	}
	return s
}
