package teamstate

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // known_hosts names are hashed with HMAC-SHA1 by OpenSSH
	"encoding/base64"
	"fmt"
	"strings"
)

// HashKnownHosts. With `HashKnownHosts yes` (the default of the OpenSSH of
// Debian and Ubuntu) ssh does not write the name of a server in clear in
// known_hosts: it writes |1|<salt>|<hash>, the salt being 20 random bytes and
// the hash the HMAC-SHA1 of the name (host, or [host]:port off the port 22, in
// lower case) under that salt, both in base64, a salt of its own for each
// line. A server oh adds is written the same way when ssh would hash it, and
// the lookup of a server reads both forms (patternsMatch).

// hostSaltLen is the length of the salt, the one of a SHA-1 digest.
const hostSaltLen = sha1.Size

// hostMAC is the HMAC-SHA1 of name under salt.
func hostMAC(salt []byte, name string) []byte {
	mac := hmac.New(sha1.New, salt)
	mac.Write([]byte(name))
	return mac.Sum(nil)
}

// hashedHostName is name as HashKnownHosts writes it under salt.
func hashedHostName(salt []byte, name string) string {
	return "|1|" + base64.StdEncoding.EncodeToString(salt) + "|" + base64.StdEncoding.EncodeToString(hostMAC(salt, name))
}

// newHashedHostName is hashedHostName under a salt drawn at random.
func newHashedHostName(name string) (string, error) {
	salt := make([]byte, hostSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt of the hashed known_hosts entry: %w", err)
	}
	return hashedHostName(salt, name), nil
}

// hostsDataHasKey reports whether the lines of a known_hosts file already hold
// the key k under name (in clear, hashed or behind a pattern).
func hostsDataHasKey(data []byte, name string, k HostKey) bool {
	name = strings.ToLower(name)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "#") || strings.HasPrefix(fields[0], "@") {
			continue
		}
		if fields[1] == k.Type && fields[2] == k.blob && patternsMatch(fields[0], name) {
			return true
		}
	}
	return false
}
