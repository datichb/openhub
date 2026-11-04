package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// These tests run the member part of the team assistants on the real steps and
// a simulation screen, against local team-state repositories: who is "ben",
// who is not, what each path writes, and what it refuses.

const benCard = "Benjamin, gitlab: ben.g"

// teamWithColleagues is a team-state of two members: ben (lead, auto, GitLab
// ben.g) and testuser (dev, no handle).
func teamWithColleagues(t *testing.T) string {
	t.Helper()
	bare, clone := claimRepos(t)
	members := "[members.ben]\ndisplay_name = \"Benjamin\"\ngitlab_username = \"ben.g\"\nrole = \"lead\"\ndefault_mode = \"auto\"\n\n" +
		"[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\n"
	gittest.Commit(t, clone, "members.toml", members, "colleagues")
	gittest.Run(t, clone, "push", "-q")
	return bare
}

// cardsOf are the members in the remote.
func cardsOf(t *testing.T, bare string) map[string]teamstate.Member {
	t.Helper()
	repo := teamstate.NewRepo(bare, filepath.Join(t.TempDir(), "check"))
	require.NoError(t, repo.Clone(t.Context()))
	members, err := repo.ListMembers(t.Context())
	require.NoError(t, err)
	out := map[string]teamstate.Member{}
	for _, m := range members {
		out[m.ID] = m
	}
	return out
}

func hubMemberID(t *testing.T, repo string) string {
	t.Helper()
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	for _, tc := range cfg.Teams {
		if tc.StateRepo == repo {
			return tc.MemberID
		}
	}
	return ""
}

// « Créer une équipe » on a repository that already has a member of that ID: the
// card of the colleague is not replaced (it was, whole), the refusal names the
// colleague, and the person goes back to the identity and takes another ID.
func TestCreateRefusesATakenIDAndTheColleagueKeepsTheCard(t *testing.T) {
	f := newTeamFlow(t, "init")
	bare := teamWithColleagues(t)
	f.ts.Repo = bare
	f.ts.MemberID = "ben"
	f.ts.DisplayName = "Impostor"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.Tf("v532m.join.id_taken", "ben", benCard))
		waitForText(k, i18n.T("v532m.join.action_claim"))
		assert.Equal(t, map[string]teamstate.Member{
			"ben": {ID: "ben", DisplayName: "Benjamin", GitLabUsername: "ben.g", Role: "lead", DefaultMode: "auto"},
		}["ben"], cardsOf(t, bare)["ben"], "the card of the colleague is not touched")
		k.key(tcell.KeyTab, tcell.KeyEnter) // retry → change my identity
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlE)
		k.key(tcell.KeyBackspace2, tcell.KeyBackspace2, tcell.KeyBackspace2)
		k.typ("eve")
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	cards := cardsOf(t, bare)
	assert.Equal(t, "Benjamin", cards["ben"].DisplayName)
	assert.Equal(t, "ben.g", cards["ben"].GitLabUsername)
	assert.Equal(t, "auto", cards["ben"].DefaultMode)
	assert.Equal(t, "Impostor", cards["eve"].DisplayName, "the new ID is a new card")
	assert.Equal(t, "lead", cards["eve"].Role, "who creates the team is its lead")
	assert.Equal(t, "semi-auto", cards["eve"].DefaultMode)
	assert.Equal(t, "eve", hubMemberID(t, bare))
}

// « C'est moi »: hub.toml points at the card that exists, which is attached
// as it is - its empty fields are filled, what it holds is kept.
func TestCreateThatsMeAttachesTheCardWithoutOverwritingIt(t *testing.T) {
	f := newTeamFlow(t, "init")
	bare := teamWithColleagues(t)
	f.ts.Repo = bare
	f.ts.MemberID = "testuser"
	f.ts.DisplayName = "Somebody else"
	f.ts.GitLabUser = "@tu.g"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.join.action_claim"))
		k.key(tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter) // retry → change my identity → that's me
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, f.ts.Configured)
	card := cardsOf(t, bare)["testuser"]
	assert.Equal(t, "Test User", card.DisplayName, "the name of the card is kept")
	assert.Equal(t, "dev", card.Role, "and its role")
	assert.Equal(t, "tu.g", card.GitLabUsername, "the empty handle is filled")
	assert.Equal(t, "testuser", hubMemberID(t, bare))
	assert.Len(t, cardsOf(t, bare), 2, "no card added")
}

