package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/fsutil"
)

// The outbox keeps the messages that could not reach their webhook (the
// server was down, the network too) and the daemon tries them again with a
// growing pause. One file per message, in the directory of the user (0600: it
// holds the webhook, which the team-state holds in the clear too). A message
// that still fails after the last pause is given up, and said.

// backoff is the pause before each new attempt: the first retry comes one
// minute after the failure.
var backoff = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour, 6 * time.Hour}

// outboxEntry is one message waiting for its destination.
type outboxEntry struct {
	ID        string    `json:"id"`
	Team      string    `json:"team,omitempty"`
	Spec      Spec      `json:"spec"`
	Text      string    `json:"text"`
	Attempts  int       `json:"attempts"`
	Created   time.Time `json:"created"`
	Next      time.Time `json:"next"`
	LastError string    `json:"last_error,omitempty"`
}

// Outbox is the queue of messages to try again.
type Outbox struct {
	dir string
	now func() time.Time
	mu  sync.Mutex
	// OnGiveUp is called with the text of a message that never got through
	// (the daemon puts it in the notification centre).
	OnGiveUp func(team, text string, err error)
	// Build makes the backend of a destination (tests replace it).
	Build func(Spec) Notifier
	seq   int
}

// NewOutbox returns the outbox that lives in dir (created on the first use).
func NewOutbox(dir string) *Outbox {
	return &Outbox{dir: dir, now: time.Now, Build: NewNotifier}
}

// Add keeps a message to try again.
func (o *Outbox) Add(spec Spec, text, team string, cause error) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := os.MkdirAll(o.dir, 0o700); err != nil {
		return err
	}
	now := o.now()
	o.seq++
	e := outboxEntry{ID: fmt.Sprintf("%d-%d-%d", now.UnixNano(), os.Getpid(), o.seq), Team: team, Spec: spec, Text: text,
		Created: now, Next: now.Add(backoff[0]), LastError: errText(cause)}
	return o.write(e)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (o *Outbox) write(e outboxEntry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomicNoSync(filepath.Join(o.dir, e.ID+".json"), data, 0o600)
}

func (o *Outbox) list() []outboxEntry {
	files, err := os.ReadDir(o.dir)
	if err != nil {
		return nil
	}
	var out []outboxEntry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(o.dir, f.Name()))
		if err != nil {
			continue
		}
		var e outboxEntry
		if json.Unmarshal(data, &e) != nil || e.ID == "" {
			_ = os.Remove(filepath.Join(o.dir, f.Name())) // unreadable: nothing to try
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out
}

// Pending is the number of messages waiting.
func (o *Outbox) Pending() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.list())
}

// Flushed is the outcome of a Flush.
type Flushed struct {
	Sent, Retry, GaveUp int
}

// Flush tries again the messages that are due. A message that gets through is
// removed; one that fails again waits longer; one that failed for good, or
// ran out of pauses, is given up.
func (o *Outbox) Flush(ctx context.Context) Flushed {
	o.mu.Lock()
	entries := o.list()
	o.mu.Unlock()
	var res Flushed
	now := o.now()
	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		if e.Next.After(now) {
			res.Retry++
			continue
		}
		n := o.Build(e.Spec)
		var err error
		if n == nil {
			err = errors.New("no destination")
		} else {
			err = n.Send(ctx, e.Text)
		}
		o.mu.Lock()
		switch {
		case err == nil:
			_ = os.Remove(filepath.Join(o.dir, e.ID+".json"))
			res.Sent++
		case !IsRetryable(err) || e.Attempts+1 >= len(backoff):
			_ = os.Remove(filepath.Join(o.dir, e.ID+".json"))
			res.GaveUp++
			slog.Warn("notify.outbox.gave_up", "team", e.Team, "type", e.Spec.Type, "attempts", e.Attempts+1, "error", err)
			if o.OnGiveUp != nil {
				o.OnGiveUp(e.Team, e.Text, err)
			}
		default:
			e.Attempts++
			e.Next = now.Add(backoff[e.Attempts])
			e.LastError = errText(err)
			_ = o.write(e)
			res.Retry++
		}
		o.mu.Unlock()
	}
	return res
}

// Reachable checks that the server of a webhook answers, without sending
// anything: any HTTP answer (even a refusal of a GET) says the server is
// there. The error never carries the URL.
func Reachable(ctx context.Context, webhook string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, webhook, http.NoBody)
	if err != nil {
		return errors.New("invalid webhook address")
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return err
	}
	_ = resp.Body.Close()
	return nil
}
