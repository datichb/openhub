package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

// Strict reading of hub.toml (v5.0, C3). Until v4 the file went through viper
// and mapstructure with weak typing; now it goes through go-toml. Every case
// below is a hub.toml that the former reader accepted: the tests say what
// happens to it now. The same table is documented in
// docs/reference/config.{fr,en}.md.

func writeHub(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(config.HubDir(), 0o755))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte(content), 0o600))
}

// loadHub loads a hub.toml written to a temporary hub, with the notices it
// raises.
func loadHub(t *testing.T, content string) (*config.Config, []config.Notice, error) {
	t.Helper()
	configtest.Home(t)
	writeHub(t, content)
	var notes []config.Notice
	config.SetNoticeHandler(func(n config.Notice) { notes = append(notes, n) })
	config.DeliverNotices()
	cfg, err := config.Load()
	return cfg, notes, err
}

// refusedCases are the values the weakly typed reader converted and the
// strict reader refuses, with where the error points.
var refusedCases = []struct {
	name              string
	toml              string
	line, col         int
	key               string
	expected, found   string
	hint              string
	formerlyGaveValue string
}{
	{name: "a boolean written as a string", toml: "[cli]\nsetup_done = \"true\"\n",
		line: 2, col: 14, key: "cli.setup_done", expected: "boolean", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "true"},
	{name: "a boolean written as the string 1", toml: "[cli]\nsetup_done = \"1\"\n",
		line: 2, col: 14, key: "cli.setup_done", expected: "boolean", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "true"},
	{name: "a boolean written as a number", toml: "[cli]\nsetup_done = 1\n",
		line: 2, col: 14, key: "cli.setup_done", expected: "boolean", found: "integer",
		formerlyGaveValue: "true"},
	{name: "a boolean of the legacy [team] section written as a string", toml: "[team]\nenabled = \"true\"\nstate_repo = \"git@x:y/z.git\"\n",
		line: 2, col: 11, key: "team.enabled", expected: "boolean", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "true"},
	{name: "an optional boolean written as a string", toml: "[tracker]\nenabled = \"false\"\n",
		line: 2, col: 11, key: "tracker.enabled", expected: "boolean", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "false"},
	{name: "an integer written as a string", toml: "[execution]\nkeep_images = \"3\"\n",
		line: 2, col: 15, key: "execution.keep_images", expected: "integer", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "3"},
	{name: "an integer written as a decimal number", toml: "[execution]\nkeep_images = 2.7\n",
		line: 2, col: 15, key: "execution.keep_images", expected: "integer", found: "number",
		formerlyGaveValue: "2"},
	{name: "an integer written as a boolean", toml: "[execution]\nkeep_images = true\n",
		line: 2, col: 15, key: "execution.keep_images", expected: "integer", found: "boolean",
		formerlyGaveValue: "1"},
	{name: "an integer of a table array written as a string", toml: "[[remote.targets]]\nname = \"t\"\nrunner_project_id = \"42\"\n",
		line: 3, col: 21, key: "remote.targets.runner_project_id", expected: "integer", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "42"},
	{name: "a decimal number written as a string", toml: "[fanout]\nbudget_usd = \"12.5\"\n",
		line: 2, col: 14, key: "fanout.budget_usd", expected: "number", found: "string", hint: config.HintUnquote,
		formerlyGaveValue: "12.5"},
	{name: "a string written as a number", toml: "[[teams]]\nid = \"a\"\nmember_id = 123\n",
		line: 3, col: 13, key: "teams.member_id", expected: "string", found: "integer",
		formerlyGaveValue: "123"},
	{name: "a string written as a boolean", toml: "[cli]\nlanguage = true\n",
		line: 2, col: 12, key: "cli.language", expected: "string", found: "boolean",
		formerlyGaveValue: "1"},
	{name: "a string written as a decimal number", toml: "[cli]\nlanguage = 1.5\n",
		line: 2, col: 12, key: "cli.language", expected: "string", found: "number",
		formerlyGaveValue: "1.5"},
	{name: "a list written as a comma-separated string", toml: "[execution]\nenv_allow = \"A,B, C\"\n",
		line: 2, col: 13, key: "execution.env_allow", expected: "list", found: "string", hint: config.HintList,
		formerlyGaveValue: "[A B  C]"},
	{name: "a list written as a single string", toml: "[deploy]\ninstruction_files = \"X.md\"\n",
		line: 2, col: 21, key: "deploy.instruction_files", expected: "list", found: "string", hint: config.HintList,
		formerlyGaveValue: "[X.md]"},
	{name: "a list of strings written with numbers", toml: "[deploy]\ninstruction_files = [1, 2]\n",
		line: 2, col: 22, key: "deploy.instruction_files", expected: "string", found: "integer",
		formerlyGaveValue: "[1 2]"},
	{name: "a list of models written as a string", toml: "[limits]\nmodels = \"a,b\"\n",
		line: 2, col: 10, key: "limits.models", expected: "list", found: "string", hint: config.HintList,
		formerlyGaveValue: "[a b]"},
	{name: "a table written as a string", toml: "cli = \"fr\"\n",
		line: 1, col: 7, key: "cli", expected: "table", found: "string",
		formerlyGaveValue: "(error)"},
}

