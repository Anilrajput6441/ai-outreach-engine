package mailer

import (
	"fmt"
	"net/smtp"
)

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

func SendEmail(cfg Config, to, subject, body string) error {
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	msj := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		cfg.Username, to, subject, body)

	return smtp.SendMail(
		cfg.Host+":"+cfg.Port,
		auth,
		cfg.Username,
		[]string{to},
		[]byte(msj),
	)
}
