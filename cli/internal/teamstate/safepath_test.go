package teamstate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeName(t *testing.T) {
	for _, name := range []string{"conventions", "my-page", "ticket_BD-42", "2026-07-api-migration", "page.with.dots"} {
		got, err := SafeName(name)
		assert.NoError(t, err, "name: %q", name)
		assert.Equal(t, name, got, "a safe name is returned as is")
	}

	for _, tt := range []struct{ kind, name string }{
		{"traversal", "../etc/passwd"},
		{"encoded traversal", "..%2fetc%2fpasswd"},
		{"traversal in the middle", "page/../secret"},
		{"parent folder", ".."},
		{"slash", "wiki/page"},
		{"backslash", "projects\\claims"},
		{"absolute path", "/absolute"},
		{"deep path", "nested/deep/path"},
		{"empty", ""},
		{"null byte", "page\x00evil"},
	} {
		_, err := SafeName(tt.name)
		assert.True(t, errors.Is(err, ErrUnsafeName), "%s: %q is refused", tt.kind, tt.name)
	}
}

// Every public entry point that takes a name from the outside (an agent, a
// command line) refuses a name that would leave its folder.
func TestPublicAPIRefusesUnsafeNames(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()
	unsafe := func(err error) bool { return errors.Is(err, ErrUnsafeName) }

	apis := []struct {
		name    string
		refused func(bad string) bool
	}{
		{"GetClaim project", func(bad string) bool { _, err := repo.GetClaim(bad, "ticket"); return unsafe(err) }},
		{"GetClaim ticket", func(bad string) bool { _, err := repo.GetClaim("T-SRU", bad); return unsafe(err) }},
		{"CreateClaim project", func(bad string) bool {
			_, err := repo.CreateClaim(ctx, Claim{Project: bad, TicketID: "ticket", ClaimedBy: "attacker"})
			return unsafe(err)
		}},
		{"CreateClaim ticket", func(bad string) bool {
			_, err := repo.CreateClaim(ctx, Claim{Project: "T-SRU", TicketID: bad, ClaimedBy: "attacker"})
			return unsafe(err)
		}},
		{"ReleaseClaim project", func(bad string) bool { return unsafe(repo.ReleaseClaim(ctx, bad, "evil")) }},
		{"ReleaseClaim ticket", func(bad string) bool { return unsafe(repo.ReleaseClaim(ctx, "T-SRU", bad)) }},
		{"TransferClaim project", func(bad string) bool { return unsafe(repo.TransferClaim(ctx, bad, "evil", "alice")) }},
		{"TransferClaim ticket", func(bad string) bool { return unsafe(repo.TransferClaim(ctx, "T-SRU", bad, "alice")) }},
		{"UpdateClaimStatus project", func(bad string) bool { return unsafe(repo.UpdateClaimStatus(ctx, bad, "evil", ClaimStatusDone)) }},
		{"UpdateClaimStatus ticket", func(bad string) bool { return unsafe(repo.UpdateClaimStatus(ctx, "T-SRU", bad, ClaimStatusDone)) }},
		{"AddClaimLabel", func(bad string) bool { return unsafe(repo.AddClaimLabel(ctx, bad, "evil", "label")) }},
		{"RemoveClaimLabel", func(bad string) bool { return unsafe(repo.RemoveClaimLabel(ctx, bad, "evil", "label")) }},
		{"SetClaimExternalIID", func(bad string) bool { return unsafe(repo.SetClaimExternalIID(ctx, bad, "evil", 42)) }},
		{"ListBriefs", func(bad string) bool { _, err := repo.ListBriefs(bad); return unsafe(err) }},
		{"BriefExists project", func(bad string) bool { return !repo.BriefExists(bad, "passwd") }},
		{"BriefExists ticket", func(bad string) bool { return !repo.BriefExists("T-SRU", bad) }},
		{"WikiReadPage", func(bad string) bool { _, err := repo.WikiReadPage(bad); return unsafe(err) }},
		{"validateProposal page", func(bad string) bool {
			err := validateProposal(WikiProposal{Page: bad, Content: "Some content", Confidence: "CONFIRMED", Author: "documentarian"})
			return errors.Is(err, ErrProposalInvalid)
		}},
	}
	names := []struct{ kind, name string }{
		{"dot-dot traversal", "../../../etc/passwd"},
		{"slash", "foo/bar"},
		{"backslash", "foo\\bar"},
		{"null byte", "foo\x00bar"},
	}
	for _, api := range apis {
		t.Run(api.name, func(t *testing.T) {
			for _, n := range names {
				assert.True(t, api.refused(n.name), "%s: %q is refused", n.kind, n.name)
			}
		})
	}
}
