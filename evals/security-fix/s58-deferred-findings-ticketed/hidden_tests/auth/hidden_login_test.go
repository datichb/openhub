package auth

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestHiddenLoginDoesNotLogPassword(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)
	_ = Login("alice", "correct-horse")
	_ = Login("bob", "s3cret-bob")
	if out := buf.String(); strings.Contains(out, "correct-horse") || strings.Contains(out, "s3cret-bob") {
		t.Fatalf("password in the logs: %q", out)
	}
}