// A GitLab handle that is a colleague's is refused whatever the ID; the
// refusal does not offer « C'est moi ».
func TestCreateRefusesAHandleThatIsAColleaguesAndWritesNothing(t *testing.T) {
	f := newTeamFlow(t, "init")
	bare := teamWithColleagues(t)
	f.ts.Repo = bare
	f.ts.MemberID = "eve"
	f.ts.DisplayName = "Eve"
	f.ts.GitLabUser = "@Ben.G"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.Tf("v532m.join.handle_taken", "Ben.G", "ben", benCard))
		txt := make(chan string, 1)
		k.tvApp.QueueUpdate(func() { txt <- screenContents(k.sc) })
		assert.NotContains(t, <-txt, i18n.T("v532m.join.action_claim"), "a handle is not claimed")
		assert.NotContains(t, cardsOf(t, bare), "eve")
		k.key(tcell.KeyTab, tcell.KeyEnter) // retry → change my identity
		waitForText(k, i18n.T("v532m.identity.gitlab"))
		k.key(tcell.KeyTab, tcell.KeyTab) // name → handle
		k.key(tcell.KeyCtrlA)
		k.key(tcell.KeyDelete, tcell.KeyDelete, tcell.KeyDelete, tcell.KeyDelete, tcell.KeyDelete, tcell.KeyDelete)
		k.typ("eve.g")
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.Equal(t, "eve.g", cardsOf(t, bare)["eve"].GitLabUsername)
	assert.Equal(t, "ben.g", cardsOf(t, bare)["ben"].GitLabUsername)
}

// The palette's « Rejoindre une équipe » can create the card of a person who is
// not in the list: the same list, the same form as oh init.
func TestPaletteRejoinCreatesTheOwnCard(t *testing.T) {
	f := newTeamFlow(t, "")
	bare := teamWithColleagues(t)
	w := makeRejoinWizard(t.Context(), f.a)
	w.stateRepo = bare
	cfg := buildPaletteRejoin(w)

	res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.pick.create"))
		waitForText(k, "Benjamin (ben)")
		k.key(tcell.KeyEnter) // the first entry: create my card
		waitForText(k, i18n.T("v532m.identity.id"))
		k.typ("eve")
		k.key(tcell.KeyTab)
		k.typ("Eve")
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, res.Completed)
	card := cardsOf(t, bare)["eve"]
	assert.Equal(t, teamstate.Member{ID: "eve", DisplayName: "Eve", Role: "dev", DefaultMode: "semi-auto"}, card,
		"a joiner starts as a developer, in semi-auto")
	assert.Equal(t, "eve", hubMemberID(t, bare))
	assert.Equal(t, "Benjamin", cardsOf(t, bare)["ben"].DisplayName)
}

// In the form of that card, a taken ID or a handle that is a colleague's is
// refused before anything is written, and says to pick the card in the list.
func TestPaletteRejoinCardFormRefusesWhatIsTaken(t *testing.T) {
	f := newTeamFlow(t, "")
	bare := teamWithColleagues(t)
	w := makeRejoinWizard(t.Context(), f.a)
	w.stateRepo = bare
	cfg := buildPaletteRejoin(w)

	res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.pick.create"))
		k.key(tcell.KeyEnter)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.typ("ben")
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.Tf("v532m.join.id_taken", "ben", benCard))
		waitForText(k, i18n.T("v532m.join.id_taken_list"))
		assert.NotContains(t, cardsOf(t, bare), "eve")
		k.key(tcell.KeyBackspace2, tcell.KeyBackspace2, tcell.KeyBackspace2)
		k.typ("eve")
		k.key(tcell.KeyTab, tcell.KeyTab) // name → handle
		k.typ("ben.g")
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.Tf("v532m.join.handle_taken", "ben.g", "ben", benCard))
		k.key(tcell.KeyCtrlA)
		for range "ben.g" {
			k.key(tcell.KeyDelete)
		}
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.Equal(t, "", cardsOf(t, bare)["eve"].GitLabUsername)
	assert.Equal(t, "ben.g", cardsOf(t, bare)["ben"].GitLabUsername)
}

