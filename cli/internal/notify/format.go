package notify

import (
	"fmt"
	"math"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// FormatEvent is the text of a team event, in the language of the sender:
// « ✔ alice a terminé une session sur api/bd-7 ». "" for an event nobody
// reads (an unknown type).
func FormatEvent(e teamstate.Event) string { return FormatEventAs(e, nil) }

// FormatEventAs is FormatEvent with the names of the members and of the
// projects where FormatEvent has their IDs (nil = the IDs). The symbols are
// those of the activity (oh team activity): the same text works on every
// service, none uses a :shortcode: that only some of them know.
func FormatEventAs(e teamstate.Event, names teamstate.EventNames) string {
	if e.Type == teamstate.EventCustomNotification {
		if msg, ok := e.Data["message"]; ok {
			return fmt.Sprintf("%v", msg)
		}
		return ""
	}
	phrase := e.DescribeAs(names)
	if phrase == e.Type || phrase == "" {
		return "" // an event type oh does not know how to say
	}
	actor := e.Actor
	if names != nil && actor != "" {
		actor = names.Member(actor)
	}
	text := strings.TrimSpace(e.Glyph().Glyph + " " + strings.TrimSpace(actor+" "+phrase))
	return text + extras(e)
}

// extras are the details an event carries that its phrase does not say.
func extras(e teamstate.Event) string {
	var b strings.Builder
	switch e.Type {
	case teamstate.EventSessionComplete:
		if d, ok := e.Data["duration_s"].(float64); ok && d > 0 {
			fmt.Fprintf(&b, " (%s)", i18n.Tf("v54n.notify.minutes", int(math.Round(d/60))))
		}
		if s, ok := e.Data["summary"].(string); ok && s != "" {
			b.WriteString(" — " + s)
		}
	case teamstate.EventReviewReady:
		if u, ok := e.Data["mr_url"].(string); ok && u != "" {
			b.WriteString(" — " + u)
		}
	case teamstate.EventClaimConflict:
		if o, ok := e.Data["current_owner"].(string); ok && o != "" {
			b.WriteString(" (" + i18n.Tf("v54n.notify.already_taken_by", o) + ")")
		}
	case teamstate.EventAuditFinding:
		if n, ok := e.Data["finding_count"]; ok {
			fmt.Fprintf(&b, " (%v)", n)
		}
	}
	return b.String()
}
