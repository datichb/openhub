// Package plugin manages opencode plugins (install, remove, status).
package plugin

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/semver"
)

//go:embed rtk.ts
var rtkPluginSource []byte

const (
	// RTKMinVersion is the minimum RTK CLI version required.
	RTKMinVersion = "0.42.0"

	// pluginsDir is the relative path from the opencode config directory.
	pluginsDir = "plugins"

	// rtkFileName is the plugin filename as loaded by opencode.
	rtkFileName = "rtk.ts"
)

// PluginStatus holds the current state of a plugin.
type PluginStatus struct {
	Installed   bool
	Version     string
	Path        string
	BinaryFound bool
	BinaryVer   string
}

// opencodePluginsDir returns the path to ~/.config/opencode/plugins/.
func opencodePluginsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", pluginsDir)
}

// RTKStatus checks the current state of the RTK plugin.
func RTKStatus() PluginStatus {
	status := PluginStatus{}

	// Check if plugin file is installed
	dir := opencodePluginsDir()
	if dir != "" {
		pluginPath := filepath.Join(dir, rtkFileName)
		if _, err := os.Stat(pluginPath); err == nil {
			status.Installed = true
			status.Path = pluginPath
		}
	}

	// Check RTK binary
	ver, err := CheckRTKBinary()
	if err == nil {
		status.BinaryFound = true
		status.BinaryVer = ver
	}

	return status
}

// RTKInstall deploys the RTK plugin to ~/.config/opencode/plugins/rtk.ts.
// It verifies that the rtk binary is available and compatible first.
func RTKInstall() error {
	// Verify RTK binary
	ver, err := CheckRTKBinary()
	if err != nil {
		return fmt.Errorf("rtk CLI non trouvé dans PATH. Installez-le avec:\n  brew install rtk\n  ou: cargo install rtk")
	}

	// Check version
	if !IsVersionAtLeast(ver, RTKMinVersion) {
		return fmt.Errorf("rtk %s est trop ancien (minimum requis: %s). Mettez à jour avec:\n  brew upgrade rtk\n  ou: cargo install rtk --force", ver, RTKMinVersion)
	}

	// Create plugins directory
	dir := opencodePluginsDir()
	if dir == "" {
		return fmt.Errorf("impossible de déterminer le répertoire plugins opencode")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("création du répertoire plugins: %w", err)
	}

	// Backup existing plugin
	pluginPath := filepath.Join(dir, rtkFileName)
	if _, err := os.Stat(pluginPath); err == nil {
		backupDir := filepath.Join(dir, ".backup")
		if err := os.MkdirAll(backupDir, 0o755); err == nil {
			backupName := fmt.Sprintf("rtk.ts.%s", time.Now().Format("20060102-150405"))
			_ = copyPluginFile(pluginPath, filepath.Join(backupDir, backupName))
		}
	}

	// Write the embedded plugin
	if err := os.WriteFile(pluginPath, rtkPluginSource, 0o644); err != nil {
		return fmt.Errorf("écriture du plugin: %w", err)
	}

	return nil
}

// RTKRemove removes the RTK plugin from ~/.config/opencode/plugins/.
func RTKRemove() error {
	dir := opencodePluginsDir()
	if dir == "" {
		return fmt.Errorf("impossible de déterminer le répertoire plugins opencode")
	}

	pluginPath := filepath.Join(dir, rtkFileName)
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("le plugin RTK n'est pas installé")
	}

	if err := os.Remove(pluginPath); err != nil {
		return fmt.Errorf("suppression du plugin: %w", err)
	}
	return nil
}

// CheckRTKBinary verifies that the rtk CLI is available and returns its version.
func CheckRTKBinary() (string, error) {
	path, err := exec.LookPath("rtk")
	if err != nil {
		return "", fmt.Errorf("rtk not found in PATH")
	}

	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("running rtk --version: %w", err)
	}

	version := strings.TrimSpace(string(out))
	// Output might be "rtk 0.45.0" or just "0.45.0"
	version = strings.TrimPrefix(version, "rtk ")
	version = strings.TrimPrefix(version, "v")
	return version, nil
}

// IsVersionAtLeast checks if version >= minimum (semantic version comparison).
func IsVersionAtLeast(version, minimum string) bool {
	return semver.IsAtLeast(version, minimum)
}

func copyPluginFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
