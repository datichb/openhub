package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/gitlabapi"
)

func TestFormatFeedbackDiscussions_Basic(t *testing.T) {
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

	out := formatFeedbackDiscussions(discussions)
	// Branch, MR and steps come from the workflow template (QB2).
	assert.NotContains(t, out, "[MODE:feedback]")

	// Inline position rendered.
	assert.Contains(t, out, "auth.go:42")

	// Both note bodies present.
	assert.Contains(t, out, "Missing nil check on token")
	assert.Contains(t, out, "Please add integration tests")

	// Discussion headers.
	assert.Contains(t, out, "Discussion 1")
	assert.Contains(t, out, "Discussion 2")
}

func TestFormatFeedbackDiscussions_WithReplies(t *testing.T) {
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

	out := formatFeedbackDiscussions(discussions)

	// Replies are prefixed with ↳.
	assert.Contains(t, out, "↳ @bob: Agreed, will fix")
	assert.Contains(t, out, "↳ @alice: Thanks!")
}

func TestFormatFeedbackDiscussions_EmptyDiscussions(t *testing.T) {
	out := formatFeedbackDiscussions(nil)

	// Count line should show (0).
	assert.Contains(t, out, "(0)")

	// No discussion blocks.
	assert.NotContains(t, out, "--- Discussion")
}

// QB2: the `mr` input of review-feedback may be a URL, !iid or iid.
func TestMRIID(t *testing.T) {
	for ref, want := range map[string]int{
		"https://gitlab.com/org/repo/-/merge_requests/42": 42, "https://gitlab.com/org/repo/-/merge_requests/42/": 42, "!7": 7, "15": 15,
	} {
		got, ok := mrIID(ref)
		assert.True(t, ok, ref)
		assert.Equal(t, want, got, ref)
	}
	for _, ref := range []string{"feat/SRU-142", "SRU-142", ""} {
		_, ok := mrIID(ref)
		assert.False(t, ok, ref)
	}
}

func TestFindBranchForTicket_NoProject(t *testing.T) {
	result := findBranchForTicket("", "SRU-142")
	assert.Equal(t, "", result)
}
