package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

var (
	// ErrSoloExists is returned when the solo folder already holds files.
	ErrSoloExists = errors.New("folder already exists and is not empty")
	// ErrHasRemote is returned when promoting a team-state that already has
	// a remote.
	ErrHasRemote = errors.New("team-state already has a remote")
)

// SoloMemberRole is the role of the single member of a solo team-state.
const SoloMemberRole = "lead"

// InitSolo creates a solo team-state at path: a git repository without
// remote, the usual layout (workflows included), member as its only member
// (role lead) and the default governance, committed locally.
func InitSolo(ctx context.Context, path string, member Member) (*Repo, error) {
	if _, err := SafeName(member.ID); err != nil {
		return nil, err
	}
	if entries, err := os.ReadDir(path); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrSoloExists, path)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", path, err)
	}
	r := NewRepo("", path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.git(ctx, path, "init"); err != nil {
		return nil, fmt.Errorf("git init: %w", err)
	}
	// A predictable branch name, whatever init.defaultBranch says.
	if _, err := r.git(ctx, path, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		return nil, err
	}
	if err := r.InitStructure(ctx); err != nil {
		return nil, err
	}
	member.Role = SoloMemberRole
	if member.DefaultMode == "" {
		member.DefaultMode = "semi-auto"
	}
	if err := r.writeMembersFile(&membersFile{Members: map[string]Member{member.ID: member}}); err != nil {
		return nil, err
	}
	cfg, err := toml.Marshal(struct {
		Governance GovernanceConfig `toml:"governance"`
	}{DefaultGovernance()})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(path, "config.toml"), cfg, 0o644); err != nil {
		return nil, err
	}
	if err := r.commitAndPush(ctx, "team: init solo space", "."); err != nil {
		return nil, err
	}
	return r, nil
}

// IsLocalOnly reports whether the repo has no git remote (solo team-state).
func (r *Repo) IsLocalOnly(ctx context.Context) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.IsCloned() && r.localOnly(ctx)
}

// Promote turns a solo team-state into a shared one: it adds remote as
// origin and pushes the whole history. The remote must be empty; on failure
// the remote is removed again and the repo stays solo.
func (r *Repo) Promote(ctx context.Context, remote string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.IsCloned() {
		return ErrNotCloned
	}
	if !r.localOnly(ctx) {
		return ErrHasRemote
	}
	if _, err := r.git(ctx, r.path, "remote", "add", "origin", remote); err != nil {
		return fmt.Errorf("adding remote: %w", err)
	}
	if _, err := r.git(ctx, r.path, "push", "-u", "origin", "HEAD"); err != nil {
		_, _ = r.git(ctx, r.path, "remote", "remove", "origin")
		if isAuthError(err) {
			return fmt.Errorf("%s", authErrorMessage(remote))
		}
		return fmt.Errorf("pushing to %s: %w", remote, err)
	}
	r.remote = remote
	return nil
}
