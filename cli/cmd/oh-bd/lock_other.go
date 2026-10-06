//go:build !unix

package main

// lockFile is a no-op where the fake bd does not run (built for Linux).
func lockFile(string) (func(), error) { return func() {}, nil }
