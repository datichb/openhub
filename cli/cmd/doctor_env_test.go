package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeEnvAllow(t *testing.T) {
	environ := []string{
		"PATH=/bin", "HOME=/h", "PWD=/x", "SHLVL=1",
		"GEMINI_API_KEY=g-secret", "GH_TOKEN=gh-secret", "MYCO_REGION=eu", "MYCO_API_TOKEN=t-secret", "SSH_AUTH_SOCK=/a",
		"ANTHROPIC_API_KEY=a-secret",
	}
	t.Run("nothing added", func(t *testing.T) {
		f := analyzeEnvAllow(nil, environ)
		assert.ElementsMatch(t, []string{"GEMINI_API_KEY", "GH_TOKEN", "MYCO_REGION", "MYCO_API_TOKEN", "SSH_AUTH_SOCK", "ANTHROPIC_API_KEY"}, f.Held,
			"names of the variables held back, without the terminal noise")
		assert.Empty(t, f.SecretLooks)
		assert.Empty(t, f.SecretPassed)
	})
	t.Run("secret-looking names added", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"GEMINI_API_KEY", "SSH_AUTH_SOCK", "OTHER_PASSWORD", "NPM_SECRET"}, environ)
		assert.ElementsMatch(t, []string{"GEMINI_API_KEY", "OTHER_PASSWORD", "NPM_SECRET"}, f.SecretLooks)
		assert.NotContains(t, f.Held, "GEMINI_API_KEY")
		assert.NotContains(t, f.Held, "SSH_AUTH_SOCK")
		assert.Contains(t, f.Held, "GH_TOKEN")
	})
	t.Run("a pattern lets a secret of the environment through", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"MYCO_*"}, environ)
		assert.Equal(t, []string{"MYCO_API_TOKEN"}, f.SecretPassed)
		assert.NotContains(t, f.Held, "MYCO_REGION")
		assert.Empty(t, f.SecretLooks, "the entry itself is not a secret name")
	})
	t.Run("invalid and managed entries", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"*", "ANTHROPIC_API_KEY", "FINE_ONE"}, environ)
		assert.Equal(t, []string{"*"}, f.Invalid)
		assert.Equal(t, []string{"ANTHROPIC_API_KEY"}, f.Managed)
		assert.Contains(t, f.Held, "ANTHROPIC_API_KEY", "removed anyway")
		assert.Empty(t, f.SecretLooks)
	})
	t.Run("values never appear", func(t *testing.T) {
		f := analyzeEnvAllow([]string{"MYCO_*"}, environ)
		for _, n := range append(f.Held, f.SecretPassed...) {
			assert.NotContains(t, n, "secret")
		}
	})
}

func TestEnvAllowDoctorChecks(t *testing.T) {
	useLocale(t, "en")
	t.Setenv("GEMINI_API_KEY", "g-secret")
	checks := envAllowDoctorChecks(t.Context())
	if assert.NotEmpty(t, checks) {
		assert.True(t, checks[0].OK)
		assert.Contains(t, checks[0].Detail, "base list")
	}
	var held string
	for _, c := range checks {
		assert.NotContains(t, c.Detail, "g-secret", "values are never shown")
		if c.Warn == false && c.Detail != checks[0].Detail {
			held = c.Detail
		}
	}
	assert.Contains(t, held, "GEMINI_API_KEY")
	assert.Contains(t, held, "oh config set execution.env_allow")
}
