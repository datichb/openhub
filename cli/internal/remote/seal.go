package remote

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// JournalKeySecret is the keychain key of the journal key of a session on
// the machine.
func JournalKeySecret(sessionID string) string { return "openhub.remote.journal." + sessionID }

// NewJournalKey returns a fresh journal key (base64).
func NewJournalKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Seal authenticates data of a kind ("journal", "envelope") for a session
// with its journal key: hex HMAC-SHA256.
func Seal(key, kind, sessionID string, data ...[]byte) (string, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(k) < 16 {
		return "", errors.New("invalid journal key")
	}
	m := hmac.New(sha256.New, k)
	m.Write([]byte("oh-remote-seal/1\x00" + kind + "\x00" + sessionID + "\x00"))
	for _, d := range data {
		var n [8]byte
		l := uint64(len(d))
		for i := range n {
			n[i] = byte(l >> (8 * i))
		}
		m.Write(n[:])
		m.Write(d)
	}
	return hex.EncodeToString(m.Sum(nil)), nil
}

// CheckSeal reports whether mac seals data.
func CheckSeal(key, kind, sessionID, mac string, data ...[]byte) bool {
	want, err := Seal(key, kind, sessionID, data...)
	return err == nil && mac != "" && hmac.Equal([]byte(want), []byte(mac))
}
