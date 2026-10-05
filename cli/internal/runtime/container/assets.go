package container

import (
	"embed"
	"fmt"
)

//go:embed assets
var assets embed.FS

// bdBinary returns the embedded fake bd for a Linux architecture (amd64|arm64).
func bdBinary(arch string) ([]byte, error) {
	data, err := assets.ReadFile("assets/bd-linux-" + arch)
	if err != nil {
		return nil, fmt.Errorf("fake bd for linux/%s is not embedded in this oh build (run `make embed-sync`): %w", arch, err)
	}
	return data, nil
}
