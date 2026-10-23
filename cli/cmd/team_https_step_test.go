package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// dumbGitServer is an HTTP git remote (dumb protocol) that serves one branch,
// with or without a token.
func dumbGitServer(t *testing.T, wantToken string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wantToken != "" && !strings.HasSuffix(r.Header.Get("Authorization"), wantToken) {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/info/refs"):
			_, _ = w.Write([]byte("0123456789012345678901234567890123456789\trefs/heads/main\n"))
		case strings.HasSuffix(r.URL.Path, "/HEAD"):
			_, _ = w.Write([]byte("ref: refs/heads/main\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Bug 13: « Authentification HTTPS » was shown for a repository already cloned
// and readable without a token; only SSH URLs skipped it.
func TestHTTPSCredStepSkippedWhenTheRepositoryAnswers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	i18ntest.Set(t, "en")
	configtest.Home(t)
	ctx := context.Background()

	t.Run("readable without a token", func(t *testing.T) {
		srv := dumbGitServer(t, "")
		repoURL := srv.URL + "/org/team.git"
		state := &httpsCredState{}
		step := buildHTTPSCredStep(ctx, &repoURL, state, nil, nil)
		assert.False(t, step.SkipIf(), "before the clone is probed the step is shown")

		state.Probe(ctx, repoURL)
		assert.True(t, step.SkipIf(), "the clone answers without a token: no authentication step")
	})

	t.Run("wants a token", func(t *testing.T) {
		srv := dumbGitServer(t, "s3cret")
		repoURL := srv.URL + "/org/team.git"
		state := &httpsCredState{}
		step := buildHTTPSCredStep(ctx, &repoURL, state, nil, nil)
		state.Probe(ctx, repoURL)
		assert.False(t, step.SkipIf(), "a repository that refuses git keeps the step")
	})

	t.Run("an ssh remote is still skipped", func(t *testing.T) {
		repoURL := "git@gitlab.example.com:org/team.git"
		step := buildHTTPSCredStep(ctx, &repoURL, &httpsCredState{}, nil, nil)
		assert.True(t, step.SkipIf())
	})

	t.Run("probe forgets an earlier answer for another URL", func(t *testing.T) {
		open := dumbGitServer(t, "")
		closed := dumbGitServer(t, "s3cret")
		state := &httpsCredState{}
		state.Probe(ctx, open.URL+"/a.git")
		require.True(t, state.Open)
		state.Probe(ctx, closed.URL+"/b.git")
		assert.False(t, state.Open)
	})
}

// Bug 13: with the default choice (provide a token) and an empty field, the
// button did nothing and said nothing.
func TestHTTPSCredStepSaysWhyANonEmptyTokenIsNeeded(t *testing.T) {
	i18ntest.Set(t, "en")
	repoURL := "https://gitlab.example.com/org/team.git"
	state := &httpsCredState{}
	step := buildHTTPSCredStep(context.Background(), &repoURL, state, nil, nil)

	require.NotNil(t, step.Validate, "an empty token is reported")
	msg := step.Validate()
	assert.NotEmpty(t, msg, "the default choice is « provide a token »: an empty field is refused with a message")
	assert.Contains(t, msg, "token")

	state.Token = "glpat-x"
	assert.Empty(t, step.Validate(), "a token is accepted")

	state.Token = ""
	state.AuthChoice = "skip"
	assert.Empty(t, step.Validate(), "« already configured » needs no token")
	state.AuthChoice = "public"
	assert.Empty(t, step.Validate(), "a public repository needs no token")
}

// The same two behaviours in the steps of the wizard of oh init (rejoin).
func TestInitWizardHTTPSStepsAreSkippedAndExplain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	i18ntest.Set(t, "en")
	ctx := context.Background()
	srv := dumbGitServer(t, "")

	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a
	state := &initWizardTeamState{Mode: "rejoin", Repo: srv.URL + "/org/team.git", Ctx: ctx}
	steps := buildInitWizardHTTPSCredSteps(appPtr, state, "rejoin")
	require.Len(t, steps, 2)
	assert.False(t, steps[0].SkipIf(), "not probed yet")

	state.ProbeRepo(ctx)
	assert.True(t, steps[0].SkipIf(), "the repository answers without a token: mode step skipped")
	assert.True(t, steps[1].SkipIf(), "and so is the token step")

	state.RepoOpen = false
	state.CredAuthChoice = "provide"
	require.NotNil(t, steps[1].Validate)
	assert.NotEmpty(t, steps[1].Validate(), "an empty token is refused with a message")
	state.CredToken = "glpat-x"
	assert.Empty(t, steps[1].Validate())
}
