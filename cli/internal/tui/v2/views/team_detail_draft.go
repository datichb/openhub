package views

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The team detail keeps two kinds of edit apart, and says so:
//
//   - what is shared with the team (config.toml of the team-state) is a draft:
//     it is published as a whole with Ctrl+S, and leaving the view with a
//     draft asks (Esc) or keeps it (a link to MCP, the models, the workflows:
//     the draft is put back on the fresh config when the view comes back);
//   - what is personal (hub.toml, this team's section included) is written by
//     itself a moment after each edit, as the Settings do.
//
// The profile of the member and the fields of the team in hub.toml (name,
// member id, default) are written at once, outside the draft.

// TeamInfo is the entry of a team in hub.toml, as the detail shows it.
type TeamInfo struct {
	Name      string
	StateRepo string
	StatePath string
	MemberID  string
	Default   bool
}

// The fields of the team that SetTeamField takes.
const (
	TeamFieldName     = "name"
	TeamFieldMemberID = "member_id"
	TeamFieldDefault  = "default"
)

// keptDraft is the unpublished draft of a visit to the detail of a team that
// left without publishing it.
type keptDraft struct {
	teamID string
	base   *teamstate.TeamConfig // the config the draft started from
	draft  *teamstate.TeamConfig
}

// putBackDraft applies the kept draft on the config that was just read: what
// the member changed since they started is set again on top of the latest
// version. A setting a colleague changed meanwhile that the draft also touched
// drops the draft, and says so.
func (v *TeamDetailView) putBackDraft() {
	k := v.kept
	v.kept = nil
	if k == nil || v.teamCfg == nil {
		return
	}
	patch := teamstate.DiffConfig(k.base, k.draft)
	if patch.Empty() {
		return
	}
	if err := patch.Apply(v.teamCfg); err != nil {
		if c, ok := teamstate.IsConfigConflict(err); ok && v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("v54c.save.draft_dropped", strings.Join(c.Keys, ", ")), false)
		}
		return
	}
	v.dirtyTeam = true
}

// GuardLeave is asked by the shell before Esc leaves the view: with a draft
// that is not published, the member chooses to publish it and leave, to
// drop it and leave, or to stay. It returns whether it took charge (the shell
// then waits for proceed).
func (v *TeamDetailView) GuardLeave(proceed func()) bool {
	if v.shell == nil || !v.dirtyTeam || v.cfgErr != nil {
		return false
	}
	keys := strings.Join(teamstate.DiffConfig(v.baseTeam, v.teamCfg).Keys(), ", ")
	v.shell.ShowScrollableModal(i18n.T("v54c.leave.title"), i18n.Tf("v54c.leave.body", keys), []ModalAction{
		{Label: i18n.T("v54c.leave.publish"), Callback: func() {
			v.afterWrite = func(ok bool) {
				if ok {
					proceed()
				}
			}
			v.save()
		}},
		{Label: i18n.T("v54c.leave.drop"), Danger: true, Callback: func() {
			v.dirtyTeam = false
			v.kept = nil
			proceed()
		}},
		{Label: i18n.T("v54c.leave.stay"), Callback: func() {}, Separator: true},
	})
	return true
}

// LeaveGuard is implemented by a view that must not be left silently by Esc
// (edits that are not saved yet): the shell asks it first.
type LeaveGuard interface {
	// GuardLeave takes charge of the departure when it returns true, and runs
	// proceed once the member agreed; false: nothing to ask, leave.
	GuardLeave(proceed func()) bool
}

var _ LeaveGuard = (*TeamDetailView)(nil)
