//go:build !windows

package opencodev2

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
)

func listServerProcesses(ctx context.Context) ([]adapters.ServerProcess, error) {
	procs, err := psServerPIDs(ctx)
	if err != nil || len(procs) == 0 {
		return procs, err
	}
	var dirs map[int][2]string
	if runtime.GOOS != "linux" {
		pids := make([]string, len(procs))
		for i, p := range procs {
			pids[i] = strconv.Itoa(p.PID)
		}
		dirs = lsofDirs(ctx, strings.Join(pids, ","))
	}
	for i := range procs {
		pid := procs[i].PID
		if runtime.GOOS == "linux" {
			procs[i].WorkDir, _ = os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
			procs[i].LogPath, _ = os.Readlink("/proc/" + strconv.Itoa(pid) + "/fd/1")
			continue
		}
		procs[i].WorkDir, procs[i].LogPath = dirs[pid][0], dirs[pid][1]
	}
	return procs, nil
}

// lsofDirs reads the current directory and the output file of processes
// (one lsof for all of them).
func lsofDirs(ctx context.Context, pids string) map[int][2]string {
	out := map[int][2]string{}
	data, err := exec.CommandContext(ctx, "lsof", "-w", "-n", "-P", "-a", "-p", pids, "-d", "cwd,1", "-Fpfn").Output()
	if len(data) == 0 && err != nil {
		return out
	}
	return parseLsof(string(data))
}

func parseLsof(data string) map[int][2]string {
	out := map[int][2]string{}
	pid, fd := 0, ""
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'f':
			fd = line[1:]
		case 'n':
			d := out[pid]
			switch fd {
			case "cwd":
				d[0] = line[1:]
			case "1":
				d[1] = line[1:]
			}
			out[pid] = d
		}
	}
	return out
}

// psServerPIDs lists the server processes from ps (pid, parent).
func psServerPIDs(ctx context.Context) ([]adapters.ServerProcess, error) {
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,command=").Output()
	if err != nil {
		return nil, err
	}
	return parsePS(string(out)), nil
}

func parsePS(out string) []adapters.ServerProcess {
	var procs []adapters.ServerProcess
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || !isOhServerCommand(strings.Join(f[2:], " ")) {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		if err1 == nil && err2 == nil {
			procs = append(procs, adapters.ServerProcess{PID: pid, PPID: ppid})
		}
	}
	return procs
}

// isOhServerCommand reports the command line of a server started by oh.
func isOhServerCommand(cmdline string) bool {
	f := strings.Fields(cmdline)
	for i := 0; i+4 < len(f); i++ {
		base := f[i][strings.LastIndexAny(f[i], `/\`)+1:]
		if strings.HasPrefix(base, "opencode") && f[i+1] == "serve" && f[i+2] == "--hostname" && f[i+4] == "--port" {
			return true
		}
	}
	return false
}
