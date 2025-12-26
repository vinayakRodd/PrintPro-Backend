package services

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"print-pro-backend/internal/config"
)

// EmailService handles sending emails
type EmailService struct {
	config *config.Config
}

// NewEmailService creates a new email service
func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		config: cfg,
	}
}

// SendOTPEmail sends an OTP code to the user's email
func (s *EmailService) SendOTPEmail(to, otp string) error {
	log.Printf("SendOTPEmail called - preparing email")
	subject := "Password Reset OTP"
	body := fmt.Sprintf(`
Hello,

You have requested to reset your password. Please use the following OTP code:

%s

This OTP will expire in 5 minutes.

If you did not request this password reset, please ignore this email.

Best regards,
Print Pro Team
`, otp)

	log.Printf("Calling sendEmail function")
	err := s.sendEmail(to, subject, body)
	if err != nil {
		log.Printf("sendEmail returned error: %v", err)
		return err
	}
	log.Printf("SendOTPEmail completed successfully")
	return nil
}

// sendEmail sends an email using SMTP
func (s *EmailService) sendEmail(to, subject, body string) error {
	log.Printf("Preparing to send email")
	log.Printf("SMTP Config Check - Host: %s, Port: %s, Username set: %v, Password set: %v", 
		s.config.SMTPHost, s.config.SMTPPort, s.config.SMTPUsername != "", s.config.SMTPPassword != "")
	
	// Validate configuration
	if s.config.SMTPUsername == "" || s.config.SMTPPassword == "" {
		log.Printf("SMTP credentials not configured - Username: '%s', Password: '%s'", s.config.SMTPUsername, s.config.SMTPPassword)
		return fmt.Errorf("SMTP credentials not configured")
	}

	// Use FROM_EMAIL if provided, otherwise use SMTP_USERNAME
	from := s.config.FromEmail
	if from == "" {
		from = s.config.SMTPUsername
	}
	
	// Validate from email is set
	if from == "" {
		log.Printf("FROM_EMAIL or SMTP_USERNAME must be configured")
		return fmt.Errorf("FROM_EMAIL or SMTP_USERNAME must be configured")
	}

	log.Printf("Setting up SMTP authentication")
	// Setup authentication
	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)

	// Email message with proper formatting
	// Format: "Display Name <email@example.com>" or just "email@example.com"
	fromHeader := fmt.Sprintf("Print Pro <%s>", from)
	
	msg := []byte(fmt.Sprintf("To: %s\r\n", to) +
		fmt.Sprintf("From: %s\r\n", fromHeader) +
		fmt.Sprintf("Subject: %s\r\n", subject) +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"MIME-Version: 1.0\r\n" +
		"\r\n" +
		body + "\r\n")

	log.Printf("Connecting to SMTP server: %s:%s", s.config.SMTPHost, s.config.SMTPPort)
	log.Printf("SMTP Username: %s", s.config.SMTPUsername)
	log.Printf("From email: %s", from)
	log.Printf("To email: %s", to)
	
	// Send email with explicit STARTTLS for Gmail (port 587)
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)
	log.Printf("Attempting to send email via SMTP at %s", addr)
	
	// Gmail requires STARTTLS on port 587 - use explicit connection
	if s.config.SMTPPort == "587" {
		log.Printf("Using STARTTLS for port 587")
		// Connect to server
		client, err := smtp.Dial(addr)
		if err != nil {
			log.Printf("Failed to dial SMTP server: %v", err)
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer client.Close()
		
		// Send EHLO
		if err = client.Hello("localhost"); err != nil {
			log.Printf("Failed to send EHLO: %v", err)
			return fmt.Errorf("failed to send EHLO: %w", err)
		}
		
		// Start TLS
		tlsConfig := &tls.Config{
			ServerName:         s.config.SMTPHost,
			InsecureSkipVerify: false,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			log.Printf("Failed to start TLS: %v", err)
			return fmt.Errorf("failed to start TLS: %w", err)
		}
		log.Printf("TLS connection established")
		
		// Authenticate
		if err = client.Auth(auth); err != nil {
			log.Printf("SMTP authentication failed: %v", err)
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
		log.Printf("SMTP authentication successful")
		
		// Set sender
		if err = client.Mail(from); err != nil {
			log.Printf("Failed to set sender: %v", err)
			return fmt.Errorf("failed to set sender: %w", err)
		}
		
		// Set recipient
		if err = client.Rcpt(to); err != nil {
			log.Printf("Failed to set recipient: %v", err)
			return fmt.Errorf("failed to set recipient: %w", err)
		}
		
		// Send email body
		writer, err := client.Data()
		if err != nil {
			log.Printf("Failed to get data writer: %v", err)
			return fmt.Errorf("failed to get data writer: %w", err)
		}
		
		_, err = writer.Write(msg)
		if err != nil {
			writer.Close()
			log.Printf("Failed to write email data: %v", err)
			return fmt.Errorf("failed to write email data: %w", err)
		}
		
		err = writer.Close()
		if err != nil {
			log.Printf("Failed to close data writer: %v", err)
			return fmt.Errorf("failed to close data writer: %w", err)
		}
		
		log.Printf("Email sent successfully via SMTP with STARTTLS")
		return nil
	} else {
		// For other ports (like 465 with SSL), use standard SendMail
		log.Printf("Using standard SMTP SendMail")
		err := smtp.SendMail(addr, auth, from, []string{to}, msg)
		if err != nil {
			log.Printf("SMTP SendMail failed with error: %v", err)
			log.Printf("Error details - Type: %T, Message: %s", err, err.Error())
			return fmt.Errorf("failed to send email: %w", err)
		}
		log.Printf("Email sent successfully via SMTP")
		return nil
	}
}