// A card that has no GitLab handle (made before it was asked) is completed by
// the person who picks it: the handle is optional, and a handle that is a
// colleague's is refused.
func TestPaletteRejoinPickedCardWithoutHandleIsCompleted(t *testing.T) {
	f := newTeamFlow(t, "")
	bare := teamWithColleagues(t)
	w := makeRejoinWizard(t.Context(), f.a)
	w.stateRepo = bare
	cfg := buildPaletteRejoin(w)

	res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.pick.create"))
		k.key(tcell.KeyDown, tcell.KeyDown, tcell.KeyEnter) // create → ben → testuser
		waitForText(k, i18n.T("v532m.handle.label"))
		k.typ("ben.g")
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.Tf("v532m.join.handle_taken", "ben.g", "ben", benCard))
		k.key(tcell.KeyTab, tcell.KeyEnter) // retry → change my identity
		waitForText(k, i18n.T("v532m.handle.label"))
		k.key(tcell.KeyCtrlA)
		for range "ben.g" {
			k.key(tcell.KeyDelete)
		}
		k.typ("tu.g")
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("cmd.init.wizard_rejoin_token_choice")) // the card has a handle now: the token
		k.key(tcell.KeyEnter, tcell.KeyDown, tcell.KeyDown, tcell.KeyEnter) // placeholder → new → skip
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})
	require.NoError(t, res.Err)

	card := cardsOf(t, bare)["testuser"]
	assert.Equal(t, "tu.g", card.GitLabUsername)
	assert.Equal(t, "Test User", card.DisplayName)
}

// The list of the members says when it may not be up to date.
func TestMemberPickerSaysWhenTheListMayBeStale(t *testing.T) {
	i18ntest.Set(t, "en")
	members := []teamstate.Member{{ID: "ana", DisplayName: "Ana"}}
	var memberID string
	var adding bool
	stale := "could not resolve host"
	var displayName, handle, role string
	join := &memberJoin{
		Members: &members, MemberID: &memberID, Adding: &adding,
		Fields:   identityFields{ID: &memberID, DisplayName: &displayName, GitLab: &handle, Role: &role},
		Stale:    func() string { return stale },
		PickerID: "rejoin_member", CardID: "rejoin_new_member", HandleID: "rejoin_handle",
	}
	intro := views.WizardStep{Label: "Intro", Form: func(_ *tview.Application, done func()) *tview.Form {
		form := tview.NewForm()
		form.AddButton("go", func() { done() })
		return form
	}}
	cfg := views.InlineWizardConfig{ID: "wizard.test.stale", Title: "t", Steps: []views.WizardStep{intro, join.PickerStep()}}

	_ = runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, "go")
		k.key(tcell.KeyEnter)
		waitForText(k, i18n.Tf("v532m.pick.stale", stale))
		waitForText(k, "Ana (ana)")
		k.key(tcell.KeyDown, tcell.KeyEnter)
	})
	assert.Equal(t, "ana", memberID)
}

// A clone that cannot be pulled gives its members and says why they may be old.
func TestListTeamMembersReportsAStaleClone(t *testing.T) {
	isolateGitAuth(t)
	bare := teamWithColleagues(t)
	path := filepath.Join(t.TempDir(), "clone")
	_, members, stale, err := listTeamMembers(t.Context(), bare, path)
	require.NoError(t, err)
	require.NoError(t, stale)
	assert.Len(t, members, 2)

	require.NoError(t, os.RemoveAll(bare))
	_, members, stale, err = listTeamMembers(t.Context(), bare, path)
	require.NoError(t, err, "the local content is used")
	assert.Len(t, members, 2)
	require.Error(t, stale)
	assert.NotEmpty(t, staleCause(stale))
}

