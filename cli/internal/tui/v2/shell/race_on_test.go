//go:build race

package shell

// raceEnabled: the race detector slows the event loop by 15×.
const raceEnabled = true
