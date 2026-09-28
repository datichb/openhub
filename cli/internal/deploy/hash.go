package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// FileHash returns the SHA-256 hex digest of a file using streaming I/O
// to avoid loading the entire file into memory.
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BytesHash returns the SHA-256 hex digest of a byte slice.
func BytesHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
