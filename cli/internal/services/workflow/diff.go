package workflow

import (
	"context"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// UnifiedDiff returns the unified diff of a and b ("" when equal).
func UnifiedDiff(a, b, from, to string) string {
	if a == b {
		return ""
	}
	s, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A: difflib.SplitLines(a), B: difflib.SplitLines(b), FromFile: from, ToFile: to, Context: 3})
	return s
}

// TextString returns the document (or, with prompt, the prompt template) of
// t ("" for nil).
func TextString(t *Text, prompt bool) string {
	if t == nil {
		return ""
	}
	if prompt {
		return string(t.Prompt)
	}
	return string(t.YAML)
}

// Governance returns the publication policy of the team-state of c
// (teamstate.GovernancePublishAnyMember…; P2-T17).
func (s *Service) Governance(ctx context.Context, c Context) (string, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return "", err
	}
	cfg, err := ts.Repo.LoadConfig()
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return teamstate.DefaultGovernance().PublishPolicy(), nil
	}
	return cfg.Governance.PublishPolicy(), nil
}
