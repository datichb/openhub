package adapters

import "context"

// ServerProcess is a running tool server started by oh (by any oh hub, or
// by a test), as the operating system shows it.
type ServerProcess struct {
	PID int
	// PPID is the parent process: 1 once the process that started it is
	// gone (the servers run in their own session).
	PPID int
	// WorkDir is the current directory of the process ("" if unknown).
	WorkDir string
	// LogPath is the file its output goes to ("" if unknown): the log oh
	// opened in the data directory of the server group.
	LogPath string
}

// ServerProcessLister finds the servers of the tool started by oh, to spot
// those nobody owns any more (oh doctor, oh gc: N18). Optional capability:
// without it, orphans are not looked for.
type ServerProcessLister interface {
	ServerProcesses(ctx context.Context) ([]ServerProcess, error)
	// StopServerProcess stops a server process and its process group.
	StopServerProcess(ctx context.Context, pid int) error
}
