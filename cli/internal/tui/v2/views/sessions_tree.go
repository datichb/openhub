package views

import (
	"sort"
	"time"

	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Fan-out in the Sessions list (modernisation tui-sessions): the children,
// mergers and fixers (↳) of a parent (⑂) shown in the list are grouped
// under it, as a tree, in the section of their parent; a group goes to « À
// traiter » when one of its sessions waits for a decision, each decision
// under its session.

// fanoutGroups takes out of rows the sessions whose parent is in rows, by
// parent (their order is kept).
func fanoutGroups(rows []SessionRow) (top []SessionRow, kids map[string][]SessionRow) {
	parents := map[string]bool{}
	for _, r := range rows {
		if r.FanoutRole == FanoutRoleParent {
			parents[r.ID] = true
		}
	}
	kids = map[string][]SessionRow{}
	for _, r := range rows {
		if r.FanoutParent != "" && r.FanoutParent != r.ID && parents[r.FanoutParent] {
			kids[r.FanoutParent] = append(kids[r.FanoutParent], r)
			continue
		}
		top = append(top, r)
	}
	return top, kids
}

// oldestDecision is the creation of the oldest decision of a group (zero
// without any).
func oldestDecision(r SessionRow, kids []SessionRow) (time.Time, bool) {
	var at time.Time
	found := false
	for _, s := range append([]SessionRow{r}, kids...) {
		for _, d := range s.Decisions {
			if !found || d.Created.Before(at) {
				at, found = d.Created, true
			}
		}
	}
	return at, found
}

// sessionsSectionsGrouped is sessionsSections for groups: the section of a
// group is the one of its parent, « À traiter » when one of its sessions
// waits for a decision.
func sessionsSectionsGrouped(rows []SessionRow, kids map[string][]SessionRow, now time.Time) (inbox, running, sleeping, finished []SessionRow) {
	for _, r := range rows {
		_, waiting := oldestDecision(r, kids[r.ID])
		switch {
		case waiting && !r.Finished:
			r.Decisions = sortedDecisions(r.Decisions)
			inbox = append(inbox, r)
		case waiting && len(r.Decisions) == 0:
			// A finished parent whose children still wait.
			inbox = append(inbox, r)
		case r.Finished:
			if now.Sub(r.Changed) <= sessionsFinishedFor || r.Changed.IsZero() {
				finished = append(finished, r)
			}
		case r.State == "sleeping":
			sleeping = append(sleeping, r)
		default:
			running = append(running, r)
		}
	}
	sort.SliceStable(inbox, func(i, j int) bool {
		a, _ := oldestDecision(inbox[i], kids[inbox[i].ID])
		b, _ := oldestDecision(inbox[j], kids[inbox[j].ID])
		return a.Before(b)
	})
	return inbox, running, sleeping, finished
}

func sortedDecisions(list []SessionDecision) []SessionDecision {
	out := append([]SessionDecision(nil), list...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out
}

// treePrefix is the branch drawn before a session of a group.
func treePrefix(last bool) string {
	b := "├"
	if last {
		b = "└"
	}
	return muted(b) + " "
}

// groupItems are the rows of a session and of its group: the children
// under it, and in « À traiter » the decisions of each one.
func groupItems(r SessionRow, kids []SessionRow, inbox bool) []widgets.SectionItem {
	items := []widgets.SectionItem{sessionRowItem(r, inbox)}
	if inbox {
		for _, d := range sortedDecisions(r.Decisions) {
			items = append(items, decisionItem(d))
		}
	}
	for i, k := range kids {
		last := i == len(kids)-1
		it := sessionRowItem(k, inbox && len(k.Decisions) > 0)
		it.MainText = "  " + treePrefix(last) + it.MainText
		it.SecondaryText = "    " + it.SecondaryText
		items = append(items, it)
		if !inbox {
			continue
		}
		pipe := "│"
		if last {
			pipe = " "
		}
		for _, d := range sortedDecisions(k.Decisions) {
			di := decisionItem(d)
			di.MainText = "  " + muted(pipe) + "  " + di.MainText
			di.SecondaryText = "    " + di.SecondaryText
			items = append(items, di)
		}
	}
	return items
}

// firstDecisionKey is the key of the oldest decision of a group ("" =
// none): the cursor lands on it.
func firstDecisionKey(r SessionRow, kids []SessionRow) string {
	var best *SessionDecision
	for _, s := range append([]SessionRow{r}, kids...) {
		for i := range s.Decisions {
			if best == nil || s.Decisions[i].Created.Before(best.Created) {
				best = &s.Decisions[i]
			}
		}
	}
	if best == nil {
		return ""
	}
	return "d:" + best.ID
}
