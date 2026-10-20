//go:build !unix

package peercheck

import "os"

// FileOwner: the owner of a file is not read on this system.
func FileOwner(os.FileInfo) (int, bool) { return -1, false }
