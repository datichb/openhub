package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	assert.False(t, strings.Contains(err.Error(), srv.URL), "the error never carries the webhook")
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
			Events:            []string{"claim.*"}, // the claims are not in the default events
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
