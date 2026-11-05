package views

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

// The Teams view used to show raw keys and %!(EXTRA …): none of its
// tui.teams.* keys existed. The golden of each language holds every text of
// the view: the list, the commands, then the empty list.
func TestTeamsViewTranslated(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		t.Run(lang, func(t *testing.T) {
			prev := i18n.Locale()
			i18n.SetLocale(lang)
			t.Cleanup(func() { i18n.SetLocale(prev) })

			cfg := &config.Config{Teams: []config.TeamConfig{
				{ID: "core", Name: "Core", Enabled: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
				{ID: "solo", Solo: true, MemberID: "bob"},
			}}
			v := NewTeamsView(TeamsViewDeps{Config: cfg})
			var b strings.Builder
			items := func(title string) {
				fmt.Fprintf(&b, "== %s\n", title)
				for _, it := range v.list.GetItems() {
					fmt.Fprintf(&b, "%s | %s\n", stripTviewTags(it.MainText), it.SecondaryText)
				}
			}
			v.rebuild()
			items("list")
			b.WriteString("== commands\n")
			for _, c := range v.ContextCommands() {
				fmt.Fprintf(&b, "%s | %s | %s\n", c.Label, c.Description, c.Category)
			}
			v.cfg.Teams = nil
			v.rebuild()
			require.NotEmpty(t, v.list.GetItems(), "an empty team list still shows an item")
			items("empty")

			text := b.String()
			assert.NotContains(t, text, "tui.teams.", "untranslated text")
			assert.NotContains(t, text, "%!", "misformatted text")
			golden.Assert(t, "teams_view_"+lang, text)
		})
	}
}

// confirmingShell keeps the title of the last confirmation and its action.
type confirmingShell struct {
	recordingShell
	title string
	yes   func()
}

func (s *confirmingShell) Confirm(title, _ string, _ bool, onYes func()) {
	s.title, s.yes = title, onYes
}

// The list of the teams names the member (read in the team-state, so after the
// mount), and the confirmation and the toast of a removal name the team, not its
// ID. A team-state that does not tell leaves the member ID.
func TestTeamsViewNamesTheMemberAndTheTeam(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Name: "Core Platform", Enabled: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
		{ID: "side", Enabled: true, StateRepo: "git@example.com:side/state.git", MemberID: "bob"},
	}}
	v := NewTeamsView(TeamsViewDeps{
		Config: cfg,
		MemberNames: func(_ context.Context, teams []config.TeamConfig) map[string]string {
			assert.Len(t, teams, 2)
			return map[string]string{"core": "Alice Martin"}
		},
	})
	sh := &confirmingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	rows := func() (rows []string) {
		onLoop(app, func() {
			for _, it := range v.list.GetItems() {
				rows = append(rows, it.MainText+" | "+it.SecondaryText)
			}
		})
		return
	}
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool { return strings.Contains(strings.Join(rows(), "\n"), "@Alice Martin") }, 3*time.Second, 20*time.Millisecond)
	all := strings.Join(rows(), "\n")
	assert.NotContains(t, all, "@alice")
	assert.Contains(t, all, "@bob", "a member the team-state does not tell is their ID")

	onLoop(app, func() {
		v.list.SelectIndex(1)
		v.handleDelete()
	})
	require.NotNil(t, sh.yes)
	assert.Contains(t, sh.title, "Core Platform")
	assert.NotContains(t, sh.title, "core")
	onLoop(app, sh.yes)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	require.NotEmpty(t, sh.toasts)
	assert.Equal(t, "Team Core Platform removed", sh.toasts[len(sh.toasts)-1])
}

// n and the « add a team » command of the palette both open the assistant of
// the host; without one the view keeps asking by itself.
func TestTeamsViewAddOpensTheAssistantOfTheHost(t *testing.T) {
	added := 0
	v := NewTeamsView(TeamsViewDeps{Config: &config.Config{}, OnAdd: func() { added++ }})

	assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone)), "the key is taken")
	assert.Equal(t, 1, added)
	for _, c := range v.ContextCommands() {
		if c.ID == "teams.add" {
			c.Action()
		}
	}
	assert.Equal(t, 2, added, "the palette command does the same")

	bare := NewTeamsView(TeamsViewDeps{Config: &config.Config{}})
	assert.NotPanics(t, func() { bare.handleAdd() }, "no host assistant and no shell: nothing to open")
}

// teamsHost is the host of the Teams view in a test: a hub.toml (the file) that
// the shared config follows after each write, as updateHubConfig does.
type teamsHost struct {
	file   config.Config
	shared *config.Config
	opened []string
}

func newTeamsHost() *teamsHost {
	h := &teamsHost{file: config.Config{Teams: []config.TeamConfig{
		{ID: "core", Name: "Core", Enabled: true, Default: true, StateRepo: "git@example.com:core/state.git", MemberID: "alice"},
		{ID: "side", Enabled: true, StateRepo: "git@example.com:side/state.git", MemberID: "bob"},
		{ID: "old", Enabled: false, StateRepo: "git@example.com:old/state.git", MemberID: "carol"},
		{ID: "mine", Solo: true, Enabled: true, MemberID: "dave"},
	}}}
	h.shared = &config.Config{}
	h.sync()
	return h
}

