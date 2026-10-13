// Package prompt builds the bootstrap prompts of agentic sessions.
package prompt

import (
	"os"
	"path/filepath"
)

func dirExists(base, name string) bool {
	info, err := os.Stat(filepath.Join(base, name))
	return err == nil && info.IsDir()
}
