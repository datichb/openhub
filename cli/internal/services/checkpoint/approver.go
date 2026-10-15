package checkpoint

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Human approver of a checkpoint (CP-R9, step 1, ADR-055). A checkpoint
// may name its approvers (`approver:`, team layer only). On this machine oh
// cannot know who sits at the keyboard: it takes the person of the session
// as Beads does (ADR-052: git user.name, and user.email, where the session
// works; else the login name), shows the approvers on the card, warns when
// this person is none of them and traces the validation as out of
// approver (approver_mismatch). It never refuses it: step 2 (a remote
// session only the approver unblocks) is not done.

// Identity is the person who validates a checkpoint.
type Identity struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// String is the identity as git writes it ("Ada Lovelace <ada@example.com>").
func (i Identity) String() string {
	switch {
	case i.Name != "" && i.Email != "":
		return i.Name + " <" + i.Email + ">"
	case i.Email != "":
		return "<" + i.Email + ">"
	}
	return i.Name
}

// Matches reports whether the person is one of approvers: an entry with
// an @ is compared with the e-mail, the others with the name (case and
// spaces ignored).
func (i Identity) Matches(approvers []string) bool {
	name, email := sessionspec.NormalizeIdentity(i.Name), sessionspec.NormalizeIdentity(i.Email)
	for _, a := range approvers {
		n := sessionspec.NormalizeIdentity(a)
		if n == "" {
			continue
		}
		if strings.Contains(n, "@") {
			if email != "" && n == email {
				return true
			}
			continue
		}
		if name != "" && n == name {
			return true
		}
	}
	return false
}

// ApproverCheck is the approver of a checkpoint and the person who would
// validate it now.
type ApproverCheck struct {
	// Approver are the identities the checkpoint names (empty = none).
	Approver []string
	// Identity is the person of the session on this machine.
	Identity Identity
	// Mismatch: the checkpoint names approvers and Identity is none.
	Mismatch bool
}

// Expected reports whether the checkpoint names approvers.
func (a ApproverCheck) Expected() bool { return len(a.Approver) > 0 }

// ApproverText lists the approvers ("Ada Lovelace, lead@example.com").
func (a ApproverCheck) ApproverText() string { return strings.Join(a.Approver, ", ") }

// Approver returns the approver check of checkpoint id of a session (zero
// without a workflow; the identity is resolved even without approvers:
// every validation is traced with it).
func (s *Service) Approver(ctx context.Context, sessionID, id string) ApproverCheck {
	sess, wf, err := s.session(ctx, sessionID)
	if sess == nil {
		return ApproverCheck{}
	}
	var c sessionspec.CheckpointDef
	if err == nil && wf != nil {
		c, _ = wf.Checkpoint(id)
	}
	return s.approverCheck(ctx, sess, c)
}

func (s *Service) approverCheck(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) ApproverCheck {
	who := s.identity(ctx, sess.LaunchPath)
	ac := ApproverCheck{Approver: append([]string(nil), c.Approver...), Identity: who}
	ac.Mismatch = ac.Expected() && !who.Matches(c.Approver)
	return ac
}

// approval is the answer of the person of the session to checkpoint id
// (traced with their identity and, against the approvers of the
// checkpoint, returned too, the mismatch).
func (s *Service) approval(ctx context.Context, sessionID, id string, by domain.DecisionResolver, message string, override bool) (ap domain.CheckpointApproval, approver []string) {
	ap = domain.CheckpointApproval{By: string(by), Message: message, Override: override}
	sess, wf, err := s.session(ctx, sessionID)
	if sess == nil {
		return ap, nil
	}
	var c sessionspec.CheckpointDef
	if err == nil && wf != nil {
		c, _ = wf.Checkpoint(id)
	}
	ac := s.approverCheck(ctx, sess, c)
	ap.Identity, ap.ApproverMismatch = ac.Identity.String(), ac.Mismatch
	return ap, ac.Approver
}

// recordApproval records a timeline event of a validation or a refusal,
// with who made it and the approvers of the checkpoint.
func (s *Service) recordApproval(st *domain.CheckpointState, kind, id string, ap domain.CheckpointApproval, approver []string) {
	s.record(st, kind, id, ap.By, ap.Message)
	ev := &st.Timeline[len(st.Timeline)-1]
	ev.Identity, ev.ApproverMismatch = ap.Identity, ap.ApproverMismatch
	ev.Approver = append([]string(nil), approver...)
}

// JobIdentity is the identity of the validations of a remote job: the
// remote policy of the workflow, never a person (cmd/runner).
func JobIdentity(context.Context, string) Identity { return Identity{Name: RemoteAuthor} }

// identityTTL bounds how long the identity of a directory is reused (a
// TUI runs for hours; git is not read again at each validation).
const identityTTL = time.Minute

type cachedIdentity struct {
	id Identity
	at time.Time
}

func (s *Service) identity(ctx context.Context, dir string) Identity {
	if s.Identity != nil {
		return s.Identity(ctx, dir)
	}
	s.identityMu.Lock()
	c, ok := s.identities[dir]
	s.identityMu.Unlock()
	if ok && time.Since(c.at) < identityTTL {
		return c.id
	}
	id := GitIdentity(ctx, dir)
	s.identityMu.Lock()
	if s.identities == nil {
		s.identities = map[string]cachedIdentity{}
	}
	s.identities[dir] = cachedIdentity{id: id, at: time.Now()}
	s.identityMu.Unlock()
	return id
}

// GitIdentity is the person of a directory as bd names them (ADR-052):
// git user.name, and user.email, of the directory; else the login name.
func GitIdentity(ctx context.Context, dir string) Identity {
	var id Identity
	read := func(key string) string {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		args := []string{"config", key}
		if dir != "" {
			args = append([]string{"-C", dir}, args...)
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		if dir != "" {
			gitutil.AsOwner(cmd, dir)
		}
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return oneLine(string(out))
	}
	id.Name, id.Email = read("user.name"), read("user.email")
	if id.Name == "" {
		for _, k := range []string{"USER", "USERNAME", "LOGNAME"} {
			if v := oneLine(os.Getenv(k)); v != "" {
				id.Name = v
				break
			}
		}
	}
	return id
}

// oneLine keeps a value on one printable line.
func oneLine(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s))
}

// PayloadApprover reads the approvers of a checkpoint decision payload
// (DataApprover; a list, read back from JSON as []any).
func PayloadApprover(data map[string]any) []string {
	switch v := data[DataApprover].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		var out []string
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
