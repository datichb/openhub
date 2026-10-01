package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/gitlabapi"
)

func TestBuildFeedbackPrompt_Basic(t *testing.T) {
	mr := &gitlabapi.MRInfo{
		IID:          42,
		WebURL:       "https://gitlab.com/org/repo/-/merge_requests/42",
		Title:        "feat: add auth",
		TargetBranch: "main",
	}

	discussions := []gitlabapi.Discussion{
		{
			ID: "d1",
			Notes: []gitlabapi.Note{{
				ID:   1,
				Body: "Missing nil check on token",
				Author: gitlabapi.Author{
					Username: "reviewer1",
					Name:     "Reviewer One",
				},
				Position: &gitlabapi.NotePos{
					NewPath: "auth.go",
					NewLine: 42,
				},
			}},
		},
		{
			ID: "d2",
			Notes: []gitlabapi.Note{{
				ID:   2,
				Body: "Please add integration tests",
				Author: gitlabapi.Author{
					Username: "reviewer2",
					Name:     "Reviewer Two",
				},
				// No position — general comment.
			}},
		},
	}

	out := buildFeedbackPrompt(mr, "feat/SRU-142", discussions)

	// Mode/branch/base tags.
	assert.Contains(t, out, "[MODE:feedback]")
	assert.Contains(t, out, "[BRANCH:feat/SRU-142]")
	assert.Contains(t, out, "[BASE:main]")

	// Inline position rendered.
	assert.Contains(t, out, "auth.go:42")

	// Both note bodies present.
	assert.Contains(t, out, "Missing nil check on token")
	assert.Contains(t, out, "Please add integration tests")

	// Discussion headers.
	assert.Contains(t, out, "Discussion 1")
	assert.Contains(t, out, "Discussion 2")
}

func TestBuildFeedbackPrompt_WithReplies(t *testing.T) {
	mr := &gitlabapi.MRInfo{
		TargetBranch: "main",
		WebURL:       "https://gitlab.com/mr/1",
		Title:        "fix: stuff",
	}

	discussions := []gitlabapi.Discussion{{
		ID: "d1",
		Notes: []gitlabapi.Note{
			{
				ID:     1,
				Body:   "Finding: unused variable",
				Author: gitlabapi.Author{Username: "alice"},
			},
			{
				ID:     2,
				Body:   "Agreed, will fix",
				Author: gitlabapi.Author{Username: "bob"},
			},
			{
				ID:     3,
				Body:   "Thanks!",
				Author: gitlabapi.Author{Username: "alice"},
			},
		},
	}}

	out := buildFeedbackPrompt(mr, "fix/branch", discussions)

	// Replies are prefixed with ↳.
	assert.Contains(t, out, "↳ @bob: Agreed, will fix")
	assert.Contains(t, out, "↳ @alice: Thanks!")
}

func TestBuildFeedbackPrompt_EmptyDiscussions(t *testing.T) {
	mr := &gitlabapi.MRInfo{
		TargetBranch: "develop",
		WebURL:       "https://gitlab.com/mr/99",
		Title:        "chore: cleanup",
	}

	out := buildFeedbackPrompt(mr, "chore/cleanup", nil)

	// Count line should show (0).
	assert.Contains(t, out, "(0)")

	// No discussion blocks.
	assert.NotContains(t, out, "--- Discussion")
}

func TestFindBranchForTicket_NoProject(t *testing.T) {
	result := findBranchForTicket("", "SRU-142")
	assert.Equal(t, "", result)
}
