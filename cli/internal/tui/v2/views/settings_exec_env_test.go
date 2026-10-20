package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// typingShell answers the input modal with a typed value and records the
// toasts.
type typingShell struct {
	mockShell
	typed  string
	toasts []string
}

func (s *typingShell) ShowInputModal(_, _ string, onDone func(string)) { onDone(s.typed) }
func (s *typingShell) ShowToastMsg(msg string, _ bool)                 { s.toasts = append(s.toasts, msg) }

func execSettings(t *testing.T) (*SettingsView, *config.Config) {
	t.Helper()
	cfg := &config.Config{}
	v := NewSettingsView(SettingsViewConfig{})
	v.live = cfg
	v.buildFields()
	return v, cfg
}

// S-28: execution.env_allow follows the rule of `oh config set`: names and
// PREFIX_* patterns, separated by commas or spaces, no duplicate, and "*"
// alone is refused.
func TestParseEnvAllow(t *testing.T) {
	useEnglish(t)
	got, err := ParseEnvAllow("SSH_AUTH_SOCK, MYCO_*  JAVA_OPTS,SSH_AUTH_SOCK")
	require.NoError(t, err)
	assert.Equal(t, []string{"SSH_AUTH_SOCK", "MYCO_*", "JAVA_OPTS"}, got, "no duplicate, order kept")

	got, err = ParseEnvAllow("")
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = ParseEnvAllow("MYCO_*, *")
	require.ErrorContains(t, err, `"*" alone`)
	for _, bad := range []string{"A*", "BAD NAME=1", "X=Y", "-V", "GH-TOKEN", "1ABC", "A**"} {
		_, err := ParseEnvAllow(bad)
		assert.Error(t, err, bad)
	}
	_, err = ParseEnvAllow("OK, X=Y")
	require.ErrorContains(t, err, "X=Y", "the entry that is wrong is named")
}

func TestSettingsEnvAllowField(t *testing.T) {
	useEnglish(t)
	v, cfg := execSettings(t)
	f := settingsField(t, v, "exec_env_allow")
	assert.Equal(t, CfgFieldString, f.Kind)
	assert.Empty(t, f.Get())
	assert.Empty(t, f.Source(), "nothing to warn about")

	require.NoError(t, f.Validator.Validate("GEMINI_API_KEY, MYCO_*"))
	f.Set("GEMINI_API_KEY, MYCO_*")
	assert.Equal(t, []string{"GEMINI_API_KEY", "MYCO_*"}, cfg.Execution.EnvAllow)
	assert.Equal(t, "GEMINI_API_KEY, MYCO_*", f.Get())

	// What the CLI refuses, the field refuses (and keeps the list).
	for _, bad := range []string{"*", "A*", "X=Y"} {
		require.Error(t, f.Validator.Validate(bad), bad)
		f.Set(bad)
		assert.Equal(t, []string{"GEMINI_API_KEY", "MYCO_*"}, cfg.Execution.EnvAllow, "an invalid list changes nothing: %s", bad)
	}

	f.Set("")
	assert.Empty(t, cfg.Execution.EnvAllow)
	assert.Empty(t, f.Get())
}

// A name that looks like a secret is accepted, with a warning: the agents can
// read it. The credentials oh manages are noted: they are removed anyway.
func TestSettingsEnvAllowWarnsAboutSecrets(t *testing.T) {
	useEnglish(t)
	v, cfg := execSettings(t)
	f := settingsField(t, v, "exec_env_allow")

	f.Set("GEMINI_API_KEY, GH_TOKEN, MYCO_*")
	note := f.Source()
	assert.Contains(t, note, "⚠")
	assert.Contains(t, note, "GEMINI_API_KEY, GH_TOKEN")
	assert.NotContains(t, note, "MYCO_*", "a prefix that does not look like a secret is not named")
	assert.Equal(t, []string{"GEMINI_API_KEY", "GH_TOKEN", "MYCO_*"}, cfg.Execution.EnvAllow, "kept: a warning, not a refusal")

	f.Set("ANTHROPIC_API_KEY, SSH_AUTH_SOCK")
	note = f.Source()
	assert.Contains(t, note, "ANTHROPIC_API_KEY")
	assert.Contains(t, note, "removed anyway")
	assert.NotContains(t, note, "⚠", "a managed credential is no leak: oh removes it")

	f.Set("SSH_AUTH_SOCK, JAVA_OPTS")
	assert.Empty(t, f.Source())

	// The warning is also given when the value is typed.
	sh := &typingShell{typed: "DEPLOY_TOKEN"}
	editConfigField(sh, f, func() {})
	require.Len(t, sh.toasts, 1)
	assert.Contains(t, sh.toasts[0], "DEPLOY_TOKEN looks like a secret")
	assert.Equal(t, []string{"DEPLOY_TOKEN"}, cfg.Execution.EnvAllow, "and the value is kept")

	sh = &typingShell{typed: "*"}
	editConfigField(sh, f, func() { t.Fatal("a refused value is not applied") })
	require.Len(t, sh.toasts, 1)
	assert.Contains(t, sh.toasts[0], `"*" alone`)
	assert.Equal(t, []string{"DEPLOY_TOKEN"}, cfg.Execution.EnvAllow)

	sh = &typingShell{typed: "MYCO_*"}
	editConfigField(sh, f, func() {})
	assert.Empty(t, sh.toasts, "nothing to warn about")
}

