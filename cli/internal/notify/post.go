package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// sendTimeout bounds one delivery to one webhook.
const sendTimeout = 8 * time.Second

func newHTTPClient(namespace string) *http.Client {
	return httplog.Wrap(&http.Client{Timeout: sendTimeout}, namespace, httplog.WithMaskURL())
}

// StatusError is a webhook that answered with an error status.
type StatusError struct {
	Service string
	Status  int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s webhook returned %d", e.Service, e.Status)
}

// Retryable reports whether the same message may be accepted later: a server
// that failed or is overloaded, not a webhook that was revoked or mistyped.
func (e *StatusError) Retryable() bool {
	return e.Status >= 500 || e.Status == http.StatusTooManyRequests || e.Status == http.StatusRequestTimeout
}

// UnreachableError is a webhook that could not be reached. The URL of the
// webhook is a secret: the error says why, never where.
type UnreachableError struct {
	Service string
	Cause   error
}

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("%s webhook unreachable: %v", e.Service, e.Cause)
}

func (e *UnreachableError) Unwrap() error { return e.Cause }

// IsRetryable reports whether a failed delivery is worth trying again.
func IsRetryable(err error) bool {
	var st *StatusError
	if errors.As(err, &st) {
		return st.Retryable()
	}
	var un *UnreachableError
	return errors.As(err, &un)
}

// postJSON posts payload to a webhook and reports the failure without the URL.
func postJSON(ctx context.Context, client *http.Client, service, webhook string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling %s message: %w", service, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
	if err != nil {
		return errors.New(i18n.Tf("v54n.notify.bad_webhook", service))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return &UnreachableError{Service: service, Cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &StatusError{Service: service, Status: resp.StatusCode}
	}
	return nil
}
