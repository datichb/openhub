package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// oh policies add: the answers of the wizard make one policy of the type
// chosen, written to the team-state.
func TestPoliciesAddWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	// Steps: 0 info (name, type, enforcement, message), 1 regex (rule),
	// 2 forbidden patterns (patterns, scope), 3 limit (value).
	info := func(name string, typ, enforcement int, message string) func(f *tview.Form) {
		return func(f *tview.Form) {
			typeInto(f, 0, name)
			choose(f, 1, typ)
			choose(f, 2, enforcement)
			typeInto(f, 3, message)
		}
	}
	tests := []struct {
		name    string
		answers map[int]func(f *tview.Form)
		abortAt int
		wantRan []int
		want    map[string]teamstate.Policy
		wantErr string
	}{
		{
			name: "regex rule, refused",
			answers: map[int]func(f *tview.Form){
				0: info("branch", 0, 1, "bad branch"),
				1: func(f *tview.Form) { typeInto(f, 0, "^feat/") },
			},
			abortAt: -1,
			wantRan: []int{0, 1},
			want: map[string]teamstate.Policy{"branch": {
				Name: "branch", Type: teamstate.PolicyTypeRegex, Rule: "^feat/", Enforcement: teamstate.EnforcementRefuse, Message: "bad branch",
			}},
		},
		{
			name: "forbidden patterns, trimmed, with a scope",
			answers: map[int]func(f *tview.Form){
				0: info("secrets", 1, 0, ""),
				2: func(f *tview.Form) {
					typeInto(f, 0, " TODO , FIXME,, ")
					choose(f, 1, 1)
				},
			},
			abortAt: -1,
			wantRan: []int{0, 2},
			want: map[string]teamstate.Policy{"secrets": {
				Name: "secrets", Type: teamstate.PolicyTypeForbiddenPattern, Patterns: []string{"TODO", "FIXME"},
				Scope: "modified_files", Enforcement: teamstate.EnforcementWarn,
			}},
		},
		{
			name: "limit",
			answers: map[int]func(f *tview.Form){
				0: info("wip", 2, 0, "too many"),
				3: func(f *tview.Form) { typeInto(f, 0, "4") },
			},
			abortAt: -1,
			wantRan: []int{0, 3},
			want: map[string]teamstate.Policy{"wip": {
				Name: "wip", Type: teamstate.PolicyTypeLimit, Max: 4, Enforcement: teamstate.EnforcementWarn, Message: "too many",
			}},
		},
		{
			name:    "boolean",
			answers: map[int]func(f *tview.Form){0: info("review", 3, 1, "")},
			abortAt: -1,
			wantRan: []int{0},
			want: map[string]teamstate.Policy{"review": {
				Name: "review", Type: teamstate.PolicyTypeBoolean, Enabled: true, Enforcement: teamstate.EnforcementRefuse,
			}},
		},
		{
			name:    "no name: the command fails, nothing is written",
			answers: map[int]func(f *tview.Form){0: info("", 0, 0, "")},
			abortAt: -1,
			wantRan: []int{0},
			wantErr: "name",
		},
		{
			name:    "the user quits: nothing is written",
			answers: map[int]func(f *tview.Form){0: info("x", 0, 0, "")},
			abortAt: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, repo, _ := setupClaimTestApp(t)
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt

			policiesAddCmd.SetContext(t.Context())
			err := runPoliciesAdd(policiesAddCmd, nil)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, "policies add", s.command, "command of the wizard")
			assert.Equal(t, tc.wantRan, s.ran, "steps that ran")
			list, err := repo.LoadPolicies(t.Context(), "")
			require.NoError(t, err)
			got := map[string]teamstate.Policy{}
			for _, p := range list {
				got[p.Name] = p
			}
			if tc.want == nil {
				assert.Empty(t, got, "policies written")
			} else {
				assert.Equal(t, tc.want, got, "policies written")
			}
		})
	}
}
