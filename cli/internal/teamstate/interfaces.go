package teamstate

import (
	"context"
	"time"
)

// The team-state repository is used through small interfaces, one per
// sub-domain (claims, members, events, configuration, policies, wiki,
// patterns, takeover briefs), that *Repo implements all. A component takes
// the ones it needs and nothing more, so that a test fakes a few methods and
// not the whole repository.
//
// The reads of the interfaces are local and short (they read the clone, they
// do not run git and do not wait for the lock), which is why they take no
// context; what can wait or run git (Pull, CommitAndPush, Transact and every
// mutation) takes one. ClaimRemovedOnPurpose, which reads the history with
// git, takes one as well.

// RepoInfo describes the clone: where it is, where it comes from, what it
// holds.
type RepoInfo interface {
	Path() string
	Remote() string
	IsCloned() bool
	HasConfig() bool
	HasPolicies() bool
}

// RepoSync synchronizes the clone with the remote.
type RepoSync interface {
	Pull(ctx context.Context) error
	CommitAndPush(ctx context.Context, msg string, files ...string) error
	// Transact runs build as one transaction (pull, build, commit, push):
	// the methods called with its ctx join it.
	Transact(ctx context.Context, build func(ctx context.Context, tx *Tx) (TxResult, error)) error
}

// BoardStatusSetter tells the repo the statuses of the board of the team
// (custom columns), which UpdateClaimStatus and UpdateClaimStatusFromTracker
// accept in addition to the built-in ones.
type BoardStatusSetter interface {
	SetBoardStatuses(statuses []string)
}

// ClaimReader reads the claims.
type ClaimReader interface {
	ListClaims(project string) ([]Claim, error)
	GetClaim(project, ticketID string) (*Claim, error)
	IsStale(claim *Claim, staleDays int) bool
}

// ClaimWriter changes the claims of the members (take, release, hand over,
// move on the board, keep alive).
type ClaimWriter interface {
	CreateClaim(ctx context.Context, c Claim) (*Claim, error)
	ReleaseClaim(ctx context.Context, project, ticketID string) error
	TransferClaim(ctx context.Context, project, ticketID, newOwner string) error
	// ClaimPoolTicket assigns an unowned pool ticket (ClaimedBy="") to a member.
	// workStatus is the status to set (empty = ClaimStatusInProgress).
	ClaimPoolTicket(ctx context.Context, project, ticketID, memberID, workStatus string) error
	UpdateClaimStatus(ctx context.Context, project, ticketID, newStatus string) error
	// TouchClaims renews the lease of the claims of the member or of the
	// session in a single commit; none is made when every lease is still
	// long. It returns the claims it renewed.
	TouchClaims(ctx context.Context, refs []ClaimRef, member, session string, now time.Time) ([]ClaimRef, error)
}

// ClaimLabeler adds and removes the labels of a claim.
type ClaimLabeler interface {
	AddClaimLabel(ctx context.Context, project, ticketID, label string) error
	RemoveClaimLabel(ctx context.Context, project, ticketID, label string) error
}

// ClaimTrackerSync is what the synchronization with the tracker writes in
// the claims.
type ClaimTrackerSync interface {
	// CreateClaimLocal writes a claim file without committing or pushing.
	// Returns the repo-relative path for use in a batch CommitAndPush.
	// The caller MUST hold the write lock (via WithWriteLock).
	CreateClaimLocal(c Claim) (relPath string, err error)
	// UpdateClaimStatusFromTracker changes status without enforcing ValidTransitions.
	// Used by the tracker sync engine for arbitrary status jumps.
	UpdateClaimStatusFromTracker(ctx context.Context, project, ticketID, newStatus string) error
	// UpdateClaimMetadata updates title, description, and tracker_status without
	// changing the claim status. Used by the tracker sync engine.
	UpdateClaimMetadata(ctx context.Context, project, ticketID, title, description, trackerStatus string) error
	SetClaimExternalIID(ctx context.Context, project, ticketID string, iid int) error
}

// ClaimCleaner releases the done claims past their retention.
type ClaimCleaner interface {
	CleanupDoneClaims(ctx context.Context, retentionDays int, terminalStatuses ...string) ([]Claim, error)
}

// ClaimExpirer releases the claims whose lease is past.
type ClaimExpirer interface {
	ReleaseExpiredClaims(ctx context.Context, now time.Time, projects ...string) ([]Claim, error)
}

// ClaimRemovalJudge tells how a claim file left the repository (on purpose
// or by accident); it reads the history with git.
type ClaimRemovalJudge interface {
	ClaimRemovedOnPurpose(ctx context.Context, project, ticketID string) bool
}

// MemberReader reads the members of the team.
type MemberReader interface {
	HasMember(id string) bool
	ListMembers() ([]Member, error)
	GetMember(id string) (*Member, error)
	FindMemberByGitLab(username string) (*Member, error)
	FindMemberByMattermost(username string) (*Member, error)
}

