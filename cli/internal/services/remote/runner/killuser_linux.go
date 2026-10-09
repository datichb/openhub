//go:build linux

package runner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// KillUser kills every process of the agent account (nil: nothing) and
// waits until none is left (zombies aside): no code of the agent may run
// while the runner, as root, reads its work tree (S-01).
func KillUser(u *User) error {
	if u == nil {
		return nil
	}
	for range 100 {
		pids := userPIDs("/proc", u.UID)
		if len(pids) == 0 {
			return nil
		}
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("processes of uid %d are still running", u.UID)
}

// userPIDs lists the live processes (zombies excluded) whose real,
// effective, saved or file-system uid is uid.
func userPIDs(proc string, uid uint32) []int {
	entries, err := os.ReadDir(proc)
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if procOfUser(filepath.Join(proc, e.Name(), "status"), uid) {
			out = append(out, pid)
		}
	}
	return out
}

func procOfUser(statusFile string, uid uint32) bool {
	f, err := os.Open(statusFile)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	want := strconv.FormatUint(uint64(uid), 10)
	match, zombie := false, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "State:"):
			fs := strings.Fields(line)
			zombie = len(fs) > 1 && (fs[1] == "Z" || fs[1] == "X")
		case strings.HasPrefix(line, "Uid:"):
			for _, f := range strings.Fields(line)[1:] {
				if f == want {
					match = true
				}
			}
		}
	}
	return match && !zombie
}
