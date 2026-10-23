package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
)

// Re-homing of claims: a project that was created again has a new ID, and the
// claims its member took under the previous one are invisible to it. The
// claims of the member move from one project directory to the other.

// ClaimMove names the project directories a member's claims move between.
type ClaimMove struct {
	From, To string
}

// RehomedClaim is one claim of a RehomeResult.
type RehomedClaim struct {
	From, To, TicketID string
}

// RehomeResult says what RehomeClaims did.
type RehomeResult struct {
	// Moved are the claims that now live under the new ID.
	Moved []RehomedClaim
	// Conflicts are the claims that stay under the old ID because the new ID
	// already has a claim for the same ticket (not overwritten).
	Conflicts []RehomedClaim
}

// RehomeClaims moves the claims of member from each move.From to move.To, all
// in one commit (the files are moved as they are: nothing of a claim is
// rewritten, so a key a newer oh wrote is kept). The claims of other members
// stay where they are, and so does a claim whose ticket already has one under
// the new ID. Running it again finds nothing to move and makes no commit.
// Offline it gives ErrOffline, nothing moved.
func (r *Repo) RehomeClaims(ctx context.Context, member string, moves []ClaimMove) (RehomeResult, error) {
	if member == "" {
		return RehomeResult{}, errors.New("re-homing claims: no member")
	}
	for _, m := range moves {
		for _, name := range []string{m.From, m.To} {
			if _, err := SafeName(name); err != nil {
				return RehomeResult{}, fmt.Errorf("invalid project name: %w", err)
			}
		}
		if m.From == m.To {
			return RehomeResult{}, fmt.Errorf("re-homing claims: %q is both the old and the new project", m.From)
		}
	}
	var res RehomeResult
	err := r.Transact(ctx, func(_ context.Context, tx *Tx) (TxResult, error) {
		res = RehomeResult{} // the build runs again when a push is rejected
		for _, m := range moves {
			claims, err := r.ListClaims(m.From)
			if err != nil {
				return TxResult{}, err
			}
			for _, c := range claims {
				if c.ClaimedBy != member {
					continue
				}
				item := RehomedClaim{From: m.From, To: m.To, TicketID: c.TicketID}
				oldRel, newRel := r.claimRelPath(m.From, c.TicketID), r.claimRelPath(m.To, c.TicketID)
				if _, err := tx.ReadFile(newRel); err == nil {
					res.Conflicts = append(res.Conflicts, item)
					continue
				} else if !os.IsNotExist(err) {
					return TxResult{}, err
				}
				data, err := tx.ReadFile(oldRel)
				if err != nil {
					return TxResult{}, err
				}
				if err := tx.WriteFile(newRel, data); err != nil {
					return TxResult{}, err
				}
				if err := tx.Remove(oldRel); err != nil {
					return TxResult{}, err
				}
				res.Moved = append(res.Moved, item)
			}
		}
		if len(res.Moved) == 0 {
			return TxResult{}, nil
		}
		return TxResult{Message: rehomeMessage(member, res.Moved)}, nil
	})
	if err != nil {
		return RehomeResult{}, err
	}
	return res, nil
}

func rehomeMessage(member string, moved []RehomedClaim) string {
	var targets []string
	for _, m := range moved {
		if t := m.From + " → " + m.To; !slices.Contains(targets, t) {
			targets = append(targets, t)
		}
	}
	return fmt.Sprintf("claims: %s re-homes %d claim(s) (%s)", member, len(moved), targets[0]+suffixMore(len(targets)-1))
}

func suffixMore(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(" +%d", n)
}
