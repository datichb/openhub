package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Bounds of execution.container_memory: below 512 MiB the tool server and
// the tests of a project are killed for nothing, above 1 TiB the limit means
// nothing.
const (
	minContainerMemoryMiB = 512
	maxContainerMemoryMiB = 1024 * 1024
)

var containerMemoryRe = regexp.MustCompile(`^([1-9]\d{0,9})([mg])$`)

// ParseContainerMemory validates execution.container_memory: a whole number
// of mebibytes (512m) or gibibytes (4g), 512m to 1024g, case-insensitive, and
// returns it normalized (lower case). "" is no limit.
func ParseContainerMemory(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", nil
	}
	m := containerMemoryRe.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("%q is not a memory size (4g, 512m)", s)
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] == "g" {
		n *= 1024
	}
	if n < minContainerMemoryMiB || n > maxContainerMemoryMiB {
		return "", fmt.Errorf("%q is out of range (512m to 1024g)", s)
	}
	return s, nil
}
