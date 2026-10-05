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
	fn  func(line string) (string, error)
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

func (f *fakeRunner) onFunc(cmd string, fn func(line string) (string, error)) *fakeRunner {
	f.resp[cmd] = fakeResp{fn: fn}
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
	best := ""
	for k := range f.resp {
		if (line == k || strings.HasPrefix(line, k+" ")) && len(k) > len(best) {
			best = k
		}
	}
	r, ok := f.resp[best]
	f.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fake: unexpected command %q", line)
	}
	if r.fn != nil {
		out, err := r.fn(line)
		return []byte(out), err
	}
	return []byte(r.out), r.err
}

func (f *fakeRunner) Stream(ctx context.Context, line func(string), name string, args ...string) error {
	out, err := f.Run(ctx, name, args...)
	if line != nil {
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l != "" {
				line(l)
			}
		}
	}
	return err
}

// called returns the recorded commands starting with prefix.
func (f *fakeRunner) called(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}
