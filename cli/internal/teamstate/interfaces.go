package teamstate

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// The team-state repository is used through small interfaces, one per
// sub-domain (claims, members, events, configuration, policies, wiki,
// patterns, takeover briefs), that *Repo implements all. A component takes
// the ones it needs and nothing more, so that a test fakes a few methods and
// not the whole repository.
//
// Everything that reads the clone or writes to it takes a context as its
// first parameter: the reads are local and short (they read the clone, they do
// not run git and do not wait for the lock), but they stop when the context is
// done, like the mutations, Pull, CommitAndPush and Transact, which can wait
// or run git. The few that take none (RepoInfo, IsStale, the claim queue kept
// in the clone, the workflow layers and lock) only stat or read a local file.

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
	ListClaims(ctx context.Context, project string) ([]Claim, error)
	GetClaim(ctx context.Context, project, ticketID string) (*Claim, error)
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

// ClaimLifecycle releases and hands over a claim with the events of what was
// done, signed by the member who acts (WithActor).
type ClaimLifecycle interface {
	ReleaseClaimAs(ctx context.Context, project, ticketID string, opts ReleaseOpts) (Claim, error)
	TransferClaimWith(ctx context.Context, project, ticketID, newOwner string, opts TransferOpts) (previousOwner string, err error)
}

// MemberClaimMover moves, releases and counts the claims of a member, and
// removes the card of a member in the same commit.
type MemberClaimMover interface {
	OpenClaimsOf(ctx context.Context, member string) (open, done []Claim, err error)
	ReassignClaims(ctx context.Context, opts ReassignOpts) (ReassignResult, error)
	HasLead(ctx context.Context) bool
}

