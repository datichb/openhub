package opencode

import (
	"fmt"
	"runtime"
)

// archiveFormatZip indicates a ZIP archive.
const archiveFormatZip = "zip"

// archiveFormatTarGz indicates a gzipped tar archive.
const archiveFormatTarGz = "tar.gz"

// platformAsset maps GOOS/GOARCH to the GitHub release asset name and archive format.
// opencode uses "x64" instead of Go's "amd64" and "arm64" stays the same.
type platformAsset struct {
	Name   string
	Format string
}

var platformMap = map[string]platformAsset{
	"darwin/arm64": {Name: "opencode-darwin-arm64.zip", Format: archiveFormatZip},
	"darwin/amd64": {Name: "opencode-darwin-x64.zip", Format: archiveFormatZip},
	"linux/arm64":  {Name: "opencode-linux-arm64.tar.gz", Format: archiveFormatTarGz},
	"linux/amd64":  {Name: "opencode-linux-x64.tar.gz", Format: archiveFormatTarGz},
}

// AssetName returns the GitHub release asset filename and archive format
// for the current platform.
func AssetName() (name, format string, err error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	asset, ok := platformMap[key]
	if !ok {
		return "", "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return asset.Name, asset.Format, nil
}

// AssetNameFor returns the asset info for a specific OS/arch combination.
// Useful for testing.
func AssetNameFor(goos, goarch string) (name, format string, err error) {
	key := goos + "/" + goarch
	asset, ok := platformMap[key]
	if !ok {
		return "", "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
	}
	return asset.Name, asset.Format, nil
}
