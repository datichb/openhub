package teamstate

import "strings"

// Event types that are mostly meant to be notified. They are not all written
// in the activity: claim.expiring is only a notification (the daemon says a
// lease is about to end, once per claim).
const (
	// EventClaimExpired: the lease of a claim ended without a renewal.
	// Data["owner"] is the member who held it.
	EventClaimExpired = "claim.expired"
	// EventClaimExpiring: the lease of a claim ends soon (Data["owner"],
	// Data["lease_until"]).
	EventClaimExpiring = "claim.expiring"
	// EventCustomNotification is the message of an agent or of
	// oh team notify test (Data["message"]).
	EventCustomNotification = "custom.notification"
)

// NotificationEvents are the event types a destination can subscribe to.
var NotificationEvents = []string{
	EventReviewReady, EventReviewApproved, EventReviewRejected,
	EventWikiProposal, EventWikiAccepted, EventWikiRejected,
	EventClaimTaken, EventClaimReleased, EventClaimTransferred, EventClaimConflict,
	EventClaimExpiring, EventClaimExpired, EventMemberRemoved,
	EventSessionComplete, EventAuditFinding,
	EventWorkflowPublished, EventWorkflowRestored, EventWorkflowArchived,
	EventConfigChanged, EventCustomNotification,
}

// DefaultNotificationEvents are the events a destination without an events
// list receives: the review and the wiki, and what an agent chose to say.
// The others (claims, sessions, configuration) are asked for by name.
var DefaultNotificationEvents = []string{"review.*", "wiki.*", EventCustomNotification}

// MatchEvent reports whether an events pattern selects an event type: the type
// itself, a family ("claim.*") or "*".
func MatchEvent(pattern, eventType string) bool {
	switch {
	case pattern == "*":
		return true
	case strings.HasSuffix(pattern, ".*"):
		return strings.HasPrefix(eventType, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == eventType
}

// ValidEventPattern reports whether a pattern selects at least one known event
// type (a typo selects none, and would silence the destination).
func ValidEventPattern(pattern string) bool {
	for _, t := range NotificationEvents {
		if MatchEvent(pattern, t) {
			return true
		}
	}
	return false
}

func wantsEvent(events []string, eventType string) bool {
	if len(events) == 0 {
		events = DefaultNotificationEvents
	}
	for _, p := range events {
		if MatchEvent(strings.TrimSpace(p), eventType) {
			return true
		}
	}
	return false
}

// WantsEvent reports whether the main webhook of the team receives eventType.
func (n NotificationConfig) WantsEvent(eventType string) bool {
	return wantsEvent(n.Events, eventType)
}

// WantsEvent is NotificationConfig.WantsEvent for a destination.
func (d NotificationDestination) WantsEvent(eventType string) bool {
	return wantsEvent(d.Events, eventType)
}