// PendingWorkReader tells what a clone holds that the remote lacks.
type PendingWorkReader interface {
	PendingWork(ctx context.Context) (PendingWork, error)
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
	// SetClaimTrackerMirror records the state and labels of the issue seen at
	// this pass, what the next pass compares the issue with.
	SetClaimTrackerMirror(ctx context.Context, project, ticketID, state string, labels []string) error
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

// SyncLockReader reads the lease of the automatic tracker sync (sync.lock)
// from the clone, without network.
type SyncLockReader interface {
	ReadSyncLock() (*SyncLock, error)
}

// MemberReader reads the members of the team.
type MemberReader interface {
	HasMember(ctx context.Context, id string) bool
	ListMembers(ctx context.Context) ([]Member, error)
	GetMember(ctx context.Context, id string) (*Member, error)
	FindMemberByGitLab(ctx context.Context, username string) (*Member, error)
	FindMemberByMattermost(ctx context.Context, username string) (*Member, error)
}

// MemberWriter adds, removes and changes the members.
type MemberWriter interface {
	AddMember(ctx context.Context, m Member) error
	RemoveMember(ctx context.Context, id string) error
	UpdateMember(ctx context.Context, m Member) error
}

// MemberJoiner registers the card of a person who joins the team without ever
// replacing the card of a colleague (see JoinMode).
type MemberJoiner interface {
	JoinMember(ctx context.Context, m Member, mode JoinMode) (JoinResult, error)
}

// MemberPatcher changes one member in a transaction (the profile of the
// member who works in the team, without rewriting the others).
type MemberPatcher interface {
	PatchMember(ctx context.Context, id string, fn func(*Member) error) error
}

// EventReader reads the activity log.
type EventReader interface {
	ListEvents(ctx context.Context, project string, since time.Time) ([]Event, error)
	ListEventsLimited(ctx context.Context, project string, limit int) ([]Event, error)
}

// EventWriter appends to the activity log.
type EventWriter interface {
	AppendEvent(ctx context.Context, e Event) error
}

// ConfigReader reads the configuration of the team.
type ConfigReader interface {
	LoadConfig(ctx context.Context) (*TeamConfig, error)
}

// ConfigWriter changes the configuration of the team on top of its latest
// version: the patch runs on the config as the transaction read it (and runs
// again on the version of the remote when a push is refused); UpdateConfigFrom
// applies the changes between two copies and refuses to overwrite a setting a
// colleague changed (ErrConfigConflict).
type ConfigWriter interface {
	UpdateConfig(ctx context.Context, patch func(*TeamConfig) error) error
	UpdateConfigFrom(ctx context.Context, base, edited *TeamConfig) error
}

// PolicyReader reads and checks the policies.
type PolicyReader interface {
	LoadPolicies(ctx context.Context, project string) ([]Policy, error)
	CheckAll(ctx context.Context, project string, pctx PolicyContext) ([]PolicyResult, error)
}

// PolicyWriter saves the policies.
type PolicyWriter interface {
	SavePolicies(ctx context.Context, policies map[string]Policy) error
}

// WikiReader reads the wiki pages and the pending proposals.
type WikiReader interface {
	WikiListPages(ctx context.Context) ([]string, error)
	WikiReadPage(ctx context.Context, name string) (string, error)
	WikiListPending(ctx context.Context) ([]WikiProposal, error)
}

// WikiWriter proposes, accepts and rejects wiki proposals.
type WikiWriter interface {
	WikiCreateProposal(ctx context.Context, p WikiProposal) error
	WikiAcceptProposal(ctx context.Context, id string) error
	WikiRejectProposal(ctx context.Context, id, reason string) error
}

// PatternReader reads the patterns.
type PatternReader interface {
	ListPatterns(ctx context.Context, tags []string, minMatchTags int) ([]Pattern, error)
	ReadPattern(ctx context.Context, name string) (string, error)
}

// PatternWriter creates, validates and removes patterns.
type PatternWriter interface {
	CreatePattern(ctx context.Context, p Pattern, content string) error
	ValidatePattern(ctx context.Context, name string) error
	RemovePattern(ctx context.Context, name string) error
}

// BriefReader reads the takeover briefs.
type BriefReader interface {
	ReadBrief(ctx context.Context, project, ticketID string) (string, error)
	ListBriefs(ctx context.Context, project string) ([]TakeoverMeta, error)
	BriefExists(ctx context.Context, project, ticketID string) bool
}

// BriefWriter generates and saves the takeover briefs.
type BriefWriter interface {
	GenerateRawBrief(ctx context.Context, project, ticketID, from, to, reason string) (*TakeoverBrief, error)
	SaveBrief(ctx context.Context, brief *TakeoverBrief) error
}

// ClaimRenewer renews the lease of a claim its owner keeps working on.
type ClaimRenewer interface {
	RenewClaim(ctx context.Context, project, ticketID, member string, now time.Time) (time.Time, error)
}

// ClaimHeartbeater keeps the claims of a member or of a session alive: the
// signal of life of the daemon.
type ClaimHeartbeater interface {
	Heartbeat(ctx context.Context, refs []ClaimRef, member, session string, now time.Time) (HeartbeatResult, error)
}

// ClaimRehomer moves the claims of a member from a project ID that no project
// has any more to the ID of the project they belong to.
type ClaimRehomer interface {
	RehomeClaims(ctx context.Context, member string, moves []ClaimMove) (RehomeResult, error)
}

// ClaimAdopter hands a claim over to the session that takes the work up; the
// adoption is queued when the remote cannot be reached.
type ClaimAdopter interface {
	AdoptClaimDurable(ctx context.Context, project, ticketID, member, session string) (adopted, queued bool, err error)
	QueueAdoptClaim(project, ticketID, member, session string) (queued bool, err error)
	// QueueCreateClaim queues the taking of a ticket that has no claim yet.
	QueueCreateClaim(project, ticketID, member, session, status string) (queued bool, err error)
}

// ClaimAdoptionReader tells the session a queued adoption gave a claim to.
type ClaimAdoptionReader interface {
	QueuedAdoption(project, ticketID, member string) (string, bool)
}

// ClaimOpQueue is the durable queue of the claim operations kept in the
// clone: an operation is queued locally (never waits for git) and applied
// later by FlushClaimQueue.
type ClaimOpQueue interface {
	ClaimQueue() ([]ClaimOp, error)
	EnqueueClaimOp(op ClaimOp) error
	FlushClaimQueue(ctx context.Context) (applied, dropped int, err error)
}

// StructureEnsurer creates the layout of the team-state (directories, base
// files) when a clone lacks it.
type StructureEnsurer interface {
	EnsureStructure(ctx context.Context) error
}

// WorkflowLayoutReader reads the workflows of the team-state: the layers a
// catalog is loaded from, the projects that have workflows, the lock.
type WorkflowLayoutReader interface {
	LoadWorkflowScopes(cat *workflow.MemCatalog, scopes ...WorkflowScope) workflow.Diagnostics
	WorkflowProjects() ([]string, error)
	ReadWorkflowLock() (*WorkflowLock, error)
}

// WorkflowLayoutCommitter commits the layout of the workflows of a scope.
type WorkflowLayoutCommitter interface {
	CommitWorkflowLayout(ctx context.Context, scope WorkflowScope, msg string) error
}

// Compile-time interface compliance checks.
var (
	_ RepoInfo                = (*Repo)(nil)
	_ RepoSync                = (*Repo)(nil)
	_ BoardStatusSetter       = (*Repo)(nil)
	_ ClaimReader             = (*Repo)(nil)
	_ ClaimWriter             = (*Repo)(nil)
	_ ClaimLabeler            = (*Repo)(nil)
	_ ClaimTrackerSync        = (*Repo)(nil)
	_ ClaimCleaner            = (*Repo)(nil)
	_ ClaimExpirer            = (*Repo)(nil)
	_ ClaimRemovalJudge       = (*Repo)(nil)
	_ SyncLockReader          = (*Repo)(nil)
	_ ClaimHeartbeater        = (*Repo)(nil)
	_ ClaimRehomer            = (*Repo)(nil)
	_ ClaimRenewer            = (*Repo)(nil)
	_ ClaimAdopter            = (*Repo)(nil)
	_ ClaimAdoptionReader     = (*Repo)(nil)
	_ ClaimOpQueue            = (*Repo)(nil)
	_ StructureEnsurer        = (*Repo)(nil)
	_ WorkflowLayoutReader    = (*Repo)(nil)
	_ WorkflowLayoutCommitter = (*Repo)(nil)
	_ MemberReader            = (*Repo)(nil)
	_ MemberPatcher           = (*Repo)(nil)
	_ MemberJoiner            = (*Repo)(nil)
	_ MemberWriter            = (*Repo)(nil)
	_ EventReader             = (*Repo)(nil)
	_ EventWriter             = (*Repo)(nil)
	_ ConfigReader            = (*Repo)(nil)
	_ ConfigWriter            = (*Repo)(nil)
	_ PolicyReader            = (*Repo)(nil)
	_ PolicyWriter            = (*Repo)(nil)
	_ WikiReader              = (*Repo)(nil)
	_ WikiWriter              = (*Repo)(nil)
	_ PatternReader           = (*Repo)(nil)
	_ PatternWriter           = (*Repo)(nil)
	_ BriefReader             = (*Repo)(nil)
	_ BriefWriter             = (*Repo)(nil)
)
