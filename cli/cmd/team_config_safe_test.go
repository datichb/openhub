package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// wizardEnv is a team whose team-state is a local repository with the given
// config.toml, a hub.toml with the given content and the CLI app on them.
type wizardEnv struct {
	a        *app.App
	repo     *teamstate.Repo
	out      *bytes.Buffer
	statePth string
}

func newWizardEnv(t *testing.T, teamToml, hubTail string) *wizardEnv {
	t.Helper()
	configtest.Home(t)
	for _, k := range []string{"GITLAB_TOKEN", "OH_GITLAB_URL", "GITLAB_URL", "JIRA_API_TOKEN", "JIRA_URL", "JIRA_TOKEN"} {
		t.Setenv(k, "")
	}
	statePath := filepath.Join(t.TempDir(), "state")
	repo, err := teamstate.InitSolo(t.Context(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	if teamToml != "" {
		require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(teamToml), 0o644))
		gittest.Run(t, statePath, "add", "config.toml")
		gittest.Run(t, statePath, "commit", "-q", "-m", "config")
	}
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@mon-alias:acme/core.git", StatePath: statePath, MemberID: "alice"},
	}}
	require.NoError(t, config.Save(cfg))
	if hubTail != "" {
		data, err := os.ReadFile(config.ConfigPath())
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(config.ConfigPath(), append(data, []byte(hubTail)...), 0o600))
		cfg, err = config.Read()
		require.NoError(t, err)
	}
	out := &bytes.Buffer{}
	io := app.DefaultIOStreams()
	io.Out, io.ErrOut = out, out
	a := &app.App{Config: cfg, IO: io, Projects: &mockProjectStore{}, Secrets: &mockSecretStore{secrets: map[string]string{}}}
	application = a
	teamConfigCmd.SetContext(t.Context())
	t.Cleanup(func() { application = nil })
	return &wizardEnv{a: a, repo: repo, out: out, statePth: statePath}
}

// run plays the assistant `oh team config` with the answers typed (one per line).
func (e *wizardEnv) run(answers ...string) error {
	e.a.IO.In = strings.NewReader(strings.Join(answers, "\n") + "\n")
	return runTeamConfigWizard(teamConfigCmd, nil)
}

func (e *wizardEnv) head(t *testing.T) string {
	t.Helper()
	return gittest.Run(t, e.statePth, "rev-parse", "HEAD")
}

func (e *wizardEnv) teamFile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.statePth, "config.toml"))
	require.NoError(t, err)
	return string(data)
}

const jiraTeam = "[tracker]\ntype = \"jira\"\nenabled = true\ntracker_url = \"https://jira.example.com\"\ntracker_project = \"ABC\"\nmax_in_progress_per_member = 3\n"

// enters is n answers that are just Enter.
func enters(n int) []string { return make([]string, n) }

// A team on Jira that goes through the tracker assistant pressing Enter stays
// on Jira, with every other setting: Enter takes the current value (it used to
// take the first choice, GitLab).
func TestTeamConfigTrackerEnterKeepsJira(t *testing.T) {
	e := newWizardEnv(t, jiraTeam, "")
	head, file := e.head(t), e.teamFile(t)

	// team scope [1], tracker sync [4]; type, URL, token key, "set up a token?",
	// token, enabled, auto-plan, limit, push, auto-sync, project: Enter;
	// the discovery: no
	answers := append([]string{"1", "4"}, enters(11)...)
	require.NoError(t, e.run(append(answers, "n")...))

	got, err := e.repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "jira", got.Tracker.Type, "Enter keeps the type of the team")
	assert.Equal(t, 3, got.Tracker.InProgressLimit())
	assert.Equal(t, file, e.teamFile(t), "nothing changed: the file is not rewritten")
	assert.Equal(t, head, e.head(t), "and nothing is committed")
}

// The type of a team that has none has to be said: Enter is not a choice.
func TestTeamConfigTrackerTypeIsRequiredWhenThereIsNone(t *testing.T) {
	e := newWizardEnv(t, "[claim]\nlease_hours = 24\n", "")
	head := e.head(t)
	err := e.run("1", "4", "", "", "") // Enter ×3 on the type
	require.Error(t, err)
	assert.Equal(t, head, e.head(t))
	assert.NotContains(t, e.teamFile(t), "type")
}

