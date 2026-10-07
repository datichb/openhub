package main

import (
	"os"

	"github.com/datichb/openhub/cli/cmd"
	"github.com/datichb/openhub/cli/internal/gateway/fakebd"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == cmd.BeadsShimArg {
		os.Exit(fakebd.Main(os.Args[2:]))
	}
	if err := cmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
