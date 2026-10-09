//go:build !darwin && !linux

package procinfo

func get(int) (Process, error) { return Process{}, ErrUnsupported }