// An input that ends before the questions do stops the assistant, and nothing
// is written.
func TestTeamConfigStopsOnAnEndedInputAndWritesNothing(t *testing.T) {
	e := newWizardEnv(t, jiraTeam, "")
	head, file := e.head(t), e.teamFile(t)
	// the type is changed to GitLab, then the input ends
	err := e.run("1", "4", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing was written")
	assert.Equal(t, file, e.teamFile(t))
	assert.Equal(t, head, e.head(t))
}

// Without a terminal to answer on, the assistant refuses to ask.
func TestTeamConfigRefusesWithoutATerminal(t *testing.T) {
	e := newWizardEnv(t, jiraTeam, "")
	head, file := e.head(t), e.teamFile(t)
	prev := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = prev })
	e.a.IO.In = os.Stdin

	err := runTeamConfigWizard(teamConfigCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
	assert.Equal(t, file, e.teamFile(t))
	assert.Equal(t, head, e.head(t))
}

// A config.toml that cannot be read is not started from as if it were empty.
func TestTeamConfigRefusesAnUnreadableTeamConfig(t *testing.T) {
	e := newWizardEnv(t, "", "")
	bad := "this is {{{{ invalid\n[tracker]\ntype = \"jira\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(e.statePth, "config.toml"), []byte(bad), 0o644))
	err := e.run("1", "4", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.toml unreadable")
	assert.Equal(t, bad, e.teamFile(t))
}

// Local overrides: Enter keeps the current state, nothing is written that was
// not chosen, and « inherit » removes the key from hub.toml.
func TestTeamConfigLocalInheritRemovesTheKey(t *testing.T) {
	hub := "\n[tracker]\nenabled = false\nauto_sync = true\n"
	e := newWizardEnv(t, jiraTeam, hub)
	before, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)

	// local scope [2], tracker sync [4]; enabled: Enter; auto-sync: inherit [1];
	// the key of the token, the GitLab of the repository: Enter
	require.NoError(t, e.run("2", "4", "", "1", "", ""))

	got, err := config.Read()
	require.NoError(t, err, "hub.toml is read again")
	require.NotNil(t, got.Tracker.Enabled)
	assert.False(t, *got.Tracker.Enabled, "Enter keeps the state of this machine")
	assert.Nil(t, got.Tracker.AutoSync, "inherit unsets the key")
	data, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.NotContains(t, string(data), "auto_sync")
	assert.Nil(t, e.a.Config.Tracker.AutoSync, "the config of the app follows the file")
	assert.NotEqual(t, string(before), string(data))
}

// Pressing Enter all along writes nothing to hub.toml (it used to write
// tracker.enabled, auto_sync and push_labels whatever was answered).
func TestTeamConfigLocalEnterWritesNothing(t *testing.T) {
	e := newWizardEnv(t, jiraTeam, "")
	before, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)

	require.NoError(t, e.run("2", "4", "", "", "", ""))

	after, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
	assert.NotContains(t, string(after), "auto_sync")
}

// A choice is written with the type of its key: a keychain key made of digits
// stays text, and hub.toml stays readable.
func TestTeamConfigLocalWritesTypedValues(t *testing.T) {
	e := newWizardEnv(t, "[claim]\nlease_hours = 24\n", "")
	a := e.a
	a.IO.In = strings.NewReader("y\n123\n\ny\n") // new token; its key; its server: Enter; write: yes
	p, err := newPrompter(a)
	require.NoError(t, err)

	require.NoError(t, configureGitLab(t.Context(), a, p, e.repo, &teamstate.TeamConfig{}, false, true))

	got, err := config.Read()
	require.NoError(t, err, "hub.toml is read back")
	assert.Equal(t, "123", got.MCP.Gitlab.Token)
	assert.True(t, got.MCP.Gitlab.Enabled)
	assert.True(t, got.MCP.Gitlab.WriteEnabled)
	assert.Equal(t, "123", a.Config.MCP.Gitlab.Token, "the config of the app follows the file")
}