func (h *teamsHost) sync() {
	h.shared.Teams = copyTeams(h.file.Teams)
}

func (h *teamsHost) update(fn func(*config.Config) error) error {
	if err := fn(&h.file); err != nil {
		return err
	}
	h.sync()
	return nil
}

func (h *teamsHost) deps() TeamsViewDeps {
	return TeamsViewDeps{
		Config:   h.shared,
		OnUpdate: h.update,
		OnOpen:   func(id string) { h.opened = append(h.opened, id) },
		OnSetDefault: func(id string) error {
			return h.update(func(c *config.Config) error { return c.SetDefaultTeam(id) })
		},
		OnArchive: func(id string, archive bool) (string, error) {
			note := ""
			err := h.update(func(c *config.Config) error {
				for i := range c.Teams {
					if c.Teams[i].ID == id {
						c.Teams[i].Enabled = !archive
					}
				}
				if archive {
					c.ClearDefaultTeam(id)
				}
				return nil
			})
			if archive && h.shared.DefaultTeam() != nil {
				note = "default → " + h.shared.DefaultTeam().ID
			}
			return note, err
		},
		OnRemove: func(id string) ([]string, string, error) {
			err := h.update(func(c *config.Config) error {
				c.Teams = slices.DeleteFunc(c.Teams, func(t config.TeamConfig) bool { return t.ID == id })
				return nil
			})
			return []string{"api", "web"}, "", err
		},
		ProjectsOf: func(id string) []string {
			if id == "core" {
				return []string{"api", "web"}
			}
			return nil
		},
	}
}

// teamsShell records the confirmation, the input window and the toasts.
type teamsShell struct {
	recordingShell
	title, detail string
	yes           func()
	current       string
	answer        func(string)
}

func (s *teamsShell) Confirm(title, detail string, _ bool, onYes func()) {
	s.title, s.detail, s.yes = title, detail, onYes
}
func (s *teamsShell) ShowInputModal(_, current string, onConfirm func(string)) {
	s.current, s.answer = current, onConfirm
}
func (s *teamsShell) lastToast() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.toasts) == 0 {
		return ""
	}
	return s.toasts[len(s.toasts)-1]
}

func teamKey(v *TeamsView, r rune) {
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
}

func (v *TeamsView) selectTeam(id string) {
	for i, it := range v.list.GetItems() {
		if ref, _ := it.Reference.(string); ref == id {
			v.list.SelectIndex(i)
			return
		}
	}
}

func newTestTeamsView(t *testing.T) (*TeamsView, *teamsHost, *teamsShell) {
	t.Helper()
	i18ntest.Set(t, "en")
	h := newTeamsHost()
	v := NewTeamsView(h.deps())
	sh := &teamsShell{}
	v.SetShell(sh)
	v.rebuild()
	return v, h, sh
}

func teamRowText(v *TeamsView, id string) string {
	for _, it := range v.list.GetItems() {
		if ref, _ := it.Reference.(string); ref == id {
			return stripTviewTags(it.MainText)
		}
	}
	return ""
}

// The default team is marked ★ in the list; * makes the team under the cursor
// the default one (hub.toml says so), u takes it back, and a solo space or an
// archived team is refused with the reason.
func TestTeamsViewDefaultTeam(t *testing.T) {
	v, h, sh := newTestTeamsView(t)
	assert.Equal(t, "core  Core  ★", teamRowText(v, "core"))
	assert.Equal(t, "side  side", teamRowText(v, "side"))

	v.selectTeam("side")
	teamKey(v, '*')
	assert.Equal(t, "side  side  ★", teamRowText(v, "side"))
	assert.Equal(t, "core  Core", teamRowText(v, "core"), "one default team only")
	assert.True(t, h.file.Teams[1].Default, "hub.toml marks it")
	assert.Equal(t, "side is now the default team", sh.lastToast())

	teamKey(v, '*')
	assert.Equal(t, "side is already the default team", sh.lastToast())

	teamKey(v, 'u')
	assert.Equal(t, "core  Core  ★", teamRowText(v, "core"), "u gives the default back")
	assert.True(t, h.file.Teams[0].Default)

	for _, id := range []string{"mine", "old"} {
		v.selectTeam(id)
		teamKey(v, '*')
		assert.Contains(t, sh.lastToast(), "default", id)
		assert.False(t, h.file.Teams[slices.IndexFunc(h.file.Teams, func(x config.TeamConfig) bool { return x.ID == id })].Default, id)
	}
}

// Without an explicit default the first enabled team is the default one, and
// the detail says so.
func TestTeamsViewImplicitDefaultIsSaid(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "a", Enabled: true, StateRepo: "git@x:a.git", MemberID: "me"}}}
	v := NewTeamsView(TeamsViewDeps{Config: cfg})
	v.rebuild()
	assert.Contains(t, stripTviewTags(v.defaultText(cfg.Teams[0])), "the first enabled team")
	cfg.Teams[0].Default = true
	assert.Equal(t, "★ default team", stripTviewTags(v.defaultText(cfg.Teams[0])))
}