func TestStrictReading_RefusesWhatViperConverted(t *testing.T) {
	for _, tc := range refusedCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("the former reader gave %s", tc.formerlyGaveValue)
			cfg, _, err := loadHub(t, tc.toml)
			require.Error(t, err)
			assert.Nil(t, cfg)

			var ce *config.ConfigError
			require.ErrorAs(t, err, &ce)
			assert.Equal(t, config.ConfigPath(), ce.Path, "the file")
			assert.Equal(t, tc.line, ce.Line, "the line")
			assert.Equal(t, tc.col, ce.Column, "the column")
			assert.Equal(t, tc.key, ce.Key, "the key")
			assert.Equal(t, tc.expected, ce.Expected)
			assert.Equal(t, tc.found, ce.Found)
			assert.Equal(t, tc.hint, ce.Hint())
			msg := err.Error()
			assert.Contains(t, msg, config.ConfigPath())
			assert.Contains(t, msg, fmt.Sprintf("%s:%d:%d", config.ConfigPath(), tc.line, tc.col))
			assert.Contains(t, msg, tc.key)
		})
	}
}

// The first error is reported, wherever it is, and a later one is not hidden
// by a key that is only unknown.
func TestStrictReading_ReportsTheErrorAfterUnknownKeys(t *testing.T) {
	_, _, err := loadHub(t, "[future]\nx = 1\n\n[cli]\nsetup_done = \"true\"\n")
	var ce *config.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, 5, ce.Line)
	assert.Equal(t, "cli.setup_done", ce.Key)
}

func TestStrictReading_ErrorOfAMultiLineList(t *testing.T) {
	_, _, err := loadHub(t, "[deploy]\ninstruction_files = [\n  \"A.md\",\n  3,\n]\n")
	var ce *config.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, 4, ce.Line, "the element at fault")
	assert.Equal(t, "deploy.instruction_files", ce.Key, "the key of the list")
}

func TestStrictReading_SyntaxErrorHasFileLineAndColumn(t *testing.T) {
	_, _, err := loadHub(t, "[cli\nlanguage = \"fr\"\n")
	var ce *config.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, config.ConfigPath(), ce.Path)
	assert.Equal(t, 1, ce.Line)
	assert.Equal(t, 5, ce.Column)
	assert.Empty(t, ce.Key, "no key for a syntax error")
	assert.Contains(t, err.Error(), config.ConfigPath()+":1:5")
}

// The TOML library panics on a date where it expects a string.
func TestStrictReading_ADateWhereAStringIsExpectedIsAnError(t *testing.T) {
	_, _, err := loadHub(t, "[cli]\nlanguage = 2020-01-01\n")
	var ce *config.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, config.ConfigPath(), ce.Path)
}

