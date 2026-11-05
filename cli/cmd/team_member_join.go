package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The member steps of « Rejoindre », shared by oh init and the palette: the
// list of the members of the team (to pick one's own card, which is « C'est
// moi » without a word), the choice to create one's own card when one is not in
// the list, the form of that card (the identity form of every assistant), and
// the GitLab handle of a card that has none.

// addNewMemberID is the ID of the first element of the member list: the choice
// that creates one's own card instead of picking an existing one.
const addNewMemberID = "__add_new__"

// memberJoin is the state and the steps of the member part of a rejoin. The
// fields point into the state of the assistant that builds it.
type memberJoin struct {
	Members  *[]teamstate.Member
	MemberID *string // picked, or typed in the card form
	Adding   *bool   // the person creates their card
	Fields   identityFields

	// Repo is the cloned team-state (nil before the clone), Ctx the context of
	// the assistant, Own the member_id hub.toml has for this team.
	Repo func() teamstate.MemberJoiner
	Ctx  func() context.Context
	Own  func() string
	// Stale is why the list of members could not be brought up to date ("" =
	// it is).
	Stale func() string
	// Active tells whether the steps apply (nil: always); OnSkip is called
	// when the person skips one of them.
	Active func() bool
	OnSkip func()
	// Errors are the buttons of the error screens (the access…).
	Errors func(error) []views.ErrorAction
	// Defaults is who the person already is in another team (ID, name, GitLab
	// handle): the form of their card starts with it. Nil: nothing.
	Defaults func() (id, name, handle string)

	// Verify checks, before the card is published, that the GitLab token of the
	// person is the one of the handle on the card. An *identityMismatchError
	// stops the step (the person decides: change the identity, or publish
	// anyway); any other error is only a note, nothing blocks. Nil: no check.
	Verify func(ctx context.Context, m teamstate.Member) error

	PickerID, CardID, HandleID string
	// publishAnyway: the person chose to publish a card whose handle the token
	// does not confirm.
	publishAnyway bool
	// HandleOnly is the handle typed for a card that has none.
	HandleOnly string
	// created: the card was made by the person just now, with the handle they
	// wanted to give: it is not asked again.
	created bool
}

func (j *memberJoin) active() bool { return j.Active == nil || j.Active() }

func (j *memberJoin) ctx() context.Context {
	if j.Ctx != nil {
		if c := j.Ctx(); c != nil {
			return c
		}
	}
	return context.Background()
}

func (j *memberJoin) own() string {
	if j.Own == nil {
		return ""
	}
	return j.Own()
}

// Label is the label of the member step: how many members the team has.
func (j *memberJoin) Label() string {
	return i18n.Tf("v532m.pick.label", len(*j.Members))
}

// Steps are the member steps, in order.
func (j *memberJoin) Steps() []views.WizardStep {
	return []views.WizardStep{j.PickerStep(), j.CardStep(), j.HandleStep()}
}

// PickerStep picks the card of the person, or chooses to create it.
func (j *memberJoin) PickerStep() views.WizardStep {
	return views.WizardStep{
		ID:       j.PickerID,
		Label:    j.Label(),
		SkipIf:   func() bool { return !j.active() },
		OnSkip:   j.OnSkip,
		Required: true,
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			j.showPicker(tvApp, container, onDone)
		},
		InfoFields: func() []views.InfoField {
			label := i18n.T("cmd.init.wizard_rejoin_member_label")
			if *j.Adding {
				return []views.InfoField{{Label: label, Value: i18n.T("v532m.pick.creating")}}
			}
			if *j.MemberID == "" {
				return nil
			}
			if m, ok := memberByID(*j.Members, *j.MemberID); ok {
				return []views.InfoField{{Label: label, Value: fmt.Sprintf("%s (%s)", m.DisplayName, m.ID)}}
			}
			return []views.InfoField{{Label: label, Value: *j.MemberID}}
		},
	}
}

