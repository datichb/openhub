package teamstate

import (
	"fmt"
	"strings"
)

// ErrUnsafeName is returned when a name contains path traversal or invalid characters.
var ErrUnsafeName = fmt.Errorf("unsafe name: contains invalid characters")

// SafeName validates that a name is safe for use as a filesystem path segment.
// It rejects empty strings, strings containing "..", "/", "\", or null bytes.
// This prevents path traversal attacks from MCP tool inputs.
func SafeName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: name is empty", ErrUnsafeName)
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("%w: %q contains path traversal", ErrUnsafeName, name)
	}
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("%w: %q contains path separator", ErrUnsafeName, name)
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: %q contains null byte", ErrUnsafeName, name)
	}
	return name, nil
}