// MemberWriter adds, removes and changes the members.
type MemberWriter interface {
	AddMember(ctx context.Context, m Member) error
	RemoveMember(ctx context.Context, id string) error
	UpdateMember(ctx context.Context, m Member) error
}

// EventReader reads the activity log.
type EventReader interface {
	ListEvents(project string, since time.Time) ([]Event, error)
	ListEventsLimited(project string, limit int) ([]Event, error)
}

// EventWriter appends to the activity log.
type EventWriter interface {
	AppendEvent(ctx context.Context, e Event) error
}

// ConfigReader reads the configuration of the team.
type ConfigReader interface {
	LoadConfig() (*TeamConfig, error)
}

// ConfigWriter saves the configuration of the team.
type ConfigWriter interface {
	SaveConfig(ctx context.Context, cfg *TeamConfig) error
}

// PolicyReader reads and checks the policies.
type PolicyReader interface {
	LoadPolicies(project string) ([]Policy, error)
	CheckAll(project string, pctx PolicyContext) ([]PolicyResult, error)
}

// PolicyWriter saves the policies.
type PolicyWriter interface {
	SavePolicies(ctx context.Context, policies map[string]Policy) error
}

// WikiReader reads the wiki pages and the pending proposals.
type WikiReader interface {
	WikiListPages() ([]string, error)
	WikiReadPage(name string) (string, error)
	WikiListPending() ([]WikiProposal, error)
}

// WikiWriter proposes, accepts and rejects wiki proposals.
type WikiWriter interface {
	WikiCreateProposal(ctx context.Context, p WikiProposal) error
	WikiAcceptProposal(ctx context.Context, id string) error
	WikiRejectProposal(ctx context.Context, id, reason string) error
}

// PatternReader reads the patterns.
type PatternReader interface {
	ListPatterns(tags []string, minMatchTags int) ([]Pattern, error)
	ReadPattern(name string) (string, error)
}

// PatternWriter creates, validates and removes patterns.
type PatternWriter interface {
	CreatePattern(ctx context.Context, p Pattern, content string) error
	ValidatePattern(ctx context.Context, name string) error
	RemovePattern(ctx context.Context, name string) error
}

// BriefReader reads the takeover briefs.
type BriefReader interface {
	ReadBrief(project, ticketID string) (string, error)
	ListBriefs(project string) ([]TakeoverMeta, error)
	BriefExists(project, ticketID string) bool
}

// BriefWriter generates and saves the takeover briefs.
type BriefWriter interface {
	GenerateRawBrief(ctx context.Context, project, ticketID, from, to, reason string) (*TakeoverBrief, error)
	SaveBrief(ctx context.Context, brief *TakeoverBrief) error
}

// TeamStateReader is the read-only view of a team-state repository: the
// composition of the readers of every sub-domain. Prefer the interface of the
// sub-domain a component needs; this one remains for the components (TUI
// views, commands) that have not been narrowed yet.
type TeamStateReader interface {
	RepoInfo
	ClaimReader
	MemberReader
	EventReader
	ConfigReader
	PolicyReader
	WikiReader
	PatternReader
	BriefReader
}

// TeamStateWriter extends TeamStateReader with the mutations that modify the
// team-state repository (create/update/delete + git commit/push): the
// composition of the writers of every sub-domain. Same remark as for
// TeamStateReader.
type TeamStateWriter interface {
	TeamStateReader
	RepoSync
	ClaimWriter
	ClaimLabeler
	ClaimTrackerSync
	ClaimCleaner
	MemberWriter
	EventWriter
	ConfigWriter
	PolicyWriter
	WikiWriter
	PatternWriter
	BriefWriter
}

// Compile-time interface compliance checks.
var (
	_ RepoInfo          = (*Repo)(nil)
	_ RepoSync          = (*Repo)(nil)
	_ BoardStatusSetter = (*Repo)(nil)
	_ ClaimReader       = (*Repo)(nil)
	_ ClaimWriter       = (*Repo)(nil)
	_ ClaimLabeler      = (*Repo)(nil)
	_ ClaimTrackerSync  = (*Repo)(nil)
	_ ClaimCleaner      = (*Repo)(nil)
	_ ClaimExpirer      = (*Repo)(nil)
	_ ClaimRemovalJudge = (*Repo)(nil)
	_ MemberReader      = (*Repo)(nil)
	_ MemberWriter      = (*Repo)(nil)
	_ EventReader       = (*Repo)(nil)
	_ EventWriter       = (*Repo)(nil)
	_ ConfigReader      = (*Repo)(nil)
	_ ConfigWriter      = (*Repo)(nil)
	_ PolicyReader      = (*Repo)(nil)
	_ PolicyWriter      = (*Repo)(nil)
	_ WikiReader        = (*Repo)(nil)
	_ WikiWriter        = (*Repo)(nil)
	_ PatternReader     = (*Repo)(nil)
	_ PatternWriter     = (*Repo)(nil)
	_ BriefReader       = (*Repo)(nil)
	_ BriefWriter       = (*Repo)(nil)
	_ TeamStateReader   = (*Repo)(nil)
	_ TeamStateWriter   = (*Repo)(nil)
)
