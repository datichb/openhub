package container

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// fakeRunner is a scripted engine command-line tool.
type fakeRunner struct {
	mu    sync.Mutex
	bins  map[string]string   // name → path
	resp  map[string]fakeResp // "name arg arg…" (prefix match) → response
	calls []string
}

type fakeResp struct {
	out string
	err error
}

func newFakeRunner(bins ...string) *fakeRunner {
	f := &fakeRunner{bins: map[string]string{}, resp: map[string]fakeResp{}}
	for _, b := range bins {
		f.bins[b] = "/usr/bin/" + b
	}
	return f
}

func (f *fakeRunner) on(cmd, out string, err error) *fakeRunner {
	f.resp[cmd] = fakeResp{out: out, err: err}
	return f
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if p, ok := f.bins[name]; ok {
		return p, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	line := strings.Join(append([]string{name}, args...), " ")
	f.mu.Lock()
	f.calls = append(f.calls, line)
	f.mu.Unlock()
	best := ""
	for k := range f.resp {
		if (line == k || strings.HasPrefix(line, k+" ")) && len(k) > len(best) {
			best = k
		}
	}
	if best == "" {
		return nil, fmt.Errorf("fake: unexpected command %q", line)
	}
	r := f.resp[best]
	return []byte(r.out), r.err
}