// What the former reader accepted and the strict one still accepts.
func TestStrictReading_StillAccepts(t *testing.T) {
	t.Run("keys written in capitals", func(t *testing.T) {
		cfg, notes, err := loadHub(t, "[CLI]\nLanguage = \"fr\"\nSetup_Done = true\n")
		require.NoError(t, err)
		assert.Equal(t, "fr", cfg.CLI.Language)
		assert.True(t, cfg.CLI.SetupDone)
		assert.Empty(t, notes, "the case of a key is not a reason to warn")
	})
	t.Run("an integer for a decimal number", func(t *testing.T) {
		cfg, _, err := loadHub(t, "[fanout]\nbudget_usd = 10\n")
		require.NoError(t, err)
		assert.InDelta(t, 10.0, cfg.Fanout.BudgetUSD, 0)
	})
	t.Run("dotted keys", func(t *testing.T) {
		cfg, _, err := loadHub(t, "cli.language = \"fr\"\n")
		require.NoError(t, err)
		assert.Equal(t, "fr", cfg.CLI.Language)
	})
	t.Run("the legacy [team] section", func(t *testing.T) {
		cfg, notes, err := loadHub(t, "[team]\nenabled = true\nstate_repo = \"git@x:y/z.git\"\nmember_id = \"bob\"\n")
		require.NoError(t, err)
		assert.True(t, cfg.Team.Enabled)
		assert.Equal(t, "git@x:y/z.git", cfg.Team.StateRepo)
		assert.Equal(t, "bob", cfg.ActiveTeam().MemberID)
		assert.Empty(t, notes, "the legacy section is migrated to [[teams]] at startup, not warned about")
	})
	t.Run("a legacy [team] section beside [[teams]] is cleared", func(t *testing.T) {
		cfg, _, err := loadHub(t, "[team]\nenabled = true\nstate_repo = \"git@x:old.git\"\n[[teams]]\nid = \"acme\"\n")
		require.NoError(t, err)
		assert.Empty(t, cfg.Team.StateRepo)
		require.Len(t, cfg.Teams, 1)
	})
	t.Run("the keys of the maps of models in capitals are lowercased", func(t *testing.T) {
		cfg, _, err := loadHub(t, "[models.agents]\nReviewer = \"opus\"\n[models.families]\nQuality = \"x\"\n")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"reviewer": "opus"}, cfg.Models.Agents)
		assert.Equal(t, map[string]string{"quality": "x"}, cfg.Models.Families)
	})
}

// Changed on purpose: the former reader lowercased every key of every map.
func TestStrictReading_KeysOfOtherMapsKeepTheirCase(t *testing.T) {
	cfg, _, err := loadHub(t, "[remote.projects]\nMyProj = \"T\"\n[[remote.targets]]\nname = \"t\"\n[remote.targets.binaries]\n\"v5.0.0/AMD64\" = \"sum\"\n")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"MyProj": "T"}, cfg.Remote.Projects)
	assert.Equal(t, map[string]string{"v5.0.0/AMD64": "sum"}, cfg.Remote.Targets[0].Binaries)
}

// Changed on purpose: the former reader split a quoted key at its dots, so
// "cli.language" = "fr" set cli.language. It is now a key named "cli.language"
// that no setting uses.
func TestStrictReading_AQuotedDottedKeyIsNotASetting(t *testing.T) {
	cfg, notes, err := loadHub(t, "\"cli.language\" = \"fr\"\n")
	require.NoError(t, err)
	assert.Equal(t, "en", cfg.CLI.Language)
	require.Len(t, notes, 1)
	assert.Equal(t, config.NoticeUnknownKey, notes[0].Kind)
}

