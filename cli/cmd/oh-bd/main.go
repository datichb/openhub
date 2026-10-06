// Command oh-bd is the fake `bd` installed in oh container images. Beads
// always stays on the machine (D9): every call is sent with its working
// directory to the Beads gateway of the oh daemon, which checks it against
// the workflow allow-list (beads.allow) and runs the real bd on the machine;
// output and exit code are relayed.
//
// Backends: "gateway" (default) and "journal" (remote runs, P5-T12: reads
// from a snapshot, writes to a journal; see journal.go).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

// backend runs one bd command.
type backend interface {
	Exec(req beadswire.ExecRequest) (beadswire.ExecResponse, error)
}

type env func(string) string

func main() {
	cwd, _ := os.Getwd()
	os.Exit(run(os.Args[1:], cwd, os.Getenv, os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, cwd string, getenv env, stdin io.Reader, stdout, stderr io.Writer) int {
	b, err := selectBackend(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "bd: "+err.Error())
		return 1
	}
	req := beadswire.ExecRequest{Argv: args, Cwd: cwd}
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
			return nil, fmt.Errorf("beads is not reachable from this container: %s/%s are not set (the session was not started by oh with the Beads gateway)", beadswire.EnvURL, beadswire.EnvToken)
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