// p opens the Team view of the team; a solo space or an archived team has none.
func TestTeamsViewOpensTheTeamView(t *testing.T) {
	v, h, sh := newTestTeamsView(t)
	v.selectTeam("side")
	teamKey(v, 'p')
	assert.Equal(t, []string{"side"}, h.opened)
	for _, id := range []string{"mine", "old"} {
		v.selectTeam(id)
		teamKey(v, 'p')
		assert.Contains(t, sh.lastToast(), "has no Team view", id)
	}
	assert.Equal(t, []string{"side"}, h.opened)
}

// x archives (an archived team is not the default any more), R restores; both
// say what they did and u takes them back.
func TestTeamsViewArchivesAndRestores(t *testing.T) {
	v, h, sh := newTestTeamsView(t)
	v.selectTeam("core")
	teamKey(v, 'x')
	assert.False(t, h.file.Teams[0].Enabled)
	assert.False(t, h.file.Teams[0].Default, "an archived team is never the default one")
	assert.Equal(t, "Team Core archived — R restores it default → side", sh.lastToast())

	teamKey(v, 'x')
	assert.Equal(t, "Core is already archived", sh.lastToast())

	teamKey(v, 'R')
	assert.True(t, h.file.Teams[0].Enabled)
	assert.Equal(t, "Team Core restored", sh.lastToast())
	teamKey(v, 'R')
	assert.Equal(t, "Core is not archived", sh.lastToast())

	teamKey(v, 'u')
	assert.False(t, h.file.Teams[0].Enabled, "u takes the restoration back")
}

// e renames the team: the display name changes in hub.toml, an empty name goes
// back to the ID.
func TestTeamsViewRenames(t *testing.T) {
	v, h, sh := newTestTeamsView(t)
	v.selectTeam("side")
	teamKey(v, 'e')
	require.NotNil(t, sh.answer)
	assert.Equal(t, "", sh.current)
	sh.answer("  Side Kicks ")
	assert.Equal(t, "Side Kicks", h.file.Teams[1].Name)
	assert.Equal(t, "side  Side Kicks", teamRowText(v, "side"))
	assert.Equal(t, "Team renamed: Side Kicks", sh.lastToast())

	teamKey(v, 'e')
	assert.Equal(t, "Side Kicks", sh.current)
	sh.answer("")
	assert.Equal(t, "side  side", teamRowText(v, "side"))
}

// d announces the projects it detaches, removes the team, and says what was
// detached.
func TestTeamsViewRemovalDetachesTheProjects(t *testing.T) {
	v, h, sh := newTestTeamsView(t)
	v.selectTeam("core")
	teamKey(v, 'd')
	require.NotNil(t, sh.yes)
	assert.Contains(t, sh.detail, "2 attached project(s) (api, web) are detached")
	sh.yes()
	assert.Equal(t, "Team Core removed — 2 project(s) detached: api, web", sh.lastToast())
	assert.Empty(t, teamRowText(v, "core"))
	assert.Len(t, h.file.Teams, 3)

	// A team without project keeps the usual wording.
	v.selectTeam("side")
	teamKey(v, 'd')
	assert.NotContains(t, sh.detail, "attached")
}

// The list ends with the row of the new team, which is the whole list without
// any team; Enter on it and n open the same window.
func TestTeamsViewNewTeamRow(t *testing.T) {
	i18ntest.Set(t, "en")
	added := 0
	v := NewTeamsView(TeamsViewDeps{Config: &config.Config{}, OnAdd: func() { added++ }})
	sh := &teamsShell{}
	v.SetShell(sh)
	v.rebuild()
	items := v.list.GetItems()
	require.Len(t, items, 1)
	assert.Equal(t, "➕  New team", items[0].MainText)
	assert.Contains(t, items[0].SecondaryText, "create a team or join")

	v.list.SelectIndex(0)
	v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	assert.Equal(t, 1, added, "Enter on the row")
	teamKey(v, 'n')
	assert.Equal(t, 2, added)
	for _, k := range []rune{'d', 'x', 'p', '*', 'e', 's'} {
		assert.NotPanics(t, func() { teamKey(v, k) }, string(k), "no team under the cursor")
	}

	full := NewTeamsView(TeamsViewDeps{Config: &config.Config{Teams: []config.TeamConfig{{ID: "a", Enabled: true, StateRepo: "git@x:a.git", MemberID: "me"}}}, OnAdd: func() { added++ }})
	full.rebuild()
	last := full.list.GetItems()[len(full.list.GetItems())-1]
	assert.Equal(t, newTeamRef, last.Reference, "the row closes the list")
	full.list.SelectIndex(len(full.list.GetItems()) - 1)
	full.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	assert.Equal(t, 3, added)
}
