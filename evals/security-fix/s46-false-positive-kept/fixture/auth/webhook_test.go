package auth

import "testing"

func TestValidWebhookToken(t *testing.T) {
	if !ValidWebhookToken("abc", "abc") || ValidWebhookToken("abc", "abd") {
		t.Fatal("ValidWebhookToken")
	}
}