// A limit that is not a number is said, and the limit stays.
func TestTeamConfigSaysAnInvalidLimit(t *testing.T) {
	e := newWizardEnv(t, jiraTeam, "")
	// type, URL, key, set up a token?, token, enabled, auto-plan: Enter; limit: abc;
	// push, auto-sync, project: Enter; discovery: no
	answers := append([]string{"1", "4"}, enters(7)...)
	answers = append(answers, "abc")
	answers = append(answers, enters(3)...)
	require.NoError(t, e.run(append(answers, "n")...))
	assert.Contains(t, e.out.String(), "“abc”")
	got, err := e.repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 3, got.Tracker.InProgressLimit())
}

// The write of the team config goes through the latest version: a setting a
// colleague changed while the assistant was asking is kept, and one that both
// changed is not overwritten.
func TestTeamConfigDoesNotOverwriteAColleague(t *testing.T) {
	e := newWizardEnv(t, jiraTeam, "")
	teamCfg, err := e.repo.LoadConfig(t.Context())
	require.NoError(t, err)

	require.NoError(t, e.repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Notification.Channel = "dev"
		return nil
	}))
	// the assistant changes the project
	p := answerPrompts(t, e.a, "1", "", "", "", "", "", "", "", "", "", "XYZ", "n")
	require.NoError(t, configureTrackerSync(t.Context(), e.a, p, e.repo, teamCfg, true, false))
	got, err := e.repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "XYZ", got.Tracker.TrackerProject)
	assert.Equal(t, "dev", got.Notification.Channel, "the colleague's setting stays")

	// the colleague changes the project too; the assistant started from the old copy
	require.NoError(t, e.repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.Tracker.TrackerProject = "QRS"
		return nil
	}))
	p = answerPrompts(t, e.a, "2", "", "", "", "", "", "", "", "", "", "TUV", "n")
	err = configureTrackerSync(t.Context(), e.a, p, e.repo, teamCfg, true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing was overwritten")
	got, err = e.repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "QRS", got.Tracker.TrackerProject)
}

// oh team config status reads: it removes nobody from members.toml, whatever
// is typed or not (the colleagues are not in the local config, so they all
// looked like orphans, and Enter or an ended input deleted them).
func TestTeamConfigStatusNeverTouchesTheMembers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stdin func(t *testing.T) *os.File
	}{
		{"Enter", func(t *testing.T) *os.File { return pipeWith(t, "\n\n\n\n\n\n") }},
		{"no terminal", func(t *testing.T) *os.File { return pipeWith(t, "") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, out, _, _, _ := statusHub(t)
			statePath := a.Config.ActiveTeam().StatePath
			repo := teamstate.NewRepo("", statePath)
			for _, id := range []string{"bob", "carol", "dave"} {
				require.NoError(t, repo.AddMember(t.Context(), teamstate.Member{ID: id, DisplayName: strings.ToUpper(id)}))
			}
			membersFile := filepath.Join(statePath, "members.toml")
			before, err := os.ReadFile(membersFile)
			require.NoError(t, err)
			head := gittest.Run(t, statePath, "rev-parse", "HEAD")

			stdin := tc.stdin(t)
			prev := os.Stdin
			os.Stdin = stdin
			t.Cleanup(func() { os.Stdin = prev })
			a.IO.In = stdin

			require.NoError(t, runTeamConfigStatus(teamConfigStatusCmd, nil))

			after, err := os.ReadFile(membersFile)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after), "members.toml is unchanged")
			assert.Equal(t, head, gittest.Run(t, statePath, "rev-parse", "HEAD"), "nothing is committed")
			assert.Empty(t, gittest.Run(t, statePath, "status", "--porcelain", "--", "members.toml"))
			assert.NotContains(t, strings.ToLower(out.String()), "orphan")
			members, err := repo.ListMembers(t.Context())
			require.NoError(t, err)
			assert.Len(t, members, 4)
		})
	}
}

// pipeWith is a file that holds content, for os.Stdin.
func pipeWith(t *testing.T, content string) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	t.Cleanup(func() { _ = r.Close() })
	return r
}
