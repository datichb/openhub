package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestMattermostSend(t *testing.T) {
	var received MattermostMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewMattermost(srv.URL, "dev-channel", "TestBot")
	err := client.Send(context.Background(), "Hello team!")
	require.NoError(t, err)

	assert.Equal(t, "Hello team!", received.Text)
	assert.Equal(t, "dev-channel", received.Channel)
	assert.Equal(t, "TestBot", received.Username)
}

func TestMattermostSendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewMattermost(srv.URL, "", "Bot")
	err := client.Send(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDispatcherDisabled(t *testing.T) {
	cfg := &teamstate.TeamConfig{
		Notification: teamstate.NotificationConfig{
			Enabled: false,
		},
	}
	d := NewDispatcher(cfg)

	err := d.Dispatch(context.Background(), teamstate.Event{
		Type:    teamstate.EventClaimTaken,
		Actor:   "benjamin",
		Project: "T-SRU",
		Ticket:  "SRU-142",
	})
	assert.NoError(t, err) // No-op, no error
}

func TestDispatcherSends(t *testing.T) {
	var received MattermostMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &teamstate.TeamConfig{
		Notification: teamstate.NotificationConfig{
			Enabled:           true,
			MattermostWebhook: srv.URL,
			Channel:           "test-channel",
			BotName:           "OpenHub",
		},
	}
	d := NewDispatcher(cfg)

	err := d.Dispatch(context.Background(), teamstate.Event{
		Type:    teamstate.EventClaimTaken,
		Actor:   "benjamin",
		Project: "T-SRU",
		Ticket:  "SRU-142",
	})
	require.NoError(t, err)
	assert.Contains(t, received.Text, "benjamin")
	assert.Contains(t, received.Text, "SRU-142")
	assert.Equal(t, "test-channel", received.Channel)
}

func TestFormatEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    teamstate.Event
		expected string
	}{
		{
			name: "session complete",
			event: teamstate.Event{
				Type:    teamstate.EventSessionComplete,
				Actor:   "benjamin",
				Project: "T-SRU",
				Ticket:  "SRU-142",
				Data:    map[string]interface{}{"duration_min": float64(75)},
			},
			expected: "[T-SRU] benjamin a terminé SRU-142 (75 min)",
		},
		{
			name: "claim taken",
			event: teamstate.Event{
				Type:    teamstate.EventClaimTaken,
				Actor:   "alice",
				Project: "T-SRU",
				Ticket:  "SRU-155",
			},
			expected: "[T-SRU] alice a pris SRU-155",
		},
		{
			name: "claim conflict",
			event: teamstate.Event{
				Type:    teamstate.EventClaimConflict,
				Actor:   "alice",
				Project: "T-SRU",
				Ticket:  "SRU-142",
				Data:    map[string]interface{}{"current_owner": "benjamin"},
			},
			expected: "[T-SRU] :warning: Conflit de claim sur SRU-142 (déjà pris par benjamin)",
		},
		{
			name: "review ready",
			event: teamstate.Event{
				Type:    teamstate.EventReviewReady,
				Actor:   "benjamin",
				Project: "T-SRU",
				Ticket:  "SRU-142",
				Data:    map[string]interface{}{"mr_url": "https://gitlab.com/mr/234"},
			},
			expected: "[T-SRU] Review prête pour SRU-142 — https://gitlab.com/mr/234",
		},
		{
			name: "wiki proposal",
			event: teamstate.Event{
				Type:    teamstate.EventWikiProposal,
				Actor:   "documentarian",
				Project: "T-SRU",
				Data:    map[string]interface{}{"page": "decisions"},
			},
			expected: "[Équipe] Proposition wiki (decisions) par documentarian (depuis T-SRU)",
		},
		{
			name: "claim released",
			event: teamstate.Event{
				Type:    teamstate.EventClaimReleased,
				Actor:   "benjamin",
				Project: "T-SRU",
				Ticket:  "SRU-142",
			},
			expected: "[T-SRU] benjamin a libéré SRU-142",
		},
		{
			name: "unknown event",
			event: teamstate.Event{
				Type:    "unknown.event",
				Actor:   "test",
				Project: "X",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEvent(tt.event)
			assert.Equal(t, tt.expected, got)
		})
	}
}
