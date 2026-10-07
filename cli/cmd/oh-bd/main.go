// Command oh-bd is the fake `bd` installed in oh container images (see
// internal/gateway/fakebd).
package main

import (
	"os"

	"github.com/datichb/openhub/cli/internal/gateway/fakebd"
)

func main() { os.Exit(fakebd.Main(os.Args[1:])) }
