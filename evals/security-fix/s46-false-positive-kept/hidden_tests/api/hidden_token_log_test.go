package api

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestHiddenTokenNotLogged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)
	req, err := Client{BaseURL: "https://partner.example", Token: "tok-hidden-42"}.NewRequest("/orders")
	if err != nil || req.Header.Get("Authorization") != "Bearer tok-hidden-42" {
		t.Fatalf("req = %v, err = %v", req, err)
	}
	if strings.Contains(buf.String(), "tok-hidden-42") {
		t.Fatalf("token in the logs: %q", buf.String())
	}
}
