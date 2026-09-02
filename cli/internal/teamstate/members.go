package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Member represents a team member registered in members.toml.
type Member struct {
	ID                 string `toml:"-"` // derived from the TOML key
	DisplayName        string `toml:"display_name"`
	GitLabUsername     string `toml:"gitlab_username"`
	MattermostUsername string `toml:"mattermost_username"`
	Role               string `toml:"role"`         // lead | dev | reviewer
	DefaultMode        string `toml:"default_mode"` // manual | semi-auto | auto
}

// membersFile is the TOML structure of members.toml.
type membersFile struct {
	Members map[string]Member `toml:"members"`
}

// ListMembers returns all members from members.toml.
func (r *Repo) ListMembers() ([]Member, error) {
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
	return members, nil
}

// GetMember retrieves a member by ID.
func (r *Repo) GetMember(id string) (*Member, error) {
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
func (r *Repo) FindMemberByGitLab(username string) (*Member, error) {
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
func (r *Repo) FindMemberByMattermost(username string) (*Member, error) {
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

// RemoveMember removes a member by ID from members.toml.
func (r *Repo) RemoveMember(ctx context.Context, id string) error {
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		mf, err := r.readMembersFile()
		if err != nil {
			return err
		}
		if _, exists := mf.Members[id]; !exists {
			return ErrMemberNotFound
		}
		delete(mf.Members, id)
		if err := r.writeMembersFile(mf); err != nil {
			return err
		}
		msg := fmt.Sprintf("members: remove %s", id)
		return r.commitAndPush(ctx, msg, "members.toml")
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
		if _, exists := mf.Members[m.ID]; !exists {
			return ErrMemberNotFound
		}
		mf.Members[m.ID] = m
		if err := r.writeMembersFile(mf); err != nil {
			return err
		}
		msg := fmt.Sprintf("members: update %s", m.ID)
		return r.commitAndPush(ctx, msg, "members.toml")
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
