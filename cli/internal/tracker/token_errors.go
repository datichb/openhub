package tracker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// TokenPurpose says what the token is wanted for; it chooses the settings the
// error tells the user to change.
type TokenPurpose string

// The purposes of a GitLab token.
const (
	// PurposeTracker: the ticket sync, the board, the tracker connection test.
	PurposeTracker TokenPurpose = "tracker"
	// PurposeMergeRequest: oh mr create, oh review feedback (the GitLab of the MCP).
	PurposeMergeRequest TokenPurpose = "mr"
	// PurposeTeam: the identity check on the GitLab of the team-state repository.
	PurposeTeam TokenPurpose = "team"
)

// Describe names a token for a message: "team token (key k)".
func (o TokenOrigin) Describe() string {
	if o.Source == TokenSourceNone {
		return ""
	}
	return i18n.Tf("tracker.token.source."+string(o.Source), o.Key)
}

// DescribeWithServer is Describe followed by the server it was sent to.
func (o TokenOrigin) DescribeWithServer() string {
	d := o.Describe()
	if d == "" {
		return i18n.Tf("tracker.token.server_only", displayServer(o.Server))
	}
	return i18n.Tf("tracker.token.origin", d, displayServer(o.Server))
}

// MissingTokenError is returned when no source holds a GitLab token for the
// server that is about to be called. It lists the sources read and tells how
// and where to declare a key for the server.
type MissingTokenError struct {
	// Server is the URL of the server called.
	Server  string
	Purpose TokenPurpose
	Looks   []TokenLook
	// TeamID is the team of the identity check (PurposeTeam).
	TeamID string
}

// NewMissingTokenError builds the error from the answer of the cascade.
func NewMissingTokenError(res GitLabTokenResult, purpose TokenPurpose, teamID string) *MissingTokenError {
	return &MissingTokenError{Server: res.Called, Purpose: purpose, Looks: res.Looks, TeamID: teamID}
}

func (e *MissingTokenError) Error() string {
	var b strings.Builder
	b.WriteString(i18n.Tf("tracker.token.missing", displayServer(e.Server)))
	b.WriteString("\n")
	if len(e.Looks) > 0 {
		b.WriteString(i18n.T("tracker.token.read_title"))
		b.WriteString("\n")
		for _, l := range e.Looks {
			fmt.Fprintf(&b, "  - %s : %s\n", TokenOrigin{Source: l.Source, Key: l.Key}.Describe(), e.outcome(l))
		}
	}
	url := hintURL(e.Server)
	switch e.Purpose {
	case PurposeMergeRequest:
		b.WriteString(i18n.Tf("tracker.token.hint.mr", url, url))
	case PurposeTeam:
		b.WriteString(i18n.Tf("tracker.token.hint.team", config.TeamGitLabTokenKey(e.TeamID)))
	default:
		b.WriteString(i18n.Tf("tracker.token.hint.tracker", url, url))
	}
	return b.String()
}

func (e *MissingTokenError) outcome(l TokenLook) string {
	switch l.Outcome {
	case OutcomeOtherServer:
		return i18n.Tf("tracker.token.outcome.other_server", displayServer(l.Server), displayServer(e.Server))
	case OutcomeNoServer:
		return i18n.T("tracker.token.outcome.no_server")
	}
	if l.Source == TokenSourceEnv {
		return i18n.T("tracker.token.outcome.unset")
	}
	return i18n.T("tracker.token.outcome.empty")
}

// IsMissingToken reports whether err is a MissingTokenError.
func IsMissingToken(err error) bool {
	var e *MissingTokenError
	return errors.As(err, &e)
}

// TokenRejectedError is a refusal of the token by the server (HTTP 401 or
// 403). It names the source of the token, its key and the server, so that the
// user knows which token to replace. It matches ErrTokenInvalid.
type TokenRejectedError struct {
	Origin TokenOrigin
	// Status is the HTTP status of the refusal.
	Status int
}

func (e *TokenRejectedError) Error() string {
	if e.Origin.Source == TokenSourceNone && e.Origin.Key == "" {
		return i18n.Tf("tracker.token.rejected_unknown", displayServer(e.Origin.Server), e.Status)
	}
	return i18n.Tf("tracker.token.rejected", displayServer(e.Origin.Server), e.Status, e.Origin.Describe(), refreshHint(e.Origin))
}

// rejected is the error for a refusal of the token of cfg by its server.
func (c Config) rejected(status int) error {
	o := c.Origin
	if o.Server == "" {
		o.Server = c.BaseURL
	}
	return &TokenRejectedError{Origin: o, Status: status}
}

// Is makes the error match ErrTokenInvalid, which the sync engine tests.
func (e *TokenRejectedError) Is(target error) bool { return target == ErrTokenInvalid }

// refreshHint is the command that replaces the rejected token.
func refreshHint(o TokenOrigin) string {
	if o.Source == TokenSourceEnv {
		return i18n.Tf("tracker.token.refresh_env", o.Key)
	}
	return i18n.Tf("tracker.token.refresh_key", o.Key)
}

// displayServer shows a server in a message: its normalized identity, or what
// was given when it cannot be reduced.
func displayServer(server string) string {
	if s := ServerOf(server); s != "" {
		return s
	}
	if server == "" {
		return "?"
	}
	return server
}

// hintURL is the URL to type in a command: the one called, with its scheme.
func hintURL(server string) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return "<url>"
	}
	if !strings.Contains(server, "://") {
		return "https://" + server
	}
	return strings.TrimRight(server, "/")
}
