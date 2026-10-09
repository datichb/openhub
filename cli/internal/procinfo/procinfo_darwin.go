//go:build darwin

package procinfo

import (
	"bytes"
	"encoding/binary"
	"errors"

	"golang.org/x/sys/unix"
)

func get(pid int) (Process, error) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return Process{}, err
	}
	if int(kp.Proc.P_pid) != pid {
		return Process{}, errors.New("procinfo: no such process")
	}
	p := Process{PID: pid, PPID: int(kp.Eproc.Ppid)}
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return p, err
	}
	p.Args = parseProcArgs(raw)
	return p, nil
}

// parseProcArgs reads kern.procargs2: argc (int32), the executable path,
// NUL padding, then argc NUL-terminated arguments (the environment follows).
func parseProcArgs(raw []byte) []string {
	if len(raw) < 4 {
		return nil
	}
	argc := int(binary.LittleEndian.Uint32(raw[:4]))
	rest := raw[4:]
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return nil
	}
	rest = rest[i:]
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}
	args := make([]string, 0, argc)
	for len(args) < argc && len(rest) > 0 {
		j := bytes.IndexByte(rest, 0)
		if j < 0 {
			args = append(args, string(rest))
			break
		}
		args = append(args, string(rest[:j]))
		rest = rest[j+1:]
	}
	return args
}
