package checkpoint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Tag window (CP-R8, p4-debts): like the commit window once the work is
// committed, the tag window closes after the first tag. oh notes the local
// tags when the window opens; once a shell call ends with the tag window
// open (and no longer waiting for the release commit) and the tags changed,
// the window closes. A tag unlocked with the commit waits for the release
// commit first (bundle.TagWaitsForCommit), so the commit that closes the
// commit window never closes the tag window.

// GitTags returns a fingerprint of the local tags of dir (names and
// objects; a moved or deleted tag changes it too).
func GitTags(ctx context.Context, dir string) (string, error) {
	if err := gitutil.Check(ctx, dir); err != nil {
		return "", err
	}
	out, err := gitutil.Command(ctx, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/tags").Output()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) tagsErr(ctx context.Context, dir string) (string, error) {
	if dir == "" {
		return "", nil
	}
	f := s.Tags
	if f == nil {
		f = GitTags
	}
	return f(ctx, dir)
}

// tagsOf is the fingerprint noted when the tag window opens ("" when git
// cannot be read: the window then closes only at the next mandatory
// checkpoint or when its checkpoint is asked again).
func (s *Service) tagsOf(ctx context.Context, dir string) string {
	t, err := s.tagsErr(ctx, dir)
	if err != nil {
		return ""
	}
	return t
}

// onShellTag closes the tag window after a shell call that created (or
// changed) a tag. A git read that fails closes it too (fails closed; the
// checkpoint asked again opens it). changed reports that the session rules
// changed.
func (s *Service) onShellTag(ctx context.Context, sess *domain.Session, call adapters.ToolCall) (changed bool, err error) {
	if call.Status == adapters.CallCalled {
		return false, nil
	}
	st, err := s.state(ctx, sess.ID)
	if err != nil {
		return false, err
	}
	u, open := st.Unlocks[sessionspec.UnlockTag]
	if !open || u.Tags == "" || bundle.TagWaitsForCommit(&st) {
		return false, nil
	}
	tags, terr := s.tagsErr(ctx, sess.LaunchPath)
	reason := sessionspec.UnlockTag
	switch {
	case terr != nil:
		reason += " (git: " + terr.Error() + ")"
	case tags == u.Tags:
		return false, nil
	}
	_, err = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
		cur, ok := st.Unlocks[sessionspec.UnlockTag]
		if !ok || cur.Checkpoint != u.Checkpoint || !cur.At.Equal(u.At) {
			return errUnchanged
		}
		delete(st.Unlocks, sessionspec.UnlockTag)
		s.record(st, domain.CheckpointWindow, u.Checkpoint, "", reason)
		changed = true
		return nil
	})
	if errors.Is(err, errUnchanged) {
		err = nil
	}
	return changed, err
}
