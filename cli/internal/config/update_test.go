package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

func seedHub(t *testing.T, home, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".oh")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "hub.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// diskHub decodes hub.toml as it is on disk, without going through the cache.
func diskHub(t *testing.T) *config.Config {
	t.Helper()
	data, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	var c config.Config
	require.NoError(t, toml.Unmarshal(data, &c))
	return &c
}

// Bug 1 of v5.1: a copy read before the wizard wrote hub.toml was saved over
// it once the file had been read again, because the protection was a mtime
// kept by the process (Reset cleared it, the next Load armed it on the
// current file). The protection follows the copy.
func TestSave_StaleCopyIsRefusedEvenAfterAReload(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "[cli]\nlanguage = \"en\"\n")

	stale, err := config.Load() // what a view holds from the start of the session
	require.NoError(t, err)

	config.Reset() // a re-read (ReadConfig, ReloadApp…)
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.CLI.SetupDone = true
		c.LLM.DefaultProvider = "bedrock"
		c.Provider.Bedrock.AWSRegion = "eu-west-1"
		c.Worktree.BranchPattern = "feat/{ticket}"
		return nil
	}))
	_, err = config.Load() // the re-read the old code used to re-arm its protection
	require.NoError(t, err)

	stale.MCP.Gitlab.Enabled = true
	err = config.Save(stale)
	require.ErrorIs(t, err, config.ErrExternalModification)

	got := diskHub(t)
	assert.True(t, got.CLI.SetupDone, "setup_done must survive")
	assert.Equal(t, "bedrock", got.LLM.DefaultProvider)
	assert.Equal(t, "eu-west-1", got.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "feat/{ticket}", got.Worktree.BranchPattern)
	assert.False(t, got.MCP.Gitlab.Enabled, "the refused copy wrote nothing")
}

func TestSave_ProtectionSurvivesReset(t *testing.T) {
	home := configtest.Home(t)
	path := seedHub(t, home, "name = \"a\"\n")

	c, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("name = \"external\"\n"), 0o600))
	later := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(path, later, later))

	config.Reset()
	c.Name = "mine"
	require.ErrorIs(t, config.Save(c), config.ErrExternalModification)
	assert.Equal(t, "external", diskHub(t).Name)
}

func TestSave_ConsecutiveSavesOfTheSameCopy(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "name = \"a\"\n")

	c, err := config.Load()
	require.NoError(t, err)
	c.Name = "b"
	require.NoError(t, config.Save(c))
	c.Name = "c"
	require.NoError(t, config.Save(c), "a save stamps the copy with the file it wrote")
	assert.Equal(t, "c", diskHub(t).Name)
}

func TestSave_ConfigBuiltByHandIsNotCompared(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "name = \"a\"\n")
	require.NoError(t, config.Save(&config.Config{Name: "fresh"}))
	assert.Equal(t, "fresh", diskHub(t).Name)
}

func TestSave_FileCreatedSinceTheDefaultsWereLoaded(t *testing.T) {
	home := configtest.Home(t)
	c, err := config.Load() // no hub.toml: the defaults
	require.NoError(t, err)
	seedHub(t, home, "name = \"someone else\"\n")
	c.CLI.SetupDone = true
	require.ErrorIs(t, config.Save(c), config.ErrExternalModification)
}

func TestSave_RemovedFileIsRecreated(t *testing.T) {
	home := configtest.Home(t)
	path := seedHub(t, home, "name = \"a\"\n")
	c, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	c.Name = "b"
	require.NoError(t, config.Save(c))
	assert.Equal(t, "b", diskHub(t).Name)
}

// Update reads hub.toml under the lock: what another process wrote after the
// cache was filled is kept, and only the field of the caller changes.
func TestUpdate_KeepsWhatAnotherProcessWrote(t *testing.T) {
	home := configtest.Home(t)
	path := seedHub(t, home, "[cli]\nlanguage = \"en\"\n")

	shared, err := config.Load()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("[cli]\nlanguage = \"en\"\nsetup_done = true\n[llm]\ndefault_provider = \"bedrock\"\n"), 0o600))

	require.NoError(t, config.Update(func(c *config.Config) error {
		c.MCP.Gitlab.Enabled = true
		return nil
	}))

	got := diskHub(t)
	assert.True(t, got.MCP.Gitlab.Enabled)
	assert.True(t, got.CLI.SetupDone, "the other process's setup_done is kept")
	assert.Equal(t, "bedrock", got.LLM.DefaultProvider)

	// whoever holds the shared Config sees the result
	assert.True(t, shared.MCP.Gitlab.Enabled)
	assert.True(t, shared.CLI.SetupDone)
}

func TestUpdate_FunctionErrorWritesNothing(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "name = \"a\"\n")
	err := config.Update(func(c *config.Config) error {
		c.Name = "changed"
		return fmt.Errorf("no")
	})
	require.EqualError(t, err, "no")
	assert.Equal(t, "a", diskHub(t).Name)
}

func TestUpdate_ConcurrentWritersKeepEveryField(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "name = \"a\"\n")

	var wg sync.WaitGroup
	// every writer sets its own team
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, config.Update(func(c *config.Config) error {
				c.UpsertTeam(config.TeamConfig{ID: fmt.Sprintf("team-%d", i)})
				return nil
			}))
		}()
	}
	wg.Wait()
	assert.Len(t, diskHub(t).Teams, 6)
}

func TestUpdate_InvalidTeamIsRefused(t *testing.T) {
	home := configtest.Home(t)
	seedHub(t, home, "name = \"a\"\n")
	err := config.Update(func(c *config.Config) error {
		c.Teams = []config.TeamConfig{{ID: "Bad ID"}}
		return nil
	})
	require.Error(t, err)
	assert.Empty(t, diskHub(t).Teams)
}
