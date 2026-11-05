package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Member represents a team member registered in members.toml.
type Member struct {
	ID                 string `toml:"-"` // derived from the TOML key
	DisplayName        string `toml:"display_name"`
	GitLabUsername     string `toml:"gitlab_username"`
	MattermostUsername string `toml:"mattermost_username"`
	// TrackerUsername overrides GitLabUsername for tracker sync when the tracker
	// points to a different GitLab/Jira instance than the team-state repo.
	// Empty = fallback to GitLabUsername.
	TrackerUsername string `toml:"tracker_username,omitempty"`
	Role            string `toml:"role"`         // lead | dev | reviewer
	DefaultMode     string `toml:"default_mode"` // manuel (or manual) | semi-auto | auto
}

// membersFile is the TOML structure of members.toml.
type membersFile struct {
	Members map[string]Member `toml:"members"`
}

// ListMembers returns all members from members.toml.
func (r *Repo) ListMembers(ctx context.Context) ([]Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	mf, err := r.readMembersFile()
	if err != nil {
		return nil, err
	}
	members := make([]Member, 0, len(mf.Members))
	for id, m := range mf.Members {
		m.ID = id
		members = append(members, m)
	}
	// The file is a map: without a sort, the lists of the TUI and of the
	// commands change order on each read.
	slices.SortFunc(members, func(a, b Member) int { return strings.Compare(a.ID, b.ID) })
	return members, nil
}

// GetMember retrieves a member by ID.
func (r *Repo) GetMember(ctx context.Context, id string) (*Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	mf, err := r.readMembersFile()
	if err != nil {
		return nil, err
	}
	m, ok := mf.Members[id]
	if !ok {
		return nil, ErrMemberNotFound
	}
	m.ID = id
	return &m, nil
}

// FindMemberByGitLab looks up a member by GitLab username.
// Used by the tracker sync engine to map GitLab assignees to team member IDs.
func (r *Repo) FindMemberByGitLab(ctx context.Context, username string) (*Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	mf, err := r.readMembersFile()
	if err != nil {
		return nil, err
	}
	for id, m := range mf.Members {
		if strings.EqualFold(m.GitLabUsername, username) {
			m.ID = id
			return &m, nil
		}
	}
	return nil, ErrMemberNotFound
}

// FindMemberByMattermost looks up a member by Mattermost username.
// Used by the notification system to resolve @mentions to member IDs.
func (r *Repo) FindMemberByMattermost(ctx context.Context, username string) (*Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	mf, err := r.readMembersFile()
	if err != nil {
		return nil, err
	}
	for id, m := range mf.Members {
		if strings.EqualFold(m.MattermostUsername, username) {
			m.ID = id
			return &m, nil
		}
	}
	return nil, ErrMemberNotFound
}

// AddMember adds a new member to members.toml.
// Returns ErrMemberExists if the ID is already taken.
func (r *Repo) AddMember(ctx context.Context, m Member) error {
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		mf, err := r.readMembersFile()
		if err != nil {
			// If file doesn't exist, start fresh
			if os.IsNotExist(err) {
				mf = &membersFile{Members: make(map[string]Member)}
			} else {
				return err
			}
		}
		if _, exists := mf.Members[m.ID]; exists {
			return ErrMemberExists
		}
		mf.Members[m.ID] = m
		if err := r.writeMembersFile(mf); err != nil {
			return err
		}
		msg := fmt.Sprintf("members: add %s", m.ID)
		return r.commitAndPush(ctx, msg, "members.toml")
	})
}

// RemoveMember removes a member by ID from members.toml, in one commit with a
// member.removed event signed by the member of ctx (WithActor). Another
// member's card is removed by a lead only (ErrRemoveNeedsLead; a team with no
// lead is not locked); the claims of the member are left as they are, see
// ReassignClaims for a departure that hands them over.
func (r *Repo) RemoveMember(ctx context.Context, id string) error {
	return r.Transact(ctx, func(ctx context.Context, tx *Tx) (TxResult, error) {
		members, _, err := r.memberIDs()
		if err != nil {
			return TxResult{}, err
		}
		if _, exists := members[id]; !exists {
			return TxResult{}, ErrMemberNotFound
		}
		actor := ActorOf(ctx)
		if err := checkMemberRemoval(actor, members, id); err != nil {
			return TxResult{}, err
		}
		if err := r.removeMemberInTx(tx, id, actor, "", 0, 0); err != nil {
			return TxResult{}, err
		}
		return TxResult{Message: fmt.Sprintf("members: remove %s", id)}, nil
	})
}