// « Créer une équipe » on a repository nobody has pushed to: the structure and
// the member are the first commits, pushed with an upstream.
func TestCreateOnAnEmptyRepository(t *testing.T) {
	f := newTeamFlow(t, "init")
	bare := gittest.EmptyBare(t)
	f.ts.Repo = bare
	f.ts.MemberID = "alice"
	f.ts.DisplayName = "Alice"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, f.ts.Configured)
	files := gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main")
	assert.Contains(t, files, "members.toml")
	assert.Contains(t, files, "catalog/agents/.gitkeep")
	card := cardsOf(t, bare)["alice"]
	assert.Equal(t, "lead", card.Role)
	assert.Equal(t, "alice", hubMemberID(t, bare))
}

// « Rejoindre » a repository where the team was never created says so, in
// words that tell what to do.
func TestRejoinAnEmptyRepositorySaysTheTeamIsNotCreated(t *testing.T) {
	f := newTeamFlow(t, "")
	bare := gittest.EmptyBare(t)
	w := makeRejoinWizard(t.Context(), f.a)
	w.stateRepo = bare
	cfg := buildPaletteRejoin(w)

	res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.rejoin.empty_repo")[:60]) // the screen wraps the rest
		waitForText(k, i18n.T("v532w.action.quit"))
		k.key(tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter) // retry → change the URL → quit
	})

	assert.False(t, res.Completed)
	assert.Empty(t, hubMemberID(t, bare))

	_, _, _, err := listTeamMembers(t.Context(), bare, filepath.Join(t.TempDir(), "again"))
	require.Error(t, err)
	assert.Equal(t, i18n.T("v532m.rejoin.empty_repo"), err.Error())
}

// A repository with commits but no team in it is told apart from an empty one.
func TestRejoinARepositoryWithoutATeamSaysSo(t *testing.T) {
	isolateGitAuth(t)
	bare, _ := gittest.BareClone(t) // a README, no members.toml
	_, _, _, err := listTeamMembers(t.Context(), bare, filepath.Join(t.TempDir(), "clone"))
	require.Error(t, err)
	assert.Equal(t, i18n.T("v532m.rejoin.no_team_state"), err.Error())
}

// The configuration of the team (oh team init, « Initialiser l'équipe ») no
// longer ignores a taken ID: the form refuses it, and the step fails if it is
// taken meanwhile (it returned nil, writing nothing, before).
func TestTeamIdentityStepRefusesATakenID(t *testing.T) {
	bare := teamWithColleagues(t)
	repo := teamstate.NewRepo(bare, filepath.Join(t.TempDir(), "state"))
	require.NoError(t, repo.Clone(t.Context()))
	s := &teamStepState{Ctx: t.Context(), Repo: repo, MemberID: "ben", DisplayName: "Impostor", Role: "dev"}
	step := buildTeamIdentityStep(s, teamStepOpts{})

	assert.Contains(t, step.Validate(), "ben")
	err := step.OnDone()
	require.Error(t, err)
	assert.ErrorIs(t, err, teamstate.ErrMemberExists)
	assert.Equal(t, "Benjamin", cardsOf(t, bare)["ben"].DisplayName)

	s.MemberID, s.GitLabUsername = "eve", "ben.g"
	assert.Contains(t, step.Validate(), "ben.g")
	assert.ErrorIs(t, step.OnDone(), teamstate.ErrHandleTaken)

	s.GitLabUsername = "eve.g"
	assert.Empty(t, step.Validate())
	require.NoError(t, step.OnDone())
	assert.Equal(t, "eve.g", cardsOf(t, bare)["eve"].GitLabUsername)

	// the card the person already has (hub.toml) is theirs to change
	s2 := &teamStepState{Ctx: t.Context(), Repo: repo, HasMember: true, MemberID: "eve", DisplayName: "Eve B.", GitLabUsername: "eve.g", Role: "lead"}
	assert.Empty(t, buildTeamIdentityStep(s2, teamStepOpts{}).Validate())
	require.NoError(t, buildTeamIdentityStep(s2, teamStepOpts{}).OnDone())
	assert.Equal(t, "Eve B.", cardsOf(t, bare)["eve"].DisplayName)
}
