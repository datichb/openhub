package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

// Client talks to ohd over its Unix socket.
type Client struct {
	paths Paths
	http  *http.Client
}

// NewClient returns a client for the daemon at paths.
func NewClient(paths Paths) *Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", paths.Socket())
		},
	}
	return &Client{paths: paths, http: &http.Client{Transport: tr, Timeout: 30 * time.Second}}
}

// ErrNotRunning is returned when the daemon socket does not answer.
var ErrNotRunning = errors.New("ohd is not running")

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader = http.NoBody
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://ohd"+apiPrefix+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return ErrNotRunning
		}
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		return fmt.Errorf("ohd %s %s: %d %s", method, path, resp.StatusCode, e.Error)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Health returns the daemon status.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var h Health
	err := c.do(ctx, http.MethodGet, "/health", nil, &h)
	return h, err
}

// IssueGrant creates a proxy grant and returns the token and provider base URL.
func (c *Client) IssueGrant(ctx context.Context, req GrantRequest) (GrantResponse, error) {
	var out GrantResponse
	err := c.do(ctx, http.MethodPost, "/grants", req, &out)
	return out, err
}

// PendingGrants lists restored grants waiting for their secret.
func (c *Client) PendingGrants(ctx context.Context) ([]PendingGrant, error) {
	var out []PendingGrant
	err := c.do(ctx, http.MethodGet, "/grants/pending", nil, &out)
	return out, err
}

// ProvideSecret supplies the secret of a pending grant.
func (c *Client) ProvideSecret(ctx context.Context, token, secret string) error {
	return c.do(ctx, http.MethodPost, "/grants/secret", SecretRequest{Token: token, Secret: secret}, nil)
}

// RevokeOwner revokes every grant of a server group.
func (c *Client) RevokeOwner(ctx context.Context, owner string) error {
	return c.do(ctx, http.MethodDelete, "/owners/"+url.PathEscape(owner)+"/grants", nil, nil)
}

// Usage returns the proxy usage of a token.
func (c *Client) Usage(ctx context.Context, token string) (UsageResponse, error) {
	var out UsageResponse
	err := c.do(ctx, http.MethodGet, "/usage?token="+url.QueryEscape(token), nil, &out)
	return out, err
}

// Touch records activity of a server group.
func (c *Client) Touch(ctx context.Context, group string) error {
	return c.do(ctx, http.MethodPost, "/servers/"+url.PathEscape(group)+"/touch", nil, nil)
}

// Shutdown asks the daemon to stop (refused while sessions run, unless force).
func (c *Client) Shutdown(ctx context.Context, force bool) error {
	q := ""
	if force {
		q = "?force=true"
	}
	return c.do(ctx, http.MethodPost, "/shutdown"+q, nil, nil)
}

// EnsureOptions configures Ensure.
type EnsureOptions struct {
	Executable string        // oh binary (default os.Executable)
	Args       []string      // default: daemon run
	Version    string        // expected daemon version ("" = any)
	Timeout    time.Duration // default 10s
	Env        []string      // environment of the spawned daemon (default os.Environ)
}

// Ensure returns a client to a running daemon, spawning one if needed.
// A daemon of another version is replaced only when it has no live server.
func Ensure(ctx context.Context, paths Paths, opts EnsureOptions) (*Client, Health, error) {
	c := NewClient(paths)
	if h, err := c.Health(ctx); err == nil {
		if opts.Version == "" || h.Version == opts.Version {
			return c, h, nil
		}
		if h.Servers > 0 {
			slog.Warn("ohd runs another version and has live sessions; keeping it", "running", h.Version, "want", opts.Version)
			return c, h, nil
		}
		_ = c.Shutdown(ctx, false)
		waitGone(ctx, c, 5*time.Second)
	}

	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return nil, Health{}, err
	}
	unlock, err := blockingLock(paths.SpawnLock())
	if err != nil {
		return nil, Health{}, err
	}
	defer unlock()
	if h, err := c.Health(ctx); err == nil { // spawned by a concurrent client
		return c, h, nil
	}

	exe := opts.Executable
	if exe == "" {
		if exe, err = os.Executable(); err != nil {
			return nil, Health{}, err
		}
	}
	args := opts.Args
	if len(args) == 0 {
		args = []string{"daemon", "run"}
	}
	logf, err := os.OpenFile(paths.Log(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, Health{}, err
	}
	defer logf.Close()
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = logf, logf, nil
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		return nil, Health{}, fmt.Errorf("starting ohd: %w", err)
	}
	go func() { _ = cmd.Wait() }()

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h, err := c.Health(ctx); err == nil {
			return c, h, nil
		}
		select {
		case <-ctx.Done():
			return nil, Health{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, Health{}, fmt.Errorf("ohd did not start within %s (see %s)", timeout, paths.Log())
}

func waitGone(ctx context.Context, c *Client, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := c.Health(ctx); err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