// UpdateMember updates an existing member in members.toml.
// Returns ErrMemberNotFound if the ID does not exist.
func (r *Repo) UpdateMember(ctx context.Context, m Member) error {
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		mf, err := r.readMembersFile()
		if err != nil {
			return err
		}
		cur, exists := mf.Members[m.ID]
		if !exists {
			return ErrMemberNotFound
		}
		if err := checkHandleChange(mf.Members, m, cur.GitLabUsername); err != nil {
			return err
		}
		mf.Members[m.ID] = m
		if err := r.writeMembersFile(mf); err != nil {
			return err
		}
		msg := fmt.Sprintf("members: update %s", m.ID)
		return r.commitAndPush(ctx, msg, "members.toml")
	})
}

// PatchMember changes one member in a transaction: fn receives the member as
// it is on top of the remote and changes the fields it wants to, the rest of
// the record is kept as it is. One commit ("members: update <id>"), none when
// fn changed nothing. Returns ErrMemberNotFound if the ID does not exist; an
// error of fn commits nothing.
func (r *Repo) PatchMember(ctx context.Context, id string, fn func(*Member) error) error {
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		mf, err := r.readMembersFile()
		if err != nil {
			return err
		}
		m, exists := mf.Members[id]
		if !exists {
			return ErrMemberNotFound
		}
		m.ID = id
		before, role := m.GitLabUsername, m.Role
		if err := fn(&m); err != nil {
			return err
		}
		if err := checkHandleChange(mf.Members, m, before); err != nil {
			return err
		}
		if err := checkRoleChange(ActorOf(ctx), mf.Members, role, m.Role); err != nil {
			return err
		}
		mf.Members[id] = m
		if err := r.writeMembersFile(mf); err != nil {
			return err
		}
		return r.commitAndPush(ctx, fmt.Sprintf("members: update %s", id), "members.toml")
	})
}

// membersFilePath returns the absolute path to members.toml.
func (r *Repo) membersFilePath() string {
	return filepath.Join(r.path, "members.toml")
}

func (r *Repo) readMembersFile() (*membersFile, error) {
	data, err := os.ReadFile(r.membersFilePath())
	if err != nil {
		return nil, err
	}
	var mf membersFile
	if err := toml.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parsing members.toml: %w", err)
	}
	if mf.Members == nil {
		mf.Members = make(map[string]Member)
	}
	return &mf, nil
}

func (r *Repo) writeMembersFile(mf *membersFile) error {
	data, err := toml.Marshal(mf)
	if err != nil {
		return fmt.Errorf("marshaling members.toml: %w", err)
	}
	return os.WriteFile(r.membersFilePath(), data, 0o644)
}

// checkHandleChange refuses a GitLab handle that another member already has
// (the handle is unique in a team: the sync maps the issues by it). Only a
// handle that changes is checked: a duplicate that was already there does not
// block the edit of another field.
func checkHandleChange(members map[string]Member, m Member, before string) error {
	h := NormalizeHandle(m.GitLabUsername)
	if h == "" || strings.EqualFold(h, NormalizeHandle(before)) {
		return nil
	}
	for id, o := range members {
		if id != m.ID && strings.EqualFold(NormalizeHandle(o.GitLabUsername), h) {
			o.ID = id
			return &HandleTakenError{Handle: h, Member: o}
		}
	}
	return nil
}

// ErrRoleNeedsLead is returned when a member who is not a lead changes a role
// (their own included): the role lead is what the enforced settings are
// reserved to, so it cannot be taken by anyone. A team with no lead is not
// locked: anyone may then give the role.
var ErrRoleNeedsLead = errors.New("only a lead can change a role")

// ValidRoles are the roles of a member.
var ValidRoles = []string{RoleLead, "dev", "reviewer"}

// checkRoleChange refuses to a member who is not a lead (actor, the member who
// asks; "" for the code that has no member to blame, which is not checked) a
// change of role. Roles other than ValidRoles are refused.
func checkRoleChange(actor string, members map[string]Member, before, after string) error {
	if after == before {
		return nil
	}
	ok := after == ""
	for _, r := range ValidRoles {
		ok = ok || after == r
	}
	if !ok {
		return fmt.Errorf("role %q: possible values %s", after, strings.Join(ValidRoles, ", "))
	}
	if actor == "" {
		return nil
	}
	hasLead := false
	for id, m := range members {
		if m.Role == RoleLead {
			hasLead = true
			if id == actor {
				return nil
			}
		}
	}
	if hasLead {
		return ErrRoleNeedsLead
	}
	return nil
}
