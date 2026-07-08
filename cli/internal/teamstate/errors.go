// Package teamstate manages the team-state Git repository transparently.
// It provides claim management, event logging, wiki access, and member registry.
package teamstate

import "errors"

// Sentinel errors for team-state operations.
var (
	// ErrNotCloned is returned when an operation requires the team-state repo
	// but it has not been cloned yet.
	ErrNotCloned = errors.New("team-state repository not cloned")
	// ErrSyncConflict is returned when a push fails after retries due to
	// concurrent modifications.
	ErrSyncConflict = errors.New("team-state sync conflict after retries")
	// ErrMemberExists is returned when attempting to add a member whose ID
	// already exists in members.toml.
	ErrMemberExists = errors.New("member already exists")
	// ErrMemberNotFound is returned when a member lookup fails.
	ErrMemberNotFound = errors.New("member not found")
	// ErrClaimExists is returned when a claim file already exists for a ticket.
	ErrClaimExists = errors.New("ticket already claimed")
	// ErrClaimNotFound is returned when attempting to release/transfer a claim
	// that does not exist.
	ErrClaimNotFound = errors.New("claim not found")
	// ErrWikiPageNotFound is returned when a requested wiki page does not exist.
	ErrWikiPageNotFound = errors.New("wiki page not found")
	// ErrProposalNotFound is returned when a wiki proposal ID is not found.
	ErrProposalNotFound = errors.New("wiki proposal not found")
	// ErrProposalTooLarge is returned when a wiki proposal exceeds the max line limit.
	ErrProposalTooLarge = errors.New("wiki proposal exceeds maximum size")
	// ErrProposalInvalid is returned when a wiki proposal fails validation.
	ErrProposalInvalid = errors.New("wiki proposal validation failed")
)
