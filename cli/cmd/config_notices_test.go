package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// C3 / A-08: a hub.toml that does not load names the file, the line, the
// column and the key, in the language of the hub; former keys, unknown keys
// and former keychain names are reported by oh doctor and announced once.

// formerKeyFor is the former key of hub.toml that the current key replaces.
func formerKeyFor(t *testing.T, current string) string {
	t.Helper()
	for former, cur := range config.LegacyKeys() {
		if cur == current {
			return former
		}
	}
	t.Fatalf("no former key for %s", current)
	return ""
}

func writeHubToml(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(config.HubDir(), 0o755))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte(content), 0o600))
}

func TestConfigLoadErrorIsLocalized(t *testing.T) {
	configtest.Home(t)
	writeHubToml(t, "[cli]\nlanguage = \"fr\"\nsetup_done = \"true\"\n")
	_, err := config.Load()
	require.Error(t, err)
	wrapped := fmt.Errorf("loading config: %w", err)

	defer i18n.SetLocale("en")
	i18n.SetLocale("fr")
	msg := localizeError(&startupError{err: wrapped})
	assert.Contains(t, msg, config.ConfigPath())
	assert.Contains(t, msg, "ligne 3, colonne 14")
	assert.Contains(t, msg, "clé cli.setup_done")
	assert.Contains(t, msg, "attendu : un booléen, trouvé : une chaîne")
	assert.Contains(t, msg, "sans guillemets")

	i18n.SetLocale("en")
	msg = localizeError(wrapped)
	assert.Contains(t, msg, "line 3, column 14")
	assert.Contains(t, msg, "key cli.setup_done")
	assert.Contains(t, msg, "expected a boolean, found a string")
	assert.Contains(t, msg, "without quotes")
}

func TestConfigLoadErrorKinds(t *testing.T) {
	defer i18n.SetLocale("en")
	i18n.SetLocale("en")
	for name, tc := range map[string]struct {
		toml string
		want []string
	}{
		"a syntax error":  {"[cli\nx = 1\n", []string{"line 1, column 5"}},
		"a list element":  {"[deploy]\ninstruction_files = [1]\n", []string{"line 2", "key deploy.instruction_files", "expected in the list: a string, found an integer"}},
		"a string list":   {"[execution]\nenv_allow = \"A,B\"\n", []string{"expected a list, found a string", `write a list: ["a", "b"]`}},
		"a decimal count": {"[execution]\nkeep_images = 2.5\n", []string{"expected an integer, found a number"}},
	} {
		t.Run(name, func(t *testing.T) {
			configtest.Home(t)
			writeHubToml(t, tc.toml)
			_, err := config.Load()
			msg, ok := configLoadError(err)
			require.True(t, ok)
			for _, w := range tc.want {
				assert.Contains(t, msg, w)
			}
			assert.Contains(t, msg, config.ConfigPath())
		})
	}
	_, ok := configLoadError(errors.New("other"))
	assert.False(t, ok)
}

// oh doctor shows the same detail for a hub.toml that does not load.
func TestDoctorConfigErrorDetail(t *testing.T) {
	configtest.Home(t)
	writeHubToml(t, "[fanout]\nbudget_usd = \"12.5\"\n")
	_, err := config.Load()
	require.Error(t, err)

	defer i18n.SetLocale("en")
	i18n.SetLocale("fr")
	detail := configErrorDetail(err)
	assert.Contains(t, detail, "ligne 2, colonne 14")
	assert.Contains(t, detail, "clé fanout.budget_usd")
	assert.Contains(t, detail, "attendu : un nombre, trouvé : une chaîne")
}

func TestNoticeTextIsTranslated(t *testing.T) {
	defer i18n.SetLocale("en")
	legacy := config.Notice{Kind: config.NoticeLegacyKey, Key: formerKeyFor(t, "execution.tool_version"), Replacement: "execution.tool_version", File: "/h/hub.toml", Line: 4, Column: 1}
	keychain := config.Notice{Kind: config.NoticeLegacyKeychain, Key: "gitlab-token", Replacement: "openhub.mcp.gitlab.token"}
	unknown := config.Notice{Kind: config.NoticeUnknownKey, Key: "cli.future", File: "/h/hub.toml", Line: 3, Column: 1}

	i18n.SetLocale("fr")
	assert.Contains(t, noticeText(legacy), "/h/hub.toml:4:1")
	assert.Contains(t, noticeText(legacy), "execution.tool_version")
	assert.Contains(t, noticeText(legacy), "v6", "the end of the support is announced")
	assert.Contains(t, noticeText(config.Notice{Kind: config.NoticeLegacyKey, Key: formerKeyFor(t, "llm.default_provider"), Replacement: "llm.default_provider"}), "llm.default_provider")
	assert.Contains(t, noticeText(keychain), "openhub.mcp.gitlab.token")
	assert.Contains(t, noticeText(keychain), "v6")
	assert.Contains(t, noticeText(unknown), "cli.future")
	assert.Contains(t, noticeText(unknown), "ignorée")

	i18n.SetLocale("en")
	assert.Contains(t, noticeText(legacy), "former key")
	assert.Contains(t, noticeText(unknown), "ignored")
	assert.NotContains(t, noticeText(unknown), "cmd.config.", "no key left untranslated")
}

