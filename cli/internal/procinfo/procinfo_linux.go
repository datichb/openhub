//go:build linux

package procinfo

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"strings"
)

func get(pid int) (Process, error) {
	dir := "/proc/" + strconv.Itoa(pid)
	stat, err := os.ReadFile(dir + "/stat")
	if err != nil {
		return Process{}, err
	}
	// pid (comm) state ppid …: comm may hold spaces and parentheses.
	end := bytes.LastIndexByte(stat, ')')
	if end < 0 {
		return Process{}, errors.New("procinfo: bad stat")
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 2 {
		return Process{}, errors.New("procinfo: bad stat")
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Process{}, err
	}
	p := Process{PID: pid, PPID: ppid}
	cmdline, err := os.ReadFile(dir + "/cmdline")
	if err != nil {
		return p, err
	}
	for _, a := range bytes.Split(bytes.TrimRight(cmdline, "\x00"), []byte{0}) {
		p.Args = append(p.Args, string(a))
	}
	return p, nil
}
