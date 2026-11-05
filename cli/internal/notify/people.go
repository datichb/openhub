package notify

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// People names the members and the projects of a message, and finds the
// handle a member is mentioned with.
type People interface {
	teamstate.EventNames
	// Handle is the mattermost_username of a member ("" when they have none).
	Handle(memberID string) string
}

// MemberLister is what NewPeople reads of a team-state.
type MemberLister interface {
	ListMembers(ctx context.Context) ([]teamstate.Member, error)
}

type people struct {
	members  map[string]teamstate.Member
	projects func(id string) string
}

// NewPeople reads the members of the team-state; projectName names a project
// by its ID (nil or "" = the ID).
func NewPeople(ctx context.Context, src MemberLister, projectName func(id string) string) People {
	p := &people{members: map[string]teamstate.Member{}, projects: projectName}
	if src == nil {
		return p
	}
	if list, err := src.ListMembers(ctx); err == nil {
		for _, m := range list {
			p.members[m.ID] = m
		}
	}
	return p
}

func (p *people) Project(id string) string {
	if p.projects != nil {
		if n := p.projects(id); n != "" {
			return n
		}
	}
	return id
}

func (p *people) Member(id string) string {
	if m, ok := p.members[id]; ok && strings.TrimSpace(m.DisplayName) != "" {
		return m.DisplayName
	}
	return id
}

func (p *people) Handle(id string) string {
	return strings.TrimPrefix(strings.TrimSpace(p.members[id].MattermostUsername), "@")
}

// Recipients are the members an event is for, by ID: the reviewer asked to
// review, the author of the work that was reviewed, the member a claim went to
// or whose claim expires.
func Recipients(e teamstate.Event) []string {
	key := ""
	switch e.Type {
	case teamstate.EventReviewReady:
		key = "reviewer"
	case teamstate.EventReviewApproved, teamstate.EventReviewRejected,
		teamstate.EventWikiAccepted, teamstate.EventWikiRejected:
		key = "author"
	case teamstate.EventClaimTransferred:
		key = "to"
	case teamstate.EventClaimExpiring, teamstate.EventClaimExpired:
		key = "owner"
	}
	if key == "" {
		return nil
	}
	if v, ok := e.Data[key].(string); ok && strings.TrimSpace(v) != "" && v != e.Actor {
		return []string{strings.TrimSpace(v)}
	}
	return nil
}

// OutboxDir is where the messages to deliver later are kept, under the hub.
func OutboxDir(hubDir string) string { return filepath.Join(hubDir, "notify", "outbox") }

// ForTeam is the dispatcher of a team-state: the names and the handles of its
// members in the text, the outbox for what cannot be delivered now.
func ForTeam(ctx context.Context, src MemberLister, cfg *teamstate.TeamConfig, team string, box *Outbox, projectName func(id string) string) *Dispatcher {
	return NewDispatcher(cfg, WithPeople(NewPeople(ctx, src, projectName)), WithOutbox(box, team))
}