func TestDoctorLegacyKeysChecks(t *testing.T) {
	defer i18n.SetLocale("en")
	i18n.SetLocale("en")

	t.Run("a clean hub", func(t *testing.T) {
		configtest.Home(t)
		writeHubToml(t, "[cli]\nlanguage = \"fr\"\n")
		checks := legacyKeysChecks(context.Background())
		require.Len(t, checks, 3)
		for _, c := range checks {
			assert.True(t, c.OK)
			assert.False(t, c.Warn, c.Name)
		}
	})

	t.Run("former and unknown keys", func(t *testing.T) {
		configtest.Home(t)
		former := formerKeyFor(t, "execution.tool_version")
		table, key, _ := strings.Cut(former, ".")
		writeHubToml(t, "["+table+"]\n"+key+" = \"2.0.19\"\nfuture = true\n")
		checks := legacyKeysChecks(context.Background())
		require.Len(t, checks, 3)
		assert.True(t, checks[0].Warn)
		assert.Contains(t, checks[0].Detail, former+" → execution.tool_version")
		assert.Contains(t, checks[0].Detail, "v6")
		assert.True(t, checks[1].Warn)
		assert.Contains(t, checks[1].Detail, "execution.future")
	})

	t.Run("a hub.toml that does not load is left to its own check", func(t *testing.T) {
		configtest.Home(t)
		writeHubToml(t, "[cli]\nsetup_done = \"true\"\n")
		checks := legacyKeysChecks(context.Background())
		require.Len(t, checks, 1, "only the keychain check")
		assert.False(t, checks[0].Warn)
	})
}

func TestDoctorLegacyKeychainCheck(t *testing.T) {
	defer i18n.SetLocale("en")
	i18n.SetLocale("en")
	ctx := context.Background()

	c := legacyKeychainCheck(ctx, nil)
	assert.True(t, c.OK)
	assert.False(t, c.Warn)

	store := &mockSecretStore{secrets: map[string]string{
		"gitlab-token":             "a", // former name only: reported
		"jira-token":               "b", // both names: just a leftover
		"openhub.mcp.jira.token":   "b",
		"openhub.mcp.figma.token":  "c", // current name only
		"linear-token":             "",  // empty: nothing
		"unrelated-token-whatever": "d",
	}}
	c = legacyKeychainCheck(ctx, store)
	assert.True(t, c.OK)
	assert.True(t, c.Warn)
	assert.Contains(t, c.Detail, "gitlab-token → openhub.mcp.gitlab.token")
	assert.NotContains(t, c.Detail, "jira")
	assert.NotContains(t, c.Detail, "figma")
	assert.NotContains(t, c.Detail, "linear")
	assert.Equal(t, "a", store.secrets["gitlab-token"], "the keychain is only read")
	assert.Empty(t, store.secrets["openhub.mcp.gitlab.token"])

	delete(store.secrets, "gitlab-token")
	c = legacyKeychainCheck(ctx, store)
	assert.False(t, c.Warn)
}

// The former `oh config` key is announced when it is typed.
func TestConfigKeyAnnouncesTheFormerKey(t *testing.T) {
	configtest.Home(t)
	var got []config.Notice
	config.SetNoticeHandler(func(n config.Notice) { got = append(got, n) })
	config.DeliverNotices()

	former := formerKeyFor(t, "llm.default_provider")
	assert.Equal(t, "llm.default_provider", configKey(former))
	assert.Equal(t, "llm.default_provider", configKey(former))
	require.Len(t, got, 1, "once")
	assert.Equal(t, "llm.default_provider", got[0].Replacement)

	assert.Equal(t, "cli.language", configKey("cli.language"))
	assert.Len(t, got, 1)
}

// oh doctor --fix only restores a copy that loads: valid TOML with a value of
// the wrong type is not one.
func TestConfigBackupsSkipACopyThatDoesNotLoad(t *testing.T) {
	configtest.Home(t)
	require.NoError(t, os.MkdirAll(config.HubDir(), 0o755))
	good := config.ConfigPath() + ".bak-20260101T000000.000000000Z"
	typed := config.ConfigPath() + ".bak-20260102T000000.000000000Z"
	require.NoError(t, os.WriteFile(good, []byte("[cli]\nlanguage = \"fr\"\n"), 0o600))
	require.NoError(t, os.WriteFile(typed, []byte("[cli]\nsetup_done = \"true\"\n"), 0o600))
	now := time.Now()
	require.NoError(t, os.Chtimes(good, now.Add(-time.Hour), now.Add(-time.Hour)))
	require.NoError(t, os.Chtimes(typed, now, now))

	assert.Equal(t, []string{good}, configBackups())
	assert.Equal(t, good, latestConfigBackup())
}
