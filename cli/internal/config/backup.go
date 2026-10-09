package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// configBackupsKept is the number of hub.toml.bak-<time> copies kept.
const configBackupsKept = 3

// backupConfig copies the current hub.toml to hub.toml.bak-<time> before it
// is replaced by next, when it is valid TOML and differs: oh doctor --fix
// restores the latest valid copy of a hub.toml that no longer loads (N13).
func backupConfig(path string, next []byte) {
	cur, err := os.ReadFile(path)
	if err != nil || bytes.Equal(cur, next) {
		return
	}
	var m map[string]any
	if toml.Unmarshal(cur, &m) != nil {
		return
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	if os.WriteFile(path+".bak-"+stamp, cur, 0o600) != nil {
		return
	}
	old, _ := filepath.Glob(path + ".bak-*")
	sort.Strings(old) // the stamp sorts by time
	for i := 0; i < len(old)-configBackupsKept; i++ {
		_ = os.Remove(old[i])
	}
}
