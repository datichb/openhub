package domain

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	projectSlugMax  = 32
	projectIDSuffix = 8
)

// NewProjectID is the ID of a new project: <slug>-<uuid8>, the slug being
// ProjectSlug(name). It is the only generator: the command line and the
// screens of the TUI call it, so that the ID of a project always tells which
// name it was given (doctor relies on it to find the claims of a project that
// was created again).
func NewProjectID(name string) string {
	return ProjectSlug(name) + "-" + uuid.New().String()[:projectIDSuffix]
}

// ProjectSlug is the name of a project as it appears in its ID: lowercase,
// spaces as dashes, only a-z, 0-9 and dashes, at most 32 characters,
// "project" when nothing is left.
func ProjectSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	var clean strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean.WriteRune(r)
		}
	}
	slug = strings.Trim(clean.String(), "-")
	if slug == "" {
		slug = "project"
	}
	if len(slug) > projectSlugMax {
		slug = slug[:projectSlugMax]
	}
	return strings.TrimRight(slug, "-")
}

var projectIDRe = regexp.MustCompile(`^(.+)-[0-9a-f]{8}$`)

// ProjectIDSlug is the slug of an ID made by NewProjectID; false for any other
// ID (a bare uuid8, an ID typed by hand).
func ProjectIDSlug(id string) (string, bool) {
	m := projectIDRe.FindStringSubmatch(id)
	if m == nil {
		return "", false
	}
	return m[1], true
}
