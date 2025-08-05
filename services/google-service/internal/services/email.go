// services/google-service/internal/services/email.go
package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"google-service/internal/config"
	"html/template"
	"log"
	"net"
	"net/smtp"
	"strings"
)

type EmailService struct {
	config *config.EmailConfig
}

type EmailTemplate struct {
	Subject string
	Body    string
	IsHTML  bool
}

type EmailData struct {
	Name       string
	Email      string
	OTPCode    string
	ExpiryTime string
	VerifyURL  string
}

func NewEmailService(_config *config.EmailConfig) *EmailService {
	return &EmailService{config: _config}
}

// SendEmail sends a basic email
func (e *EmailService) SendEmail(title, body string, recipients []string) error {
	if !e.config.Enabled {
		log.Println("Email service is disabled, skipping email send")
		return nil
	}

	return e.sendEmailWithAuth(title, body, recipients, false)
}

// SendEmailWithTemplate sends an email using a template
func (e *EmailService) SendEmailWithTemplate(templateName string, data EmailData, recipients []string) error {
	if !e.config.Enabled {
		log.Printf("Email service is disabled, would send %s template to %s", templateName, recipients[0])
		return nil
	}

	emailTemplate := e.getEmailTemplate(templateName)
	if emailTemplate == nil {
		return fmt.Errorf("template %s not found", templateName)
	}

	// Parse and execute subject template
	subjectTmpl, err := template.New("subject").Parse(emailTemplate.Subject)
	if err != nil {
		return fmt.Errorf("error parsing subject template: %w", err)
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		return fmt.Errorf("error executing subject template: %w", err)
	}

	// Parse and execute body template
	bodyTmpl, err := template.New("body").Parse(emailTemplate.Body)
	if err != nil {
		return fmt.Errorf("error parsing body template: %w", err)
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		return fmt.Errorf("error executing body template: %w", err)
	}

	return e.sendEmailWithAuth(subjectBuf.String(), bodyBuf.String(), recipients, emailTemplate.IsHTML)
}

// sendEmailWithAuth handles the actual email sending with SMTP authentication
func (e *EmailService) sendEmailWithAuth(subject, body string, recipients []string, isHTML bool) error {
	// Build message first
	headers := make(map[string]string)
	headers["From"] = e.config.SMTPConfig.From
	headers["To"] = strings.Join(recipients, ",")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"

	if isHTML {
		headers["Content-Type"] = "text/html; charset=UTF-8"
	} else {
		headers["Content-Type"] = "text/plain; charset=UTF-8"
	}

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// For Gmail (port 587), use STARTTLS
	if e.config.SMTPConfig.Port == "587" {
		return e.sendEmailWithSTARTTLS(subject, body, recipients, isHTML)
	}

	// For other configurations, use the original method
	auth := smtp.PlainAuth("", e.config.SMTPConfig.Username, e.config.SMTPConfig.Password, e.config.SMTPConfig.Host)
	addr := e.config.SMTPConfig.Host + ":" + e.config.SMTPConfig.Port

	if e.config.SMTPConfig.UseTLS {
		return e.sendEmailWithDirectTLS(addr, auth, e.config.SMTPConfig.From, recipients, []byte(message))
	}

	err := smtp.SendMail(addr, auth, e.config.SMTPConfig.From, recipients, []byte(message))
	if err != nil {
		log.Printf("Error sending email: %s", err)
		return err
	}

	log.Printf("Email sent successfully to %s", strings.Join(recipients, ", "))
	return nil
}

