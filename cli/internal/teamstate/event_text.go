package teamstate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Glyph is the symbol and the tone of the event (glyph.Events): the same in
// the CLI and in the TUI.
func (e Event) Glyph() glyph.EventGlyph { return glyph.ForEvent(e.Type) }

// Describe is the phrase of the event after its actor ("a pris api/bd-7"):
// oh team activity, the Activity view and the Team Status view all use it. An
// event type it does not know is returned as it is.
func (e Event) Describe() string { return e.DescribeAs(nil) }

// EventNames names what an event refers to by ID: the project of a ticket,
// the member a claim went to. An ID it does not know is returned as it is.
type EventNames interface {
	Project(id string) string
	Member(id string) string
}

// DescribeAs is Describe with the names of the projects and of the members
// where Describe has their IDs ("a pris API/bd-7", "transféré à Alice"). nil
// names: the IDs.
func (e Event) DescribeAs(names EventNames) string {
	subject := e.subject(names)
	switch e.Type {
	case EventClaimTaken:
		return i18n.Tf("teamstate.event.claim_taken", subject)
	case EventClaimReleased:
		return i18n.Tf("teamstate.event.claim_released", subject)
	case EventClaimTransferred:
		if to := e.dataText("to"); to != "" {
			if names != nil {
				to = names.Member(to)
			}
			return i18n.Tf("teamstate.event.claim_transferred_to", subject, to)
		}
		return i18n.Tf("teamstate.event.claim_transferred", subject)
	case EventClaimConflict:
		return i18n.Tf("teamstate.event.claim_conflict", subject)
	case EventSessionComplete:
		if subject != "" {
			return i18n.Tf("teamstate.event.session_complete", subject)
		}
		return i18n.T("teamstate.event.session_complete_bare")
	case EventReviewReady:
		return i18n.Tf("teamstate.event.review_ready", subject)
	case EventReviewApproved:
		return i18n.Tf("teamstate.event.review_approved", subject)
	case EventReviewRejected:
		return withReason(i18n.Tf("teamstate.event.review_rejected", subject), e.dataText("reason"))
	case EventAuditFinding:
		return i18n.Tf("teamstate.event.audit_finding", subject)
	case EventWikiProposal:
		return i18n.Tf("teamstate.event.wiki_proposal", e.dataText("page"))
	case EventWikiAccepted:
		return i18n.Tf("teamstate.event.wiki_accepted", e.dataText("page"))
	case EventWikiRejected:
		return withReason(i18n.Tf("teamstate.event.wiki_rejected", e.dataText("page")), e.dataText("reason"))
	case EventWorkflowPublished:
		return i18n.Tf("teamstate.event.workflow_published", e.dataText("workflow"), e.dataText("version"))
	case EventWorkflowRestored:
		return i18n.Tf("teamstate.event.workflow_restored", e.dataText("workflow"), e.dataText("restored_from"), e.dataText("version"))
	case EventWorkflowArchived:
		return i18n.Tf("teamstate.event.workflow_archived", e.dataText("workflow"))
	}
	return e.Type
}

// subject is what the event is about: project and ticket when known.
func (e Event) subject(names EventNames) string {
	project := e.Project
	if names != nil && project != "" {
		project = names.Project(project)
	}
	switch {
	case project != "" && e.Ticket != "":
		return project + "/" + e.Ticket
	case project != "":
		return project
	}
	return e.Ticket
}

// dataText is a value of the data of the event as text ("" when absent); a
// whole number read back from JSON (float64) is written without decimals.
func (e Event) dataText(key string) string {
	v, ok := e.Data[key]
	if !ok || v == nil {
		return ""
	}
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// withReason adds the reason of a refusal after the phrase.
func withReason(phrase, reason string) string {
	if reason == "" {
		return phrase
	}
	return phrase + " : " + reason
}