// showPicker puts the filterable list of the members in the step: the choice
// to create one's own card first, then each member with the handle and the
// role; / asks the words of the filter, Enter picks. A list that could not be
// brought up to date says so above.
func (j *memberJoin) showPicker(tvApp *tview.Application, container *tview.Flex, onDone func()) {
	items := []views.ActionItem{{ID: addNewMemberID, Cells: []string{i18n.T("v532m.pick.create")}}}
	for _, m := range *j.Members {
		secondary := ""
		if m.GitLabUsername != "" {
			secondary = i18n.Tf("cmd.init.wizard_rejoin_member_gitlab", m.GitLabUsername)
		}
		if m.Role != "" {
			if secondary != "" {
				secondary += " — "
			}
			secondary += m.Role
		}
		items = append(items, views.ActionItem{ID: m.ID, Cells: []string{fmt.Sprintf("%s (%s)", m.DisplayName, m.ID), secondary}})
	}

	box := tview.NewFlex().SetDirection(tview.FlexRow)
	var list *views.ActionList
	list = views.NewActionList(views.ActionListConfig{
		Layout:    views.DetailNone,
		OpenLabel: i18n.T("wizard.hint.submit"),
		OnOpen: func(it views.ActionItem) {
			*j.Adding = it.ID == addNewMemberID
			j.created = false
			*j.MemberID = ""
			if !*j.Adding {
				*j.MemberID = it.ID
			} else if j.Defaults != nil {
				id, name, handle := j.Defaults()
				*j.MemberID = id
				if *j.Fields.DisplayName == "" {
					*j.Fields.DisplayName = name
				}
				if *j.Fields.GitLab == "" {
					*j.Fields.GitLab = handle
				}
			}
			onDone()
		},
		Filter: &views.ActionListFilter{},
		AskQuery: func(title, current string, done func(string)) {
			askInlineQuery(tvApp, box, list.Focusable(), title, current, done)
		},
	})
	list.SetItems(items)
	if j.Stale != nil {
		if cause := j.Stale(); cause != "" {
			warn := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
			warn.SetBackgroundColor(theme.BgPanel)
			warn.SetText(fmt.Sprintf("  %s%s %s[-]", theme.ColorTagOf(theme.Warning), theme.IconWarning,
				tview.Escape(i18n.Tf("v532m.pick.stale", cause))))
			box.AddItem(warn, 3, 0, false)
		}
	}
	box.AddItem(list.Primitive(), 0, 1, true)
	container.AddItem(box, 0, 1, true)
	tvApp.SetFocus(list.Focusable())
}

// CardStep is the form of one's own card, and its registration. It is shown
// when the person chose to create it.
func (j *memberJoin) CardStep() views.WizardStep {
	return views.WizardStep{
		ID:           j.CardID,
		Label:        i18n.T("v532m.card.label"),
		Processing:   i18n.T("v532m.card.processing"),
		SkipIf:       func() bool { return !j.active() || !*j.Adding },
		OnSkip:       j.OnSkip,
		ErrorActions: j.cardErrorActions,
		Validate: func() string {
			return j.Fields.problem(*j.Members, j.own(), "v532m.join.id_taken_list")
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			j.Fields.addTo(form)
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			repo := j.Repo()
			if repo == nil {
				return errors.New(i18n.T("cmd.team_helpers.team_state_repository_not_available"))
			}
			m := j.Fields.member()
			if err := j.verifyBeforePublishing(m); err != nil {
				return err
			}
			res, err := repo.JoinMember(j.ctx(), m, teamstate.JoinModeFor(j.own(), m.ID, false))
			if err != nil {
				return joinRefusalError(err, "v532m.join.id_taken_list")
			}
			j.publishAnyway = false
			*j.Members = replaceMember(*j.Members, res.Member)
			*j.MemberID = res.Member.ID
			*j.Adding = false
			j.created = true
			return nil
		},
		InfoFields: func() []views.InfoField {
			if strings.TrimSpace(*j.Fields.ID) == "" {
				return nil
			}
			name := firstNonEmpty(strings.TrimSpace(*j.Fields.DisplayName), strings.TrimSpace(*j.Fields.ID))
			return []views.InfoField{{
				Label: i18n.T("cmd.init.wizard_rejoin_member_label"),
				Value: fmt.Sprintf("%s (%s)", name, strings.TrimSpace(*j.Fields.ID)),
			}}
		},
	}
}

