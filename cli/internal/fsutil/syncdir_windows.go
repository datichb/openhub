//go:build windows

package fsutil

// syncDir does nothing on Windows: a directory cannot be opened and synced
// like a file.
func syncDir(string) {}
