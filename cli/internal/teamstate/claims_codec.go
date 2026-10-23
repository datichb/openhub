package teamstate

import (
	"regexp"

	toml "github.com/pelletier/go-toml/v2"
)

// Writing of a claim file. Every write of a claim goes through marshalClaim,
// never through toml.Marshal: last_activity must be a native TOML date-time
// (last_activity = 2026-07-08T14:30:00Z), the shape an oh v5.0 reads into its
// time.Time field; a quoted string there makes the whole claim unreadable for
// it (cannot decode TOML string into struct field). go-toml writes a string
// type as a string, and drops a time.Time tagged omitempty, so the field is
// an Instant (a string) in memory and the quotes are taken off on writing.
//
// lease_until and session are keys an older oh does not know and ignores:
// lease_until stays a quoted string.

var lastActivityLine = regexp.MustCompile(`(?m)^last_activity = '([^'\n]*)'$`)

// marshalClaim is the TOML of a claim. A last_activity that is not a date is
// left out (it would be read as no activity anyway, and an older oh could
// not read it).
func marshalClaim(c *Claim) ([]byte, error) {
	cc := *c
	if t := cc.LastActivity.Time(); t.IsZero() {
		cc.LastActivity = ""
	} else {
		cc.LastActivity = InstantOf(t)
	}
	data, err := toml.Marshal(&cc)
	if err != nil {
		return nil, err
	}
	return lastActivityLine.ReplaceAll(data, []byte("last_activity = $1")), nil
}
