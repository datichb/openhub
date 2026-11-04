package cmd

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The identity form every team assistant asks (« Créer » and « Rejoindre » of
// oh init, oh team init, « Initialiser l'équipe » and « Rejoindre une équipe »
// of the palette): the ID, the display name, the GitLab handle (optional) and
// the role. One builder, so that the five paths ask the same thing and write
// the same card.

// teamRoles are the roles of a member, in the order of the drop-down.
var teamRoles = []string{"lead", "dev", "reviewer"}

// The default role: who creates the team is its lead (as the member of a solo
// space is: teamstate.SoloMemberRole); who joins a team starts as a developer,
// the least a colleague can be, and the role is one drop-down away. The
// default mode of every new card is semi-auto, whoever writes it
// (teamstate.JoinMember).
const (
	creatorRole = "lead"
	joinerRole  = "dev"
)

// identityFields are the values the identity form edits (pointers into the
// state of the assistant).
type identityFields struct {
	ID, DisplayName, GitLab, Role *string
	// Mattermost and Tracker are asked by the configuration of the team only
	// (oh team init, « Initialiser l'équipe »); nil: not asked.
	Mattermost, Tracker *string
	// LockID: the ID is the one of the card the person already has, it is not
	// asked again.
	LockID bool
	// DefaultRole is the role shown while the person has not chosen one.
	DefaultRole string
}

// addTo adds the fields to form.
func (f identityFields) addTo(form *tview.Form) {
	if !f.LockID {
		form.AddInputField(i18n.T("v532m.identity.id"), *f.ID, 0, nil, func(t string) { *f.ID = t })
	}
	form.AddInputField(i18n.T("v532m.identity.display_name"), *f.DisplayName, 0, nil, func(t string) { *f.DisplayName = t })
	form.AddInputField(i18n.T("v532m.identity.gitlab"), *f.GitLab, 0, nil, func(t string) { *f.GitLab = t })
	if f.Mattermost != nil {
		form.AddInputField(i18n.T("cmd.team.init.identity_mattermost"), *f.Mattermost, 0, nil, func(t string) { *f.Mattermost = t })
	}
	if f.Tracker != nil {
		form.AddInputField(i18n.T("tui.team.identity_tracker_username"), *f.Tracker, 0, nil, func(t string) { *f.Tracker = t })
	}
	labels := []string{
		i18n.T("cmd.init.wizard_team_role_lead"),
		i18n.T("cmd.init.wizard_team_role_dev"),
		i18n.T("cmd.init.wizard_team_role_reviewer"),
	}
	if *f.Role == "" {
		*f.Role = firstNonEmpty(f.DefaultRole, joinerRole)
	}
	idx := 0
	for i, r := range teamRoles {
		if r == *f.Role {
			idx = i
		}
	}
	form.AddDropDown(i18n.T("v532m.identity.role"), labels, idx, func(_ string, i int) {
		if i >= 0 && i < len(teamRoles) {
			*f.Role = teamRoles[i]
		}
	})
}

// member is the card the fields describe.
func (f identityFields) member() teamstate.Member {
	m := teamstate.Member{
		ID:             strings.TrimSpace(*f.ID),
		DisplayName:    strings.TrimSpace(*f.DisplayName),
		GitLabUsername: teamstate.NormalizeHandle(*f.GitLab),
		Role:           *f.Role,
	}
	if f.Mattermost != nil {
		m.MattermostUsername = strings.TrimSpace(*f.Mattermost)
	}
	if f.Tracker != nil {
		m.TrackerUsername = teamstate.NormalizeHandle(*f.Tracker)
	}
	return m
}

// problem is why the form cannot be submitted, "" when it can: the ID is
// required, an ID and a handle have no blank, and, when the members of the team
// are known, neither the ID nor the handle may be a colleague's (own: the
// member_id hub.toml already has for this team). hint tells what to do about a
// taken ID (the key of the sentence for the place the person is at).
func (f identityFields) problem(members []teamstate.Member, own, hintKey string) string {
	m := f.member()
	switch {
	case m.ID == "":
		return i18n.T("cmd.init.wizard_team_member_required")
	case strings.IndexFunc(m.ID, unicode.IsSpace) >= 0:
		return i18n.T("v532m.identity.id_spaces")
	case strings.IndexFunc(m.GitLabUsername, unicode.IsSpace) >= 0:
		return i18n.T("v532m.identity.handle_spaces")
	}
	if members == nil {
		return ""
	}
	if err := teamstate.CheckJoin(members, m, teamstate.JoinModeFor(own, m.ID, false)); err != nil {
		return joinRefusalText(err, hintKey)
	}
	return ""
}

// joinRefusal is a refusal of JoinMember in the words of the user; it still is
// the error it was (errors.Is/As).
type joinRefusal struct {
	msg string
	err error
}

func (e *joinRefusal) Error() string { return e.msg }
func (e *joinRefusal) Unwrap() error { return e.err }

// isJoinRefusal: err is a taken ID or a taken handle.
func isJoinRefusal(err error) bool {
	return errors.Is(err, teamstate.ErrMemberExists) || errors.Is(err, teamstate.ErrHandleTaken)
}

// joinRefusalError words a refusal of JoinMember (any other error is
// returned as it is). hintKey is the sentence that says what to do about a
// taken ID.
func joinRefusalError(err error, hintKey string) error {
	if !isJoinRefusal(err) {
		return err
	}
	return &joinRefusal{msg: joinRefusalText(err, hintKey), err: err}
}

// joinRefusalText says why a card cannot join and what to do.
func joinRefusalText(err error, hintKey string) string {
	var idErr *teamstate.MemberIDTakenError
	var handleErr *teamstate.HandleTakenError
	switch {
	case errors.As(err, &idErr):
		return i18n.Tf("v532m.join.id_taken", idErr.Member.ID, cardOf(idErr.Member)) + " " + i18n.T(hintKey)
	case errors.As(err, &handleErr):
		return i18n.Tf("v532m.join.handle_taken", handleErr.Handle, handleErr.Member.ID, cardOf(handleErr.Member)) + " " + i18n.T("v532m.join.handle_hint")
	}
	return err.Error()
}

// cardOf is a member in a few words: their display name and GitLab handle.
func cardOf(m teamstate.Member) string {
	name := firstNonEmpty(m.DisplayName, m.ID)
	if m.GitLabUsername != "" {
		return fmt.Sprintf("%s, gitlab: %s", name, m.GitLabUsername)
	}
	return name
}

// replaceMember puts m in members (by ID), at the end when it is not there.
func replaceMember(members []teamstate.Member, m teamstate.Member) []teamstate.Member {
	for i := range members {
		if members[i].ID == m.ID {
			members[i] = m
			return members
		}
	}
	return append(members, m)
}

// memberByID is the member with this ID in members.
func memberByID(members []teamstate.Member, id string) (teamstate.Member, bool) {
	for _, m := range members {
		if m.ID == id {
			return m, true
		}
	}
	return teamstate.Member{}, false
}

// identityErrorActions are the buttons of the error screen of a step that
// registers the card of the person: when the card was refused, correct the
// identity (the step of identityStep), and, for a taken ID, « C'est moi »
// (attach the card that exists, without changing it) by doing the step again
// (claimStep, its own ID); the buttons of base (retry, the access…) follow.
// claimStep "" offers no « C'est moi ».
func identityErrorActions(claim *bool, identityStep, claimStep string, base func(error) []views.ErrorAction) func(error) []views.ErrorAction {
	return func(err error) []views.ErrorAction {
		var actions []views.ErrorAction
		if base != nil {
			actions = base(err)
		}
		if !isJoinRefusal(err) {
			return actions
		}
		var extra []views.ErrorAction
		extra = append(extra, views.ErrorAction{
			Label: i18n.T("v532m.join.action_change"), Before: func() { *claim = false }, GoToStep: identityStep,
		})
		if errors.Is(err, teamstate.ErrMemberExists) && claimStep != "" {
			extra = append(extra, views.ErrorAction{
				Label: i18n.T("v532m.join.action_claim"), Before: func() { *claim = true }, GoToStep: claimStep,
			})
		}
		// retry stays first (Enter), the identity buttons come before the
		// others: the ones that make sense for a refusal
		if len(actions) > 0 && actions[0].Retry {
			return append(append([]views.ErrorAction{actions[0]}, extra...), actions[1:]...)
		}
		return append(extra, actions...)
	}
}

// teamCreatorFields are the fields of the identity form of « Créer une
// équipe » of oh init.
func teamCreatorFields(s *initWizardTeamState) identityFields {
	return identityFields{ID: &s.MemberID, DisplayName: &s.DisplayName, GitLab: &s.GitLabUser, Role: &s.Role, DefaultRole: creatorRole}
}
