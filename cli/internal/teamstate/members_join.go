package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Joining a team means having a card in members.toml. The card of a colleague
// is never replaced by the one of someone who types the same ID: JoinMember
// refuses it, unless the person says the card is theirs (JoinClaim), and then
// the card is kept as it is and only its empty fields are filled.

// JoinMode says what a person joining means for the card they name.
type JoinMode int

const (
	// JoinNew: the card is new, its ID must be free (ErrMemberExists
	// otherwise, as *MemberIDTakenError).
	JoinNew JoinMode = iota
	// JoinOwn: the card is the one the person already has in this team (the
	// member_id of hub.toml, a reinstallation): it is changed where the form
	// shows it; the rest of the record is kept.
	JoinOwn
	// JoinClaim: « C'est moi » - the card already exists and the person says
	// it is theirs. It is attached as it is: the empty fields are filled with
	// what the person gave, nothing that is filled is overwritten.
	JoinClaim
)

// JoinModeFor is what joining as id means: JoinOwn when ownID, the member_id
// hub.toml already has for the team, is that id (a reinstallation), JoinClaim
// when the person said « C'est moi », JoinNew otherwise.
func JoinModeFor(ownID, id string, claim bool) JoinMode {
	switch {
	case ownID != "" && strings.TrimSpace(id) == ownID:
		return JoinOwn
	case claim:
		return JoinClaim
	}
	return JoinNew
}

// ErrHandleTaken: the GitLab handle is already the one of another member.
var ErrHandleTaken = errors.New("GitLab handle already used by another member")

// MemberIDTakenError is ErrMemberExists with the card that has the ID, so that
// the caller can tell the person whose it is.
type MemberIDTakenError struct{ Member Member }

func (e *MemberIDTakenError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", ErrMemberExists, e.Member.ID, e.Member.DisplayName)
}

func (e *MemberIDTakenError) Unwrap() error { return ErrMemberExists }

// HandleTakenError is ErrHandleTaken with the member who has the handle.
type HandleTakenError struct {
	Handle string
	Member Member
}

func (e *HandleTakenError) Error() string {
	return fmt.Sprintf("%s: %s is %s (%s)", ErrHandleTaken, e.Handle, e.Member.ID, e.Member.DisplayName)
}

func (e *HandleTakenError) Unwrap() error { return ErrHandleTaken }

// JoinResult is what JoinMember did.
type JoinResult struct {
	// Member is the card as it is in the team-state after the join.
	Member Member
	// Created: the card did not exist, it was added.
	Created bool
	// Filled lists the fields of an existing card that were empty and got the
	// value the person gave (JoinClaim).
	Filled []string
	// Kept lists the fields the person gave that differ from the filled field
	// of the card; the card keeps its own (JoinClaim).
	Kept []string
}

// NormalizeHandle is a GitLab handle as the servers know it: trimmed, without
// the « @ » it is typed with.
func NormalizeHandle(h string) string {
	return strings.TrimPrefix(strings.TrimSpace(h), "@")
}

// CheckJoin tells whether m can join the members as mode says, without
// writing: nil, or *MemberIDTakenError, or *HandleTakenError. JoinMember checks
// the same against the latest state of the team-state; the forms call this to
// refuse early, on the list they show.
func CheckJoin(members []Member, m Member, mode JoinMode) error {
	byID := make(map[string]Member, len(members))
	for _, o := range members {
		byID[o.ID] = o
	}
	_, err := planJoin(byID, m, mode)
	return err
}

// planJoin is the card m becomes in the team-state (members by ID), or the
// reason it cannot join.
func planJoin(members map[string]Member, m Member, mode JoinMode) (JoinResult, error) {
	m.ID = strings.TrimSpace(m.ID)
	m.GitLabUsername = NormalizeHandle(m.GitLabUsername)
	if h := m.GitLabUsername; h != "" {
		for id, o := range members {
			if id != m.ID && strings.EqualFold(NormalizeHandle(o.GitLabUsername), h) {
				o.ID = id
				return JoinResult{}, &HandleTakenError{Handle: h, Member: o}
			}
		}
	}
	cur, exists := members[m.ID]
	cur.ID = m.ID
	if !exists {
		if m.Role == "" {
			m.Role = "dev"
		}
		if m.DefaultMode == "" {
			m.DefaultMode = "semi-auto"
		}
		return JoinResult{Member: m, Created: true}, nil
	}
	res := JoinResult{Member: cur}
	switch mode {
	case JoinOwn:
		if name := strings.TrimSpace(m.DisplayName); name != "" {
			res.Member.DisplayName = name
		}
		res.Member.GitLabUsername = m.GitLabUsername
		if m.Role != "" {
			res.Member.Role = m.Role
		}
		if m.MattermostUsername != "" {
			res.Member.MattermostUsername = strings.TrimSpace(m.MattermostUsername)
		}
		if m.TrackerUsername != "" {
			res.Member.TrackerUsername = NormalizeHandle(m.TrackerUsername)
		}
	case JoinClaim:
		fill := func(field string, have *string, given string) {
			given = strings.TrimSpace(given)
			switch {
			case given == "":
			case *have == "":
				*have = given
				res.Filled = append(res.Filled, field)
			case !strings.EqualFold(*have, given):
				res.Kept = append(res.Kept, field)
			}
		}
		fill("display_name", &res.Member.DisplayName, m.DisplayName)
		fill("gitlab_username", &res.Member.GitLabUsername, m.GitLabUsername)
		fill("role", &res.Member.Role, m.Role)
	default:
		return JoinResult{}, &MemberIDTakenError{Member: cur}
	}
	if res.Member.DefaultMode == "" {
		res.Member.DefaultMode = "semi-auto"
	}
	return res, nil
}

// JoinMember registers the card of a person who joins the team, in one
// transaction (pull, write, commit, push) on top of the latest state: the ID
// and the GitLab handle are checked against it. See JoinMode. Nothing is
// committed when the card is already as it should be.
func (r *Repo) JoinMember(ctx context.Context, m Member, mode JoinMode) (JoinResult, error) {
	var res JoinResult
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		res = JoinResult{}
		mf, err := r.readMembersFile()
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			mf = &membersFile{Members: make(map[string]Member)}
		}
		plan, err := planJoin(mf.Members, m, mode)
		if err != nil {
			return err
		}
		res = plan
		before, existed := mf.Members[plan.Member.ID]
		if existed && before == withID(plan.Member, "") {
			return nil
		}
		mf.Members[plan.Member.ID] = withID(plan.Member, "")
		if err := r.writeMembersFile(mf); err != nil {
			return err
		}
		verb := "update"
		if plan.Created {
			verb = "add"
		}
		return r.commitAndPush(ctx, fmt.Sprintf("members: %s %s", verb, plan.Member.ID), "members.toml")
	})
	return res, err
}

// withID returns m with the ID given (the ID is the TOML key, not a field).
func withID(m Member, id string) Member {
	m.ID = id
	return m
}
