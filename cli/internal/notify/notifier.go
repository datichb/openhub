// Package notify provides notification dispatch to external channels.
// Supported: Mattermost, Slack, Discord, Microsoft Teams.
package notify

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Notifier is the interface for all notification backends.
type Notifier interface {
	// Send dispatches a text message to the configured channel.
	Send(ctx context.Context, text string) error
	// Name returns the notifier type name (e.g. "slack", "mattermost").
	Name() string
}

// Mentioner is a Notifier that can notify a person: Mention is how the
// service writes the mention of a user name.
type Mentioner interface {
	Mention(username string) string
}

// Spec is where a destination sends: enough to build its Notifier again
// (the outbox keeps it for the messages to try again).
type Spec struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	Channel string `json:"channel,omitempty"`
	BotName string `json:"bot_name,omitempty"`
}

// NewNotifier builds the backend of a destination; nil when the type or the
// URL is missing.
func NewNotifier(s Spec) Notifier {
	if s.URL == "" {
		return nil
	}
	bot := s.BotName
	if bot == "" {
		bot = "OpenHub"
	}
	switch s.Type {
	case "mattermost":
		return NewMattermost(s.URL, s.Channel, bot)
	case "slack":
		return NewSlack(s.URL, bot)
	case "discord":
		return NewDiscord(s.URL, bot)
	case "teams":
		return NewTeams(s.URL)
	}
	return nil
}

// target is one destination of a team with what it subscribed to.
type target struct {
	spec    Spec
	n       Notifier
	wants   func(eventType string) bool
	mention bool
}

// Dispatcher routes events to one or more notification backends.
type Dispatcher struct {
	targets []target
	people  People
	outbox  *Outbox
	team    string
}

// Option tunes a Dispatcher.
type Option func(*Dispatcher)

// WithPeople gives the names of the members and projects (their display names
// in the text) and the handles of the members (the mentions).
func WithPeople(p People) Option { return func(d *Dispatcher) { d.people = p } }

// WithOutbox keeps a message that could not be delivered, to try again later.
func WithOutbox(o *Outbox, team string) Option {
	return func(d *Dispatcher) { d.outbox, d.team = o, team }
}

// NewDispatcher creates a Dispatcher from team-state notification config.
// Returns a no-op dispatcher if notifications are disabled.
func NewDispatcher(cfg *teamstate.TeamConfig, opts ...Option) *Dispatcher {
	d := &Dispatcher{}
	for _, o := range opts {
		o(d)
	}
	n := cfg.Notification
	if !n.Enabled {
		return d
	}
	if len(n.Destinations) > 0 {
		for _, dest := range n.Destinations {
			d.add(Spec{Type: dest.Type, URL: dest.WebhookURL, Channel: dest.Channel, BotName: firstNonEmpty(dest.BotName, n.BotName)},
				dest.WantsEvent, dest.MentionOn())
		}
	} else {
		// Single destination (backward compat + type-based routing): the type is
		// the one of the webhook when none is written.
		d.add(Spec{Type: n.EffectiveType(), URL: n.EffectiveURL(), Channel: n.Channel, BotName: n.BotName},
			n.WantsEvent, n.MentionOn())
	}
	return d
}

func (d *Dispatcher) add(s Spec, wants func(string) bool, mention bool) {
	if n := NewNotifier(s); n != nil {
		d.targets = append(d.targets, target{spec: s, n: n, wants: wants, mention: mention})
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// Enabled reports whether at least one destination can receive messages.
func (d *Dispatcher) Enabled() bool { return len(d.targets) > 0 }

// Destinations is the number of destinations.
func (d *Dispatcher) Destinations() int { return len(d.targets) }

// Report is the outcome of a dispatch.
type Report struct {
	// Matched is the number of destinations that subscribed to the event.
	Matched int
	// Sent reached their destination; Queued did not and will be tried again.
	Sent, Queued int
	// Err is the last failure that was not queued (never carries a webhook).
	Err error
}

// Dispatch formats and sends a notification for the given event.
// Returns nil immediately if notifications are disabled. It returns the last
// error of a delivery that failed and is not kept to be tried again.
func (d *Dispatcher) Dispatch(ctx context.Context, e teamstate.Event) error {
	return d.Send(ctx, e).Err
}

// Send is Dispatch with the outcome: how many destinations received the
// message, how many will get it later.
func (d *Dispatcher) Send(ctx context.Context, e teamstate.Event) Report {
	var rep Report
	if !d.Enabled() {
		return rep
	}
	var names teamstate.EventNames
	if d.people != nil {
		names = d.people
	}
	text := FormatEventAs(e, names)
	if text == "" {
		return rep
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, t := range d.targets {
		if !t.wants(e.Type) {
			continue
		}
		rep.Matched++
		msg := text
		if t.mention {
			msg = d.withMentions(msg, t.n, e)
		}
		wg.Add(1)
		safego.Go(func() {
			defer wg.Done()
			err := t.n.Send(ctx, msg)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				slog.Debug("notify.dispatch.done", "type", t.n.Name(), "event", e.Type)
				rep.Sent++
			case d.outbox != nil && IsRetryable(err) && d.outbox.Add(t.spec, msg, d.team, err) == nil:
				slog.Warn("notify.dispatch.queued", "type", t.n.Name(), "event", e.Type, "error", err)
				rep.Queued++
			default:
				slog.Warn("notify.dispatch.failed", "type", t.n.Name(), "event", e.Type, "error", err)
				rep.Err = err
			}
		})
	}
	wg.Wait()
	return rep
}

// withMentions adds the mention of the person an event is for to its text,
// when the service knows how to mention and the person gave a handle.
func (d *Dispatcher) withMentions(text string, n Notifier, e teamstate.Event) string {
	m, ok := n.(Mentioner)
	if !ok || d.people == nil {
		return text
	}
	var handles []string
	for _, id := range Recipients(e) {
		if h := d.people.Handle(id); h != "" {
			handles = append(handles, m.Mention(h))
		}
	}
	if len(handles) == 0 {
		return text
	}
	return strings.Join(handles, " ") + " " + text
}
