//go:build !linux

package runner

import "errors"

// KillUser is only supported on Linux (remote jobs run on Linux runners).
func KillUser(u *User) error {
	if u == nil {
		return nil
	}
	return errors.New("stopping the processes of the agent account is only supported on Linux")
}
