package remote

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
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

// Journal key transport (v5 N21 finitions): the envelope is a generic
// package of oh-runner that every member of the project, and the image
// build jobs (CI_JOB_TOKEN), can read. The journal key travels in it
// encrypted for the target: X25519 key agreement with an ephemeral key, HKDF-
// SHA256, AES-256-GCM bound to the session. The private key of the target
// is a masked, protected CI variable of the run job only (VarJournalKey);
// its public key is a file of the oh-runner repository (JournalPublicKeyFile).

// JournalPublicKeyFile is the public key of the target in the oh-runner
// repository (written by oh remote setup).
const JournalPublicKeyFile = "oh-journal-key.pub"

const journalKeyInfo = "oh-remote-journal-key/1"

var b64 = base64.RawStdEncoding

// NewJournalKeyPair returns an X25519 key pair (base64, no padding: a
// value GitLab can mask).
func NewJournalKeyPair() (public, private string, err error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return b64.EncodeToString(k.PublicKey().Bytes()), b64.EncodeToString(k.Bytes()), nil
}

// JournalPublicKey returns the public key of a private key.
func JournalPublicKey(private string) (string, error) {
	k, err := parsePrivate(private)
	if err != nil {
		return "", err
	}
	return b64.EncodeToString(k.PublicKey().Bytes()), nil
}

// ParseJournalPublicKey reads the content of JournalPublicKeyFile.
func ParseJournalPublicKey(data string) (string, error) {
	pub := strings.TrimSpace(data)
	raw, err := b64.DecodeString(pub)
	if err != nil {
		return "", errors.New("invalid journal public key")
	}
	if _, err := ecdh.X25519().NewPublicKey(raw); err != nil {
		return "", errors.New("invalid journal public key")
	}
	return pub, nil
}

func parsePrivate(private string) (*ecdh.PrivateKey, error) {
	raw, err := b64.DecodeString(strings.TrimSpace(private))
	if err != nil {
		return nil, errors.New("invalid journal private key")
	}
	k, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return nil, errors.New("invalid journal private key")
	}
	return k, nil
}

// ErrJournalKeySealed is returned when the sealed journal key of a session
// cannot be opened with the private key of the job (another key pair, a
// modified envelope).
var ErrJournalKeySealed = errors.New("the journal key of the session cannot be opened with the key of oh-runner")

func journalAEAD(shared, ephemeral, recipient []byte) (cipher.AEAD, error) {
	salt := append(append([]byte{}, ephemeral...), recipient...)
	k, err := hkdf.Key(sha256.New, shared, salt, journalKeyInfo, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// SealJournalKey encrypts the journal key of a session for the public key
// of the target (base64: ephemeral public key, nonce, ciphertext).
func SealJournalKey(public, sessionID, key string) (string, error) {
	praw, err := b64.DecodeString(strings.TrimSpace(public))
	if err != nil {
		return "", errors.New("invalid journal public key")
	}
	pub, err := ecdh.X25519().NewPublicKey(praw)
	if err != nil {
		return "", errors.New("invalid journal public key")
	}
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	shared, err := eph.ECDH(pub)
	if err != nil {
		return "", err
	}
	aead, err := journalAEAD(shared, eph.PublicKey().Bytes(), praw)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := append(append([]byte{}, eph.PublicKey().Bytes()...), nonce...)
	out = aead.Seal(out, nonce, []byte(key), []byte(journalKeyInfo+"\x00"+sessionID))
	return b64.EncodeToString(out), nil
}

// OpenJournalKey decrypts a sealed journal key with the private key of the
// target (in the run job, in memory only).
func OpenJournalKey(private, sessionID, sealed string) (string, error) {
	k, err := parsePrivate(private)
	if err != nil {
		return "", err
	}
	raw, err := b64.DecodeString(sealed)
	if err != nil || len(raw) < 32+12+16 {
		return "", ErrJournalKeySealed
	}
	eph, err := ecdh.X25519().NewPublicKey(raw[:32])
	if err != nil {
		return "", ErrJournalKeySealed
	}
	shared, err := k.ECDH(eph)
	if err != nil {
		return "", ErrJournalKeySealed
	}
	aead, err := journalAEAD(shared, raw[:32], k.PublicKey().Bytes())
	if err != nil {
		return "", err
	}
	n := aead.NonceSize()
	plain, err := aead.Open(nil, raw[32:32+n], raw[32+n:], []byte(journalKeyInfo+"\x00"+sessionID))
	if err != nil {
		return "", ErrJournalKeySealed
	}
	return string(plain), nil
}
