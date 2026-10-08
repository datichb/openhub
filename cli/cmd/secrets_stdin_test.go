package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// A4: without a terminal, the secret is read from the standard input
// (scripts, CI) instead of failing with "inappropriate ioctl for device".
func TestReadSecretValueFromStdin(t *testing.T) {
	var out bytes.Buffer
	got, err := readSecretValue(strings.NewReader("s3cret\n"), &out, false, "k", "global")
	if err != nil || got != "s3cret" {
		t.Fatalf("readSecretValue = %q, %v", got, err)
	}
	if out.Len() != 0 {
		t.Fatalf("no prompt without terminal, got %q", out.String())
	}
	if _, err := readSecretValue(strings.NewReader("\n"), &out, false, "k", "global"); err == nil {
		t.Fatal("empty value accepted")
	}
}
