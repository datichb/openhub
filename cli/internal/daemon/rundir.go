package daemon

import (
	"fmt"
	"os"
	"runtime"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/peercheck"
)

// securePrivateDir checks the folder of the daemon files (socket, lock,
// state) before the socket is created in it (audit v5, S-28): it must be a
// real folder of the user. If it is open to the group or the others (a
// folder created by an older oh, or by hand) it is closed (0700) first: the
// socket is created with the mode of the umask, then restricted, and in
// between nobody else must be able to reach it. A folder that belongs to
// someone else, or a link, is refused.
func securePrivateDir(dir string) error {
	uid := os.Getuid()
	if runtime.GOOS == "windows" || uid < 0 {
		return nil
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s", i18n.Tf("daemon.rundir.not_directory", dir))
	}
	if owner, ok := peercheck.FileOwner(fi); ok && owner != uid {
		return fmt.Errorf("%s", i18n.Tf("daemon.rundir.owner", dir, owner))
	}
	if fi.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("%s", i18n.Tf("daemon.rundir.chmod", dir, err))
		}
	}
	return nil
}
