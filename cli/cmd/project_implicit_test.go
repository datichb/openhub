package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// implicitProjectApp has one project, the current directory outside of it.
func implicitProjectApp(t *testing.T, projects ...domain.Project) (*app.App, *bytes.Buffer) {
	t.Helper()
	useLocale(t, "en")
	t.Chdir(t.TempDir())
	if len(projects) == 0 {
		projects = []domain.Project{{ID: "web", Name: "web-app", Path: t.TempDir()}}
	}
	errOut := &bytes.Buffer{}
	return &app.App{
		Config:   &config.Config{},
		Projects: &mockProjectStore{projects: projects},
		IO:       &app.IOStreams{Out: io.Discard, ErrOut: errOut},
	}, errOut
}

func withTerminal(t *testing.T, on bool) {
	t.Helper()
	prev := stdinIsTerminal
	stdinIsTerminal = func() bool { return on }
	t.Cleanup(func() { stdinIsTerminal = prev })
}

func withConfirm(t *testing.T, answer bool) *int {
	t.Helper()
	asked := 0
	prev := cliConfirm
	cliConfirm = func(io.Writer, string) (bool, error) { asked++; return answer, nil }
	t.Cleanup(func() { cliConfirm = prev })
	return &asked
}

// N12: outside of every project, a command that writes is refused without
// a terminal (never "the only project" silently).
func TestImplicitProjectWriteWithoutTerminal(t *testing.T) {
	a, _ := implicitProjectApp(t)
	withTerminal(t, false)
	asked := withConfirm(t, true)
	_, err := resolveProject(context.Background(), a, "", projectWrite)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-p <project>")
	assert.Zero(t, *asked)
}

// N12: with a terminal, the project is confirmed; declined → nothing.
func TestImplicitProjectWriteConfirmed(t *testing.T) {
	a, _ := implicitProjectApp(t)
	withTerminal(t, true)
	asked := withConfirm(t, true)
	p, err := resolveProject(context.Background(), a, "", projectWrite)
	require.NoError(t, err)
	assert.Equal(t, "web", p.ID)
	assert.Equal(t, 1, *asked)

	withConfirm(t, false)
	_, err = resolveProject(context.Background(), a, "", projectWrite)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// N12: a command that reads announces the project it took on stderr.
func TestImplicitProjectReadWarns(t *testing.T) {
	a, errOut := implicitProjectApp(t)
	withTerminal(t, false)
	p, err := resolveProject(context.Background(), a, "", projectRead)
	require.NoError(t, err)
	assert.Equal(t, "web", p.ID)
	assert.Contains(t, errOut.String(), "Using project web-app")
}

// Inside the project directory, or with -p, nothing is asked.
func TestProjectFromDirectoryOrFlag(t *testing.T) {
	dir := t.TempDir()
	a, errOut := implicitProjectApp(t, domain.Project{ID: "web", Name: "web-app", Path: dir}, domain.Project{ID: "api", Name: "api", Path: t.TempDir()})
	withTerminal(t, false)
	asked := withConfirm(t, true)
	t.Chdir(dir)
	p, err := resolveProject(context.Background(), a, "", projectWrite)
	require.NoError(t, err)
	assert.Equal(t, "web", p.ID)
	p, err = resolveProject(context.Background(), a, "api", projectWrite)
	require.NoError(t, err)
	assert.Equal(t, "api", p.ID)
	assert.Zero(t, *asked)
	assert.Empty(t, errOut.String())

	_, err = resolveProject(context.Background(), a, "nope", projectRead)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `project "nope" not found (list: oh project list)`)
}

// N12: several projects without a terminal → refused (no form opened).
func TestImplicitProjectSeveralWithoutTerminal(t *testing.T) {
	a, _ := implicitProjectApp(t, domain.Project{ID: "web", Name: "web", Path: t.TempDir()}, domain.Project{ID: "api", Name: "api", Path: t.TempDir()})
	withTerminal(t, false)
	_, err := resolveProject(context.Background(), a, "", projectRead)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-p <project>")
}

// N12: oh run checks the tool before asking for a project.
func TestRunChecksTheToolBeforeTheProject(t *testing.T) {
	a, _ := implicitProjectApp(t)
	withTerminal(t, true)
	asked := withConfirm(t, true)
	withoutV2(t, nil)
	application = a
	t.Cleanup(func() { application = nil })
	c := &cobra.Command{}
	addRunFlags(c)
	c.SetContext(context.Background())
	err := runWorkflowCmd(c, []string{"quick"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "faketool")
	assert.Zero(t, *asked, "no project question for a launch that cannot start")
}

// N12: no credentials for the provider → a translated, actionable error
// (checked before the daemon is asked for a grant).
func TestLaunchWithoutCredentials(t *testing.T) {
	useLocale(t, "en")
	t.Setenv("OH_HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")
	a := &app.App{Config: &config.Config{}, Secrets: &mockSecretStore{secrets: map[string]string{}}}
	a.Config.LLM.DefaultProvider = "anthropic"
	project := &domain.Project{ID: "web", Name: "web-app", Path: t.TempDir()}

	err := launchCredentials(context.Background(), a, project, "")
	var pe *ProviderCredentialError
	require.ErrorAs(t, err, &pe)
	assert.Contains(t, err.Error(), "oh provider setup")
	assert.Contains(t, err.Error(), "anthropic")

	a.Secrets = &mockSecretStore{secrets: map[string]string{"openhub.provider.anthropic.token": "sk-x"}}
	assert.NoError(t, launchCredentials(context.Background(), a, project, ""))
}

// N12: the launch form shows the CLI message (daemon refusal translated).
func TestTUILaunchErrorIsTranslated(t *testing.T) {
	useLocale(t, "en")
	err := tuiLaunchError(errors.Join(errors.New("issuing proxy grant"), &daemon.APIError{Method: "POST", Path: "/grants", Status: 422, Message: "no secret"}))
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "the oh daemon could not prepare the provider credentials"), err.Error())
	assert.Contains(t, err.Error(), "oh provider setup")
	plain := errors.New("plain")
	assert.Equal(t, plain, tuiLaunchError(plain))
	assert.NoError(t, tuiLaunchError(nil))
	assert.ErrorIs(t, err, views.ErrProviderCredentials, "daemon refusal: « Configure the provider »")
	assert.NotErrorIs(t, plain, views.ErrProviderCredentials)

	missing := tuiLaunchError(fmt.Errorf("prepare: %w", &ProviderCredentialError{Detail: "anthropic"}))
	assert.ErrorIs(t, missing, views.ErrProviderCredentials)
	assert.Contains(t, missing.Error(), "oh provider setup")
}
