package mcpresolve

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiagnoseToken(t *testing.T) {
	env := func(set map[string]string) func(string) string {
		return func(k string) string { return set[k] }
	}
	tests := []struct {
		name    string
		service string
		enabled bool
		key     string
		env     map[string]string
		want    *Diagnostic
	}{
		{name: "enabled, no key, no variable", service: "linear", enabled: true,
			want: &Diagnostic{Service: "linear", Reason: ReasonNoToken, TokenEnv: "LINEAR_API_KEY"}},
		{name: "the name is normalized", service: " GitHub ", enabled: true,
			want: &Diagnostic{Service: "github", Reason: ReasonNoToken, TokenEnv: "GITHUB_TOKEN"}},
		{name: "a key is a source", service: "linear", enabled: true, key: "openhub.mcp.linear.token"},
		{name: "the variable is a source", service: "gitlab", enabled: true, env: map[string]string{"GITLAB_TOKEN": "x"}},
		{name: "another variable is not", service: "gitlab", enabled: true, env: map[string]string{"JIRA_TOKEN": "x"},
			want: &Diagnostic{Service: "gitlab", Reason: ReasonNoToken, TokenEnv: "GITLAB_TOKEN"}},
		{name: "disabled: nothing to say", service: "linear", enabled: false},
		{name: "team needs no token", service: "team", enabled: true},
		{name: "unknown service", service: "slack", enabled: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DiagnoseToken(tc.service, tc.enabled, tc.key, env(tc.env)))
		})
	}
	assert.NotNil(t, DiagnoseToken("jira", true, "", nil), "no environment reader: the variable is empty")
}
