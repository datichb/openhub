package mail

import "testing"

func TestSendUnreachable(t *testing.T) {
	c := &Client{Addr: "127.0.0.1:1", From: "shop@example.com"}
	if err := c.Send("ana@example.com", "test", "corps"); err == nil {
		t.Fatal("erreur attendue sans serveur SMTP")
	}
}