func TestSettingsProxyTimeoutField(t *testing.T) {
	useEnglish(t)
	v, cfg := execSettings(t)
	f := settingsField(t, v, "exec_proxy_first_byte")
	assert.Equal(t, CfgFieldInt, f.Kind)
	assert.Equal(t, "5", f.Placeholder, "the default shown")
	assert.Empty(t, f.Get(), "not configured")
	for _, bad := range []string{"0", "241", "-1", "x"} {
		assert.Error(t, f.Validator.Validate(bad), bad)
	}
	for _, ok := range []string{"1", "5", "240", ""} {
		assert.NoError(t, f.Validator.Validate(ok), ok)
	}
	f.Set("10")
	assert.Equal(t, 10, cfg.Execution.ProxyFirstByteTimeoutMinutes)
	assert.Equal(t, "10", f.Get())
	assert.Equal(t, 10*60, int(cfg.Execution.ProxyFirstByteTimeout().Seconds()))
	f.Set("")
	assert.Zero(t, cfg.Execution.ProxyFirstByteTimeoutMinutes)
	assert.Equal(t, 5*60, int(cfg.Execution.ProxyFirstByteTimeout().Seconds()))
}

// The two settings come after the others of the section, with their own
// description in both languages.
func TestSettingsExecSectionOrderAndTexts(t *testing.T) {
	v, _ := execSettings(t)
	var keys []string
	for _, f := range v.fields {
		if len(f.Key) > 5 && f.Key[:5] == "exec_" {
			keys = append(keys, f.Key)
		}
	}
	assert.Equal(t, []string{"exec_runtime", "exec_engine", "exec_keep_images", "exec_tool_version", "exec_strict_isolation", "exec_env_allow", "exec_proxy_first_byte"}, keys)
	for _, lang := range []string{"en", "fr"} {
		prev := i18n.Locale()
		i18n.SetLocale(lang)
		v, _ := execSettings(t)
		for _, k := range []string{"exec_env_allow", "exec_proxy_first_byte"} {
			f := settingsField(t, v, k)
			assert.NotEmpty(t, f.Label, lang+" "+k)
			assert.NotEqual(t, "tui.settings.exec.env_allow.label", f.Label)
			assert.NotEmpty(t, f.Description, lang+" "+k)
		}
		i18n.SetLocale(prev)
	}
}

func useEnglish(t *testing.T) {
	t.Helper()
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

// Render snapshot of the « Exécution » section, in French, with the two
// settings set (a name that looks like a secret warns).
func TestSettingsExecSectionRender(t *testing.T) {
	frLocale(t)
	v, cfg := execSettings(t)
	cfg.Execution.EnvAllow = []string{"SSH_AUTH_SOCK", "GEMINI_API_KEY", "MYCO_*"}
	cfg.Execution.ProxyFirstByteTimeoutMinutes = 10

	list := widgets.NewSectionedList()
	var items []widgets.SectionItem
	in := false
	for _, f := range v.fields {
		if f.Kind == CfgFieldSectionHeader {
			in = f.Label == i18n.T("tui.settings.exec.section")
		}
		if in {
			items = append(items, renderConfigItem(f, 28))
		}
	}
	require.NotEmpty(t, items)
	list.SetItems(items)
	assertGolden(t, "settings_exec_section", screenText(t, list, 120, 16))
}
