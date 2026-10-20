// Package fakebd is the fake `bd` of oh sessions: installed in oh container
// images (cmd/oh-bd) and put first on the PATH of local sessions (hidden oh
// command, QB1). Beads always stays on the machine (D9): every call is sent
// with its working directory to the Beads gateway of the oh daemon, which
// checks it against the workflow allow-list (beads.allow) and runs the real
// bd on the machine; output and exit code are relayed.
//
// Backends: "gateway" (default) and "journal" (remote runs, P5-T12: reads
// from a snapshot, writes to a journal; see journal.go).
package fakebd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/peercheck"
)

// backend runs one bd command.
type backend interface {
	Exec(req beadswire.ExecRequest) (beadswire.ExecResponse, error)
}

type env func(string) string

// Main runs the fake bd with the process arguments and environment and
// returns its exit code.
func Main(args []string) int {
	cwd, _ := os.Getwd()
	return Run(args, cwd, os.Getenv, os.Stdin, os.Stdout, os.Stderr)
}

// Run sends one bd command to the selected backend and relays its outcome.
func Run(args []string, cwd string, getenv env, stdin io.Reader, stdout, stderr io.Writer) int {
	b, err := selectBackend(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "bd: "+err.Error())
		return 1
	}
	req := beadswire.ExecRequest{Argv: args, Cwd: cwd, GitHook: getenv(beadswire.EnvGitHook) == "1"}
	if req.GitHook && len(args) >= 3 && args[0] == "hooks" && args[1] == "run" {
		// Run by a git hook: the daemon checks that git runs this process
		// from the session hook and gives a one-time hook token (S-06).
		tok, err := hookToken(getenv, args)
		if err != nil {
			fmt.Fprintln(stderr, "bd: no hook token from the oh daemon: "+err.Error())
		}
		req.HookToken = tok
	}
	if wantsStdin(args) && stdin != nil {
		data, err := io.ReadAll(io.LimitReader(stdin, beadswire.MaxStdin+1))
		if err != nil {
			fmt.Fprintln(stderr, "bd: reading standard input: "+err.Error())
			return 1
		}
		if len(data) > beadswire.MaxStdin {
			fmt.Fprintf(stderr, "bd: standard input larger than %d bytes\n", beadswire.MaxStdin)
			return 1
		}
		req.Stdin = data
	}
	resp, err := b.Exec(req)
	if err != nil {
		fmt.Fprintln(stderr, "bd: "+err.Error())
		return 1
	}
	_, _ = stdout.Write(resp.Stdout)
	_, _ = stderr.Write(resp.Stderr)
	if resp.Error != "" {
		fmt.Fprintln(stderr, "bd: "+resp.Error)
		if resp.ExitCode == 0 {
			return 1
		}
	}
	return resp.ExitCode
}

func selectBackend(getenv env) (backend, error) {
	switch mode := getenv(beadswire.EnvMode); mode {
	case "", beadswire.ModeGateway:
		url, token := strings.TrimRight(getenv(beadswire.EnvURL), "/"), getenv(beadswire.EnvToken)
		if url == "" || token == "" {
			return nil, fmt.Errorf("beads is not reachable from this session: %s/%s are not set (the session was not started by oh with the Beads gateway)", beadswire.EnvURL, beadswire.EnvToken)
		}
		return &gatewayBackend{url: url, token: token, client: &http.Client{Timeout: 3 * time.Minute}}, nil
	case beadswire.ModeJournal:
		return journalBackend{snapshot: getenv(beadswire.EnvSnapshot), journal: getenv(beadswire.EnvJournal)}, nil
	default:
		return nil, fmt.Errorf("unknown %s %q", beadswire.EnvMode, mode)
	}
}

// wantsStdin reports whether bd will read its standard input (--stdin, or
// "-" as a file argument). Stdin is never read otherwise: agent shells may
// hand an open pipe that never closes.
func wantsStdin(args []string) bool {
	for _, a := range args {
		if a == "--stdin" || a == "-" || strings.HasSuffix(a, "=-") {
			return true
		}
	}
	return false
}

// hookToken asks the oh daemon, on its socket (set by the bd of local
// sessions), for the one-time token of a git hook ("" without a socket: the
// session is not local, the gateway skips the hook).
func hookToken(getenv env, args []string) (string, error) {
	sock := getenv(beadswire.EnvDaemonSocket)
	if sock == "" || getenv(beadswire.EnvMode) == beadswire.ModeJournal {
		return "", nil
	}
	body, err := json.Marshal(beadswire.HookTokenRequest{Token: getenv(beadswire.EnvToken), Argv: args})
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{
		// The token is sent to the socket only after checking that the user's
		// daemon is behind it (S-28).
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return peercheck.Dial(ctx, sock)
		},
	}}
	resp, err := client.Post("http://ohd"+beadswire.HookTokenPath, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var out beadswire.HookTokenResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	return out.HookToken, nil
}

// gatewayBackend sends the command to the oh daemon.
type gatewayBackend struct {
	url, token string
	client     *http.Client
}

func (g *gatewayBackend) Exec(req beadswire.ExecRequest) (beadswire.ExecResponse, error) {
	var out beadswire.ExecResponse
	body, err := json.Marshal(req)
	if err != nil {
		return out, err
	}
	hreq, err := http.NewRequest(http.MethodPost, g.url+beadswire.ExecPath, bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	hreq.Header.Set("Authorization", "Bearer "+g.token)
	hreq.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(hreq)
	if err != nil {
		return out, fmt.Errorf("the oh Beads gateway is not reachable (%v)", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("unexpected answer from the oh Beads gateway (%s)", resp.Status)
	}
	if resp.StatusCode >= 300 && out.Error == "" {
		out.Error = resp.Status
	}
	return out, nil
}
