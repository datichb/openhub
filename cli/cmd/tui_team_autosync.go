package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// syncSummary is the one-line summary of a tracker sync, for a toast.
func syncSummary(r *tracker.SyncResult) string {
	return i18n.Tf("v54y.sync.summary", r.ClaimsCreated, r.ClaimsUpdated, r.ClaimsReleased, len(r.Warnings)+len(r.Errors))
}

// syncChangedSomething reports whether a sync did, or said, anything: an
// automatic sync that did not is not worth a toast.
func syncChangedSomething(r *tracker.SyncResult) bool {
	return r.ClaimsCreated+r.ClaimsUpdated+r.ClaimsReleased+r.ClaimsExpired+r.LabelsPushed+len(r.Warnings)+len(r.Errors) > 0
}

// autoSyncForTUI is the automatic tracker sync of the opening of the board. It
// is the same as the one of the daemon: only when the team turned auto_sync
// on, not while another member's sync holds the lease of the interval, not
// once the server refused the token three times in a row. The cases where
// there is nothing to do give no summary and no error.
func autoSyncForTUI(a *app.App, ctx context.Context) (string, error) {
	res, err := teamService(a).SyncTracker(ctx, teamsvc.SyncOptions{Auto: true})
	var held *tracker.LeaseHeldError
	switch {
	case err == nil:
		if syncChangedSomething(res) {
			return syncSummary(res), nil
		}
		return "", nil
	case errors.As(err, &held),
		errors.Is(err, tracker.ErrAutoSyncDisabled),
		errors.Is(err, tracker.ErrAutoSyncMuted),
		errors.Is(err, teamsvc.ErrTeamNotConfigured),
		errors.Is(err, teamsvc.ErrNotCloned),
		errors.Is(err, teamsvc.ErrTrackerNotConfigured),
		errors.Is(err, teamsvc.ErrTrackerDisabled),
		errors.Is(err, teamsvc.ErrNoProjects):
		return "", nil
	}
	return "", syncTrackerTUIError(err)
}
