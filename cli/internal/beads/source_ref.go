// Package beads — external reference helpers for linking beads to tracker tickets.
package beads

import (
	"fmt"
	"regexp"
	"strings"
)

// Supported external reference providers.
const (
	ProviderGitLab = "gitlab"
	ProviderJira   = "jira"
	ProviderGitHub = "github"
)

// ParseExternalRef splits an external_ref string into provider and ID.
// Examples:
//
//	"gitlab-693"       → ("gitlab", "693")
//	"jira-MYAPP-42"   → ("jira", "MYAPP-42")
//	"github-123"      → ("github", "123")
//
// Returns empty strings if the format is not recognized.
func ParseExternalRef(ref string) (provider, id string) {
	if ref == "" {
		return "", ""
	}
	for _, p := range []string{ProviderGitLab, ProviderGitHub, ProviderJira} {
		prefix := p + "-"
		if strings.HasPrefix(ref, prefix) && len(ref) > len(prefix) {
			return p, ref[len(prefix):]
		}
	}
	return "", ""
}

// BuildExternalRef constructs a formatted external_ref string.
// Examples:
//
//	("gitlab", "693") → "gitlab-693"
//	("jira", "MYAPP-42") → "jira-MYAPP-42"
func BuildExternalRef(provider, id string) string {
	if provider == "" || id == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s", provider, id)
}

// titleRefRe matches a [ref] prefix in a bead title, e.g. "[gitlab-693] Fix bug"
// or "[#42] Some task". The ref must contain at least one non-bracket character.
var titleRefRe = regexp.MustCompile(`^\[([^\]\s]+)\]\s*`)

// ExtractRefFromTitle extracts an external reference from a bead title
// that uses the [ref] convention (e.g. "[#42] Fix bug" or "[gitlab-693] Fix bug").
// Returns the inner ref string, or empty if no match.
func ExtractRefFromTitle(title string) string {
	m := titleRefRe.FindStringSubmatch(title)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// ExternalRefForTicket returns the ExternalRef for a ticket, falling back to
// parsing the title if the structured field is empty.
// This covers legacy beads created before --external-ref was set at creation.
func ExternalRefForTicket(t Ticket) string {
	if t.ExternalRef != "" {
		return t.ExternalRef
	}
	return ExtractRefFromTitle(t.Title)
}
