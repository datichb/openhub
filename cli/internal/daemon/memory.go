package daemon

import (
	"context"
	"log/slog"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
)

// Memory cap (I6): the daemon measures the resident memory of the live tool
// servers (process trees) at each supervision. Above the cap, new sessions
// are queued (RunService, dequeue), idle groups are put to sleep, the
// oldest first and one per pass, and the user is warned once. Container
// servers count for their CLI process only (the container memory is not
// measured).

// MemoryFunc returns the resident memory (MB) of the process trees rooted
// at pids.
type MemoryFunc func(ctx context.Context, pids []int) (int, error)

// processTreeMemory reads the resident size of the process trees with ps
// (macOS, Linux).
func processTreeMemory(ctx context.Context, pids []int) (int, error) {
	if len(pids) == 0 || runtime.GOOS == "windows" {
		return 0, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "ps", "-axo", "pid=,ppid=,rss=").Output()
	if err != nil {
		return 0, err
	}
	return treeRSS(string(out), pids), nil
}

// treeRSS sums the resident size (ps KiB) of the trees rooted at pids, in MB.
func treeRSS(psOut string, pids []int) int {
	rss, children := map[int]int{}, map[int][]int{}
	for _, line := range strings.Split(psOut, "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		kb, err3 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		rss[pid] = kb
		children[ppid] = append(children[ppid], pid)
	}
	seen, total := map[int]bool{}, 0
	var walk func(int)
	walk = func(p int) {
		if seen[p] {
			return
		}
		seen[p] = true
		total += rss[p]
		for _, c := range children[p] {
			walk(c)
		}
	}
	for _, p := range pids {
		walk(p)
	}
	return total / 1024
}

// measureMemory updates the memory used by the ready servers.
func (d *Daemon) measureMemory(ctx context.Context, ready []domain.Server) {
	pids := make([]int, 0, len(ready))
	for _, s := range ready {
		if s.PID > 0 {
			pids = append(pids, s.PID)
		}
	}
	f := d.opts.Memory
	if f == nil {
		f = processTreeMemory
	}
	mb, err := f(ctx, pids)
	if err != nil {
		slog.Debug("ohd: memory not measured", "error", err)
		return
	}
	d.mu.Lock()
	d.memoryMB = mb
	d.mu.Unlock()
}

func (d *Daemon) memoryUsed() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.memoryMB
}

// memoryCap is the lowest memory cap of the open sessions of the ready
// groups (0 = none).
func (d *Daemon) memoryCap(ctx context.Context, ready []domain.Server) int {
	if d.opts.Sessions == nil || d.opts.SessionsDir == "" {
		return 0
	}
	groups := map[string]bool{}
	for _, s := range ready {
		groups[s.GroupKey] = true
	}
	all, err := d.opts.Sessions.List(ctx, "")
	if err != nil {
		return 0
	}
	limitMB := 0
	for _, s := range all {
		if !groups[s.GroupKey] || isTerminal(s.State) || s.State == domain.RunSleeping {
			continue
		}
		if l, err := limits.Load(d.opts.SessionsDir, s.ID); err == nil && l.MemoryMB > 0 && (limitMB == 0 || l.MemoryMB < limitMB) {
			limitMB = l.MemoryMB
		}
	}
	return limitMB
}

// enforceMemory puts one idle group to sleep when the servers use more than
// the cap, and warns once per crossing.
func (d *Daemon) enforceMemory(ctx context.Context, ready []domain.Server, held map[string]bool) {
	limitMB := d.memoryCap(ctx, ready)
	used := d.memoryUsed()
	over := limitMB > 0 && used > limitMB
	d.mu.Lock()
	warn := over && !d.memoryOver
	d.memoryOver = over
	d.mu.Unlock()
	if !over {
		return
	}
	if warn {
		slog.Warn("ohd: tool servers above the memory cap", "used_mb", used, "cap_mb", limitMB)
		if d.opts.Notify != nil {
			_ = d.opts.Notify(ctx, i18n.T("cmd.budget.memory_title"), i18n.Tf("cmd.budget.memory_over", used, limitMB))
		}
	}
	idle := make([]domain.Server, 0, len(ready))
	for _, s := range ready {
		if held[s.GroupKey] {
			continue
		}
		d.wmu.Lock()
		w := d.watchers[s.GroupKey]
		d.wmu.Unlock()
		if w == nil || !w.isSynced() {
			continue
		}
		if attached, _ := d.clientState(s.GroupKey); attached {
			continue
		}
		if executing, pending, _ := w.snapshot(); executing > 0 || pending > 0 {
			continue
		}
		idle = append(idle, s)
	}
	if len(idle) == 0 {
		return
	}
	sort.Slice(idle, func(i, j int) bool { return idle[i].LastActivityAt.Before(idle[j].LastActivityAt) })
	if d.toolBusy(ctx, idle[0]) {
		return
	}
	slog.Info("ohd: idle group put to sleep (memory cap)", "group", idle[0].GroupKey)
	d.sleepLocked(ctx, idle[0], false)
}