// sendEmailWithSTARTTLS sends email using STARTTLS (Gmail port 587)
func (e *EmailService) sendEmailWithSTARTTLS(subject, body string, recipients []string, isHTML bool) error {
	// Build message
	headers := make(map[string]string)
	headers["From"] = e.config.SMTPConfig.From
	headers["To"] = strings.Join(recipients, ",")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"

	if isHTML {
		headers["Content-Type"] = "text/html; charset=UTF-8"
	} else {
		headers["Content-Type"] = "text/plain; charset=UTF-8"
	}

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Server address
	serverAddr := e.config.SMTPConfig.Host + ":" + e.config.SMTPConfig.Port

	log.Printf("Connecting to SMTP server: %s", serverAddr)

	// Step 1: Connect to the server (plain connection)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Step 2: Create SMTP client
	client, err := smtp.NewClient(conn, e.config.SMTPConfig.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Step 3: Start TLS if supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		log.Printf("Starting STARTTLS...")
		tlsConfig := &tls.Config{
			ServerName:         e.config.SMTPConfig.Host,
			InsecureSkipVerify: false,
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
		log.Printf("STARTTLS successful")
	}

	// Step 4: Authenticate
	auth := smtp.PlainAuth("", e.config.SMTPConfig.Username, e.config.SMTPConfig.Password, e.config.SMTPConfig.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}
	log.Printf("SMTP authentication successful")

	// Step 5: Set sender
	if err := client.Mail(e.config.SMTPConfig.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Step 6: Set recipients
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Step 7: Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Step 8: Quit
	err = client.Quit()
	if err != nil {
		log.Printf("Warning: error during QUIT: %v", err)
	}

	log.Printf("Email sent successfully to %s via STARTTLS", strings.Join(recipients, ", "))
	return nil
}

// sendEmailWithDirectTLS sends email with direct TLS connection (port 465)
func (e *EmailService) sendEmailWithDirectTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// Create TLS connection
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         e.config.SMTPConfig.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create TLS connection: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.config.SMTPConfig.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// getEmailTemplate returns the email template for the given template name
func (e *EmailService) getEmailTemplate(templateName string) *EmailTemplate {
	templates := map[string]*EmailTemplate{
		"email_verification": {
			Subject: "Verify Your Email Address - {{.Name}}",
			Body: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Email Verification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4285f4; color: white; padding: 20px; text-align: center; }
        .content { padding: 30px 20px; background-color: #f9f9f9; }
        .otp-code { background-color: #e3f2fd; padding: 15px; text-align: center; font-size: 24px; font-weight: bold; letter-spacing: 3px; margin: 20px 0; border-radius: 5px; }
        .button { display: inline-block; background-color: #4285f4; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; padding: 20px; font-size: 12px; color: #666; }
        .expiry { color: #ff5722; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Email Verification</h1>
        </div>
        <div class="content">
            <h2>Hello {{.Name}},</h2>
            <p>Thank you for registering with us! To complete your email verification, please use the OTP code below:</p>
            
            <div class="otp-code">{{.OTPCode}}</div>
            
            <p>Alternatively, you can click the button below to verify your email:</p>
            <a href="{{.VerifyURL}}" class="button">Verify Email</a>
            
            <p class="expiry">This OTP will expire in {{.ExpiryTime}}.</p>
            
            <p>If you didn't request this verification, please ignore this email.</p>
            
            <p>Best regards,<br>The Evolvia Team</p>
        </div>
        <div class="footer">
            <p>This is an automated message. Please do not reply to this email.</p>
        </div>
    </div>
</body>
</html>`,
			IsHTML: true,
		},
		"welcome": {
			Subject: "Welcome to Evolvia - {{.Name}}",
			Body: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Welcome to Evolvia</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4caf50; color: white; padding: 20px; text-align: center; }
        .content { padding: 30px 20px; background-color: #f9f9f9; }
        .button { display: inline-block; background-color: #4caf50; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; padding: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to Evolvia!</h1>
        </div>
        <div class="content">
            <h2>Hello {{.Name}},</h2>
            <p>Congratulations! Your email has been successfully verified and your account is now active.</p>
            
            <p>You can now access all features of our platform:</p>
            <ul>
                <li>Complete learning assessments</li>
                <li>Track your progress</li>
                <li>Access personalized recommendations</li>
                <li>Connect with the community</li>
            </ul>
            
            <a href="{{.VerifyURL}}" class="button">Get Started</a>
            
            <p>If you have any questions, feel free to contact our support team.</p>
            
            <p>Best regards,<br>The Evolvia Team</p>
        </div>
        <div class="footer">
            <p>This is an automated message. Please do not reply to this email.</p>
        </div>
    </div>
</body>
</html>`,
			IsHTML: true,
		},
	}

	return templates[templateName]
}
