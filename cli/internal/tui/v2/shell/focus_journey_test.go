package shell

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// journey wires the real list of teams and the real team detail the way the
// host does: the team opened from the list is the one the detail resolves.
type journey struct {
	*simShell
	mu     sync.Mutex
	picked string
	added  int // times the row « Nouvelle équipe » (or n) asked the host for a team
	solos  map[string]views.TeamResolution
}

func newJourney(t *testing.T, ids ...string) *journey {
	t.Helper()
	j := &journey{solos: map[string]views.TeamResolution{}}
	hub := &config.Config{}
	for _, id := range ids {
		dir := t.TempDir() + "/state"
		_, err := teamstate.InitSolo(context.Background(), dir, teamstate.Member{ID: "alice"})
		require.NoError(t, err)
		hub.Teams = append(hub.Teams, config.TeamConfig{ID: id, Name: "Team " + id, Enabled: true, Solo: true, MemberID: "alice", StatePath: dir})
		j.solos[id] = views.TeamResolution{Enabled: true, Solo: true, TeamID: id, StatePath: dir, MemberID: "alice"}
	}
	var sh *simShell
	teams := views.NewTeamsView(views.TeamsViewDeps{
		Config:       hub,
		OnNavigate:   func(id string) { sh.NavigateTo(id) },
		OnSelectTeam: func(id string) { j.mu.Lock(); j.picked = id; j.mu.Unlock() },
		OnAdd:        func() { j.mu.Lock(); j.added++; j.mu.Unlock() },
	})
	detail := views.NewTeamDetailView(views.TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return config.MCPConfig{} },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return config.TrackerLocalConfig{} },
		GetSecrets:            func() tracker.SecretGetter { return nil },
		GetHubConfig:          func() *config.Config { return hub },
		ResolveTeam: func() views.TeamResolution {
			j.mu.Lock()
			defer j.mu.Unlock()
			return j.solos[j.picked]
		},
	})
	sh = runSimShell(t, 120, 40, Config{ProjectName: "test", HomeViewID: "home",
		Views: []views.View{&keyView{id: "home", focusList: true}, &keyView{id: "settings", focusList: true}, teams, detail}})
	j.simShell = sh
	sh.loop(func() { sh.NavigateHome("home", views.ModeHub) })
	return j
}

func (j *journey) pickedID() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.picked
}

func (j *journey) esc(t *testing.T, wantDepth int, step string) {
	t.Helper()
	j.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, wantDepth, j.depth(), step)
	assert.Equal(t, 0, j.toastCount(), "no toast after %s", step)
}

// Home → Configuration → Teams → detail: Esc goes back one level at every
// step, with 0, 1 and 2 teams (the row of the new team closes the list), and a detail left unedited shows no toast.
func TestEscWalksBackHomeConfigurationTeamsDetail(t *testing.T) {
	for _, ids := range [][]string{{}, {"alpha"}, {"alpha", "beta"}} {
		t.Run(fmt.Sprintf("%d teams", len(ids)), func(t *testing.T) {
			j := newJourney(t, ids...)
			j.loop(func() { j.NavigateTo("settings") })
			j.loop(func() { j.NavigateTo("teams") })
			require.Equal(t, 3, j.depth())

			screen := j.screenText()
			if len(ids) == 0 {
				assert.Contains(t, strings.ToLower(screen), "no team", "the empty list says so")
				j.key(tcell.KeyEnter, 0, tcell.ModNone)
				assert.Equal(t, 3, j.depth(), "no team to open")
				j.mu.Lock()
				assert.Equal(t, 1, j.added, "the row of the new team starts one")
				j.mu.Unlock()
				j.loop(func() { j.NavigateTo("team.detail") }) // from the palette
				assert.Contains(t, j.screenText(), "No team to configure")
				assert.Contains(t, j.screenText(), "Esc", "the way back is shown")
				j.esc(t, 3, "the empty detail")
			} else {
				if len(ids) > 1 {
					j.key(tcell.KeyDown, 0, tcell.ModNone)
				}
				j.key(tcell.KeyEnter, 0, tcell.ModNone)
				require.Equal(t, 4, j.depth(), "the team opens")
				want := ids[len(ids)-1] // Down moves to the second team when there is one
				assert.Equal(t, want, j.pickedID(), "the team chosen in the list is the one opened")
				require.Eventually(t, func() bool { return strings.Contains(j.screenText(), "· "+want) }, 3e9, 20e6,
					"the detail shows the chosen team")
				assert.Contains(t, j.screenText(), "Esc", "the way back is shown")
				j.esc(t, 3, "the detail")
			}
			j.esc(t, 2, "the teams")
			j.esc(t, 1, "the configuration")
		})
	}
}

// n in the list of teams opens the assistant that joins a team (the host's
// OnAdd), not a form that only writes hub.toml; leaving the assistant, the list
// shows the team it added.
func TestTeamsViewNOpensTheJoinAssistantAndTheListShowsTheNewTeam(t *testing.T) {
	hub := &config.Config{}
	var sh *simShell
	joined := &keyView{id: "join", focusList: true}
	teams := views.NewTeamsView(views.TeamsViewDeps{
		Config: hub,
		OnAdd: func() {
			// what the assistant does when it ends: the team is in hub.toml
			hub.Teams = append(hub.Teams, config.TeamConfig{ID: "acme-core", Name: "Acme core", Enabled: true,
				StateRepo: "git@example.com:acme/core.git", StatePath: "/clones/acme-core", MemberID: "eve"})
			sh.PushView(joined)
		},
	})
	sh = runSimShell(t, 120, 40, Config{ProjectName: "test", HomeViewID: "home",
		Views: []views.View{&keyView{id: "home", focusList: true}, &keyView{id: "settings", focusList: true}, teams, joined}})
	sh.loop(func() { sh.NavigateHome("home", views.ModeHub) })
	sh.loop(func() { sh.NavigateTo("settings") })
	sh.loop(func() { sh.NavigateTo("teams") })
	require.Equal(t, 3, sh.depth())
	assert.Contains(t, strings.ToLower(sh.screenText()), "no team")

	sh.key(tcell.KeyRune, 'n', tcell.ModNone)
	require.Equal(t, 4, sh.depth(), "the assistant is open over the list of teams")
	assert.Zero(t, sh.toastCount())

	sh.key(tcell.KeyEscape, 0, tcell.ModNone)
	require.Equal(t, 3, sh.depth(), "back to the list")
	require.Eventually(t, func() bool { return strings.Contains(sh.screenText(), "acme-core") }, 3e9, 20e6,
		"the team the assistant added is in the list")
}