// Unknown keys are warnings: a hub.toml written by a newer oh still loads.
func TestStrictReading_UnknownKeysAreWarnings(t *testing.T) {
	t.Run("an unknown key in a known table", func(t *testing.T) {
		cfg, notes, err := loadHub(t, "[cli]\nlanguage = \"fr\"\nfuture = 1\n")
		require.NoError(t, err)
		assert.Equal(t, "fr", cfg.CLI.Language, "the rest is read")
		require.Len(t, notes, 1)
		assert.Equal(t, config.Notice{Kind: config.NoticeUnknownKey, Key: "cli.future", File: config.ConfigPath(), Line: 3, Column: 1}, notes[0])
	})
	t.Run("an unknown table is one warning, not one per key", func(t *testing.T) {
		_, notes, err := loadHub(t, "[future]\nx = 1\ny = 2\n[future.sub]\nz = 3\n")
		require.NoError(t, err)
		require.Len(t, notes, 1)
		assert.Equal(t, "future", notes[0].Key)
		assert.Equal(t, 1, notes[0].Line)
	})
	t.Run("a near miss of a real key", func(t *testing.T) {
		_, notes, err := loadHub(t, "[mcp.gitlab]\ntoken = \"k\"\n")
		require.NoError(t, err)
		require.Len(t, notes, 1)
		assert.Equal(t, "mcp.gitlab.token", notes[0].Key, "the key is token_key")
	})
	t.Run("once per process", func(t *testing.T) {
		_, notes, err := loadHub(t, "[future]\nx = 1\n")
		require.NoError(t, err)
		require.Len(t, notes, 1)
		config.Reset()
		_, err = config.Load()
		require.NoError(t, err)
		assert.Len(t, notes, 1, "a second read of the same key says nothing")
	})
}

// Environment variables: the former reader never read any (no AutomaticEnv,
// no prefix), so no variable sets a key of hub.toml, and none does now.
func TestStrictReading_EnvironmentDoesNotSetKeys(t *testing.T) {
	configtest.Home(t)
	for _, name := range []string{"CLI_LANGUAGE", "OH_CLI_LANGUAGE", "HUB_CLI_LANGUAGE", "OH_LANGUAGE", "NAME"} {
		t.Setenv(name, "fr")
	}
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "en", cfg.CLI.Language)
	assert.Equal(t, "OpenHub", cfg.Name)
}

func TestStrictReading_Defaults(t *testing.T) {
	check := func(t *testing.T, cfg *config.Config) {
		t.Helper()
		assert.Equal(t, "OpenHub", cfg.Name)
		assert.Equal(t, "en", cfg.CLI.Language)
		assert.True(t, cfg.Worktree.AutoCleanup)
		assert.True(t, cfg.GC.Enabled)
		assert.Empty(t, cfg.Worktree.BaseBranch)
		assert.False(t, cfg.Websearch.Enabled)
	}
	t.Run("no hub.toml", func(t *testing.T) {
		configtest.Home(t)
		cfg, err := config.Load()
		require.NoError(t, err)
		check(t, cfg)
	})
	t.Run("an empty hub.toml", func(t *testing.T) {
		cfg, notes, err := loadHub(t, "")
		require.NoError(t, err)
		check(t, cfg)
		assert.Empty(t, notes)
	})
	t.Run("a key left out of a table keeps its default", func(t *testing.T) {
		cfg, _, err := loadHub(t, "[worktree]\nbase_branch = \"develop\"\n[gc]\nevery_hours = 6\n[cli]\nsetup_done = true\n")
		require.NoError(t, err)
		assert.Equal(t, "develop", cfg.Worktree.BaseBranch)
		assert.True(t, cfg.Worktree.AutoCleanup, "default kept")
		assert.True(t, cfg.GC.Enabled, "default kept")
		assert.Equal(t, 6, cfg.GC.EveryHours)
		assert.Equal(t, "en", cfg.CLI.Language, "default kept")
	})
	t.Run("a default can be overridden, even by an empty value", func(t *testing.T) {
		cfg, _, err := loadHub(t, "name = \"\"\n[cli]\nlanguage = \"\"\n[worktree]\nauto_cleanup = false\n[gc]\nenabled = false\n")
		require.NoError(t, err)
		assert.Empty(t, cfg.Name)
		assert.Empty(t, cfg.CLI.Language)
		assert.False(t, cfg.Worktree.AutoCleanup)
		assert.False(t, cfg.GC.Enabled)
	})
}

func TestStrictReading_UnreadableFileIsAnError(t *testing.T) {
	configtest.Home(t)
	require.NoError(t, os.MkdirAll(config.ConfigPath(), 0o755), "hub.toml is a directory")
	_, err := config.Load()
	require.Error(t, err)
	var ce *config.ConfigError
	assert.False(t, errors.As(err, &ce), "not a content error")
}

