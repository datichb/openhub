// Command oh-bd-stub is the fake `bd` installed in oh container images. Beads
// always stays on the machine (D9): the real gateway client comes with the
// Beads gateway (P4-T07); until then every call fails with a clear message.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "bd: Beads is not reachable from this oh container yet (the Beads gateway is not available in this version of oh).")
	os.Exit(1)
}
