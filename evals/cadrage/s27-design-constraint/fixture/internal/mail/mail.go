// Package mail envoie des e-mails par SMTP.
package mail

import (
	"fmt"
	"net/smtp"
)

// Client envoie des e-mails par un serveur SMTP.
type Client struct {
	Addr string // hôte:port du serveur SMTP
	From string
}

// Send envoie un e-mail texte à un destinataire.
func (c *Client) Send(to, subject, body string) error {
	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
	return smtp.SendMail(c.Addr, nil, c.From, []string{to}, []byte(msg))
}