// verifyBeforePublishing verifies the identity of the card before it leaves
// the machine: a handle that the token of the person does not confirm is told
// before the colleagues see the card. A token that cannot tell does not stop
// anything.
func (j *memberJoin) verifyBeforePublishing(m teamstate.Member) error {
	if j.Verify == nil || m.GitLabUsername == "" || j.publishAnyway {
		return nil
	}
	err := j.Verify(j.ctx(), m)
	var mismatch *identityMismatchError
	switch {
	case err == nil:
	case errors.As(err, &mismatch):
		return err
	default:
		slog.Warn("identity not verified before publishing the card", "member", m.ID, "error", err)
	}
	return nil
}

// cardErrorActions: a refused card goes back to the form, or to the list to
// pick the card that exists (« C'est moi »); a handle the token does not
// confirm can be published all the same.
func (j *memberJoin) cardErrorActions(err error) []views.ErrorAction {
	var actions []views.ErrorAction
	if j.Errors != nil {
		actions = j.Errors(err)
	}
	var mismatch *identityMismatchError
	if errors.As(err, &mismatch) {
		return append([]views.ErrorAction{
			{Label: i18n.T("v54a.join.publish_anyway"), Before: func() { j.publishAnyway = true }, Retry: true},
			{Label: i18n.T("v532m.join.action_change"), GoToStep: j.CardID},
		}, actions...)
	}
	if !isJoinRefusal(err) {
		return actions
	}
	extra := []views.ErrorAction{{Label: i18n.T("v532m.join.action_change"), GoToStep: j.CardID}}
	if errors.Is(err, teamstate.ErrMemberExists) {
		extra = append(extra, views.ErrorAction{Label: i18n.T("v532m.join.action_pick"), GoToStep: j.PickerID})
	}
	if len(actions) > 0 && actions[0].Retry {
		return append(append([]views.ErrorAction{actions[0]}, extra...), actions[1:]...)
	}
	return append(extra, actions...)
}

// HandleStep asks the GitLab handle of the card the person picked when it has
// none (a card made before the handle was asked): it completes the card, and
// lets oh check that the token is the person's. Left empty, nothing changes.
func (j *memberJoin) HandleStep() views.WizardStep {
	return views.WizardStep{
		ID:    j.HandleID,
		Label: i18n.T("v532m.handle.label"),
		SkipIf: func() bool {
			if !j.active() || *j.Adding || j.created || *j.MemberID == "" {
				return true
			}
			m, ok := memberByID(*j.Members, *j.MemberID)
			return !ok || m.GitLabUsername != ""
		},
		OnSkip:       j.OnSkip,
		Processing:   i18n.T("v532m.card.processing"),
		ErrorActions: j.handleErrorActions,
		Validate: func() string {
			if strings.ContainsAny(strings.TrimSpace(j.HandleOnly), " \t") {
				return i18n.T("v532m.identity.handle_spaces")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddTextView("", i18n.T("v532m.handle.hint"), 60, 3, true, false)
			form.AddInputField(i18n.T("v532m.identity.gitlab"), j.HandleOnly, 0, nil, func(t string) { j.HandleOnly = t })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			handle := teamstate.NormalizeHandle(j.HandleOnly)
			if handle == "" {
				return nil
			}
			repo := j.Repo()
			if repo == nil {
				return errors.New(i18n.T("cmd.team_helpers.team_state_repository_not_available"))
			}
			// the card is the person's own: only its empty fields are filled
			res, err := repo.JoinMember(j.ctx(), teamstate.Member{ID: *j.MemberID, GitLabUsername: handle}, teamstate.JoinClaim)
			if err != nil {
				return joinRefusalError(err, "v532m.join.id_taken_list")
			}
			*j.Members = replaceMember(*j.Members, res.Member)
			return nil
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: i18n.T("v532m.identity.gitlab"), Value: firstNonEmpty(teamstate.NormalizeHandle(j.HandleOnly), "—")}}
		},
	}
}

func (j *memberJoin) handleErrorActions(err error) []views.ErrorAction {
	var actions []views.ErrorAction
	if j.Errors != nil {
		actions = j.Errors(err)
	}
	if !errors.Is(err, teamstate.ErrHandleTaken) {
		return actions
	}
	extra := views.ErrorAction{Label: i18n.T("v532m.join.action_change"), GoToStep: j.HandleID}
	if len(actions) > 0 && actions[0].Retry {
		return append([]views.ErrorAction{actions[0], extra}, actions[1:]...)
	}
	return append([]views.ErrorAction{extra}, actions...)
}
