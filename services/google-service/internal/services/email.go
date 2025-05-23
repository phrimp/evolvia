package services

import (
	"fmt"
	"google-service/internal/config"
	"net/smtp"
	"strings"
)

type EmailService struct {
	config *config.EmailConfig
}

func NewEmailService(_config *config.EmailConfig) *EmailService {
	return &EmailService{config: _config}
}

func (e *EmailService) SendEmail(title, body string, receipient []string) error {
	to := receipient

	message := fmt.Appendf(nil, "To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", strings.Join(to, ","), title, body)

	auth := smtp.PlainAuth("", e.config.SMTPConfig.Username, e.config.SMTPConfig.Password, e.config.SMTPConfig.Host)

	addr := e.config.SMTPConfig.Host + ":" + e.config.SMTPConfig.Port

	err := smtp.SendMail(addr, auth, e.config.SMTPConfig.From, to, message)
	if err != nil {
		fmt.Printf("Error sending email: %s", err)
		return err
	}
	return nil
}
