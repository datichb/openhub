package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// staleDays: a ticket in progress not updated for this many days was
// probably left (crashed session, forgotten claim).
const staleDays = 3

// beadsTrackingChecks reports, per Beads project, the work that needs a
// follow-up: open tickets named in commits (bd orphans), tickets in
// progress left for days (bd stale), epics whose children are all closed
// (bd epic status). Warnings only: oh changes nothing.
func beadsTrackingChecks() []doctor.Result {
	a := TryApp()
	if a == nil || a.Projects == nil {
		return nil
	}
	projects, err := a.Projects.List(context.Background(), "")
	if err != nil {
		return nil
	}
	var out []doctor.Result
	checked := 0
	for _, p := range projects {
		if p.Path == "" || !beads.IsInitialized(p.Path) {
			continue
		}
		checked++
		orphans, oerr := beads.Orphans(p.Path)
		stale, serr := beads.Stale(p.Path, "in_progress", staleDays)
		epics, eerr := beads.EpicStatus(p.Path, true)
		out = append(out, beadsTrackingResult(p.Name, orphans, stale, epics, firstErr(oerr, serr, eerr))...)
	}
	if len(out) == 0 && checked > 0 {
		out = append(out, doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.tracking.ok", checked)})
	}
	return out
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// beadsTrackingResult turns the follow-up of one project into checks.
func beadsTrackingResult(project string, orphans []beads.Orphan, stale []beads.Ticket, epics []beads.EpicProgress, err error) []doctor.Result {
	var out []doctor.Result
	warn := func(key string, args ...any) {
		out = append(out, doctor.Result{Name: i18n.Tf("cmd.doctor.tracking.name", project), OK: true, Warn: true, Detail: i18n.Tf(key, args...)})
	}
	if err != nil {
		warn("cmd.doctor.tracking.failed", err.Error())
	}
	if len(orphans) > 0 {
		ids := make([]string, 0, len(orphans))
		for _, o := range orphans {
			ids = append(ids, fmt.Sprintf("%s (%s)", o.ID, o.Commit))
		}
		warn("cmd.doctor.tracking.orphans", len(orphans), shortList(ids))
	}
	if len(stale) > 0 {
		ids := make([]string, 0, len(stale))
		for _, t := range stale {
			ids = append(ids, t.ID)
		}
		warn("cmd.doctor.tracking.stale", len(stale), staleDays, shortList(ids))
	}
	if len(epics) > 0 {
		ids := make([]string, 0, len(epics))
		for _, e := range epics {
			ids = append(ids, e.Epic.ID)
		}
		warn("cmd.doctor.tracking.epics", len(epics), shortList(ids))
	}
	return out
}

// shortList shows the first five entries.
func shortList(items []string) string {
	if len(items) <= 5 {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:5], ", ") + " " + i18n.Tf("cmd.doctor.beads.more", len(items)-5)
}