// Inspect (oh doctor) sees the same notices as Load, without announcing them.
func TestInspect(t *testing.T) {
	configtest.Home(t)
	notes, err := config.Inspect()
	require.NoError(t, err)
	assert.Empty(t, notes, "no hub.toml")

	writeHub(t, "[cli]\nfuture = 1\n")
	var announced []config.Notice
	config.SetNoticeHandler(func(n config.Notice) { announced = append(announced, n) })
	config.DeliverNotices()
	notes, err = config.Inspect()
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "cli.future", notes[0].Key)
	assert.Empty(t, announced, "Inspect announces nothing")

	writeHub(t, "[cli]\nsetup_done = \"true\"\n")
	_, err = config.Inspect()
	var ce *config.ConfigError
	require.ErrorAs(t, err, &ce)
}

func TestNoticesAreHeldUntilDelivered(t *testing.T) {
	configtest.Home(t)
	writeHub(t, "[cli]\nfuture = 1\n")
	var got []config.Notice
	config.SetNoticeHandler(func(n config.Notice) { got = append(got, n) })
	_, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, got, "held: the language of the hub is not known yet")

	config.DeliverNotices()
	require.Len(t, got, 1)
	assert.Equal(t, "cli.future", got[0].Key)

	config.Reset()
	_, err = config.Load()
	require.NoError(t, err)
	assert.Len(t, got, 1, "once per process")
}

func TestNoticesWithoutAHandlerAreKept(t *testing.T) {
	configtest.Home(t)
	writeHub(t, "[cli]\nfuture = 1\n")
	config.DeliverNotices()
	_, err := config.Load()
	require.NoError(t, err)

	var got []config.Notice
	config.SetNoticeHandler(func(n config.Notice) { got = append(got, n) })
	require.Len(t, got, 1, "delivered when the handler arrives")
}

// Every setting of hub.toml is named by its toml tag: the library matches
// field names too (without case), which would silently make a Go name a
// valid key.
func TestEveryFieldHasATomlName(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(path string, rt reflect.Type)
	walk = func(path string, rt reflect.Type) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Map {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
			assert.NotEmpty(t, name, "%s.%s has no toml name", path, f.Name)
			assert.NotContains(t, string(f.Tag), "mapstructure", "%s.%s: mapstructure is gone", path, f.Name)
			walk(path+"."+f.Name, f.Type)
		}
	}
	walk("Config", reflect.TypeOf(config.Config{}))
}

// Everything Save writes is read back as it was written.
func TestSavedFileIsReadStrictly(t *testing.T) {
	configtest.Home(t)
	c := &config.Config{Name: "x"}
	c.CLI.Language = "fr"
	c.Execution.EnvAllow = []string{"A", "B_*"}
	c.Fanout.BudgetUSD = 12.5
	c.Limits.Models = []string{"m1", "m2"}
	tracker := true
	c.Tracker.Enabled = &tracker
	n := 3
	c.Tracker.MaxAutoPlanPerMember = &n
	c.Remote.Projects = map[string]string{"P1": "t"}
	require.NoError(t, config.Save(c))

	got, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, c.Execution.EnvAllow, got.Execution.EnvAllow)
	assert.InDelta(t, 12.5, got.Fanout.BudgetUSD, 0)
	assert.Equal(t, c.Limits.Models, got.Limits.Models)
	require.NotNil(t, got.Tracker.Enabled)
	assert.True(t, *got.Tracker.Enabled)
	assert.Equal(t, 3, *got.Tracker.MaxAutoPlanPerMember)
	assert.Equal(t, map[string]string{"P1": "t"}, got.Remote.Projects)

	entries, err := os.ReadDir(config.HubDir())
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "no temporary file left: %s", e.Name())
	}
	_, err = os.Stat(filepath.Join(config.HubDir(), "hub.toml.tmp"))
	assert.True(t, os.IsNotExist(err), "the fixed temporary name is not used any more")
}
