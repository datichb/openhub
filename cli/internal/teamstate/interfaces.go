package teamstate

import (
	"context"
	"time"
)

// TeamStateReader is the read-only view of a team-state repository.
// Use this interface when a component only needs to query team data
// without performing any mutations (TUI views, MCP read handlers, etc.).
type TeamStateReader interface {
	// Repo metadata
	Path() string
	Remote() string
	IsCloned() bool
	HasConfig() bool
	HasPolicies() bool
	HasMember(id string) bool

	// Claims
	ListClaims(project string) ([]Claim, error)
	GetClaim(project, ticketID string) (*Claim, error)
	IsStale(claim *Claim, staleDays int) bool

	// Members
	ListMembers() ([]Member, error)
	GetMember(id string) (*Member, error)
	FindMemberByGitLab(username string) (*Member, error)
	FindMemberByMattermost(username string) (*Member, error)

	// Events
	ListEvents(project string, since time.Time) ([]Event, error)
	ListEventsLimited(project string, limit int) ([]Event, error)

	// Config & Policies
	LoadConfig() (*TeamConfig, error)
	LoadPolicies(project string) ([]Policy, error)
	CheckAll(project string, pctx PolicyContext) ([]PolicyResult, error)

	// Wiki
	WikiListPages() ([]string, error)
	WikiReadPage(name string) (string, error)
	WikiListPending() ([]WikiProposal, error)

	// Patterns
	ListPatterns(tags []string, minMatchTags int) ([]Pattern, error)
	ReadPattern(name string) (string, error)

	// Takeover briefs
	ReadBrief(project, ticketID string) (string, error)
	ListBriefs(project string) ([]TakeoverMeta, error)
	BriefExists(project, ticketID string) bool
}

// TeamStateWriter extends TeamStateReader with mutation methods that modify
// the team-state repository (create/update/delete + git commit/push).
type TeamStateWriter interface {
	TeamStateReader

	// Sync
	Pull(ctx context.Context) error
	CommitAndPush(ctx context.Context, msg string, files ...string) error

	// Claims
	CreateClaim(ctx context.Context, c Claim) (*Claim, error)
	ReleaseClaim(ctx context.Context, project, ticketID string) error
	TransferClaim(ctx context.Context, project, ticketID, newOwner string) error
	UpdateClaimStatus(ctx context.Context, project, ticketID, newStatus string) error
	AddClaimLabel(ctx context.Context, project, ticketID, label string) error
	RemoveClaimLabel(ctx context.Context, project, ticketID, label string) error
	SetClaimExternalIID(ctx context.Context, project, ticketID string, iid int) error
	CleanupDoneClaims(ctx context.Context, retentionDays int) ([]Claim, error)

	// Members
	AddMember(ctx context.Context, m Member) error
	RemoveMember(ctx context.Context, id string) error
	UpdateMember(ctx context.Context, m Member) error

	// Events
	AppendEvent(ctx context.Context, e Event) error

	// Config & Policies
	SaveConfig(ctx context.Context, cfg *TeamConfig) error
	SavePolicies(ctx context.Context, policies map[string]Policy) error

	// Wiki
	WikiCreateProposal(ctx context.Context, p WikiProposal) error
	WikiAcceptProposal(ctx context.Context, id string) error
	WikiRejectProposal(ctx context.Context, id string) error

	// Patterns
	CreatePattern(ctx context.Context, p Pattern, content string) error
	ValidatePattern(ctx context.Context, name string) error
	RemovePattern(ctx context.Context, name string) error

	// Takeover briefs
	GenerateRawBrief(ctx context.Context, project, ticketID, from, to, reason string) (*TakeoverBrief, error)
	SaveBrief(ctx context.Context, brief *TakeoverBrief) error
}

// Compile-time interface compliance checks.
var (
	_ TeamStateReader = (*Repo)(nil)
	_ TeamStateWriter = (*Repo)(nil)
)
