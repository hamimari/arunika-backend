package services

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
)

// SendGenericEmail sends an HTML email with a custom subject and body.
// Used for campaign dispatch and payment receipt emails.
func SendGenericEmail(to, subject, htmlBody string) error {
	_ = godotenv.Load()
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		return fmt.Errorf("SMTP_PORT not configured: %w", err)
	}
	m := gomail.NewMessage()
	m.SetHeader("From", fromAddress())
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)
	d := gomail.NewDialer(os.Getenv("SMTP_HOST"), port, os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
	if err := d.DialAndSend(m); err != nil {
		slog.Error("SendGenericEmail: failed", "error", err, "to", to)
		return err
	}
	return nil
}

// fromAddress is the sender shown on outgoing email. SMTP_EMAIL lets the
// sending domain be configured without a code change (most SMTP providers
// reject or spoof-flag a From address on a domain they haven't verified for
// this account, so it should usually match SMTP_USER's domain); it's
// optional and falls back to the SMTP account address, then a placeholder.
func fromAddress() string {
	if v := os.Getenv("SMTP_EMAIL"); v != "" {
		return v
	}
	if v := os.Getenv("SMTP_USER"); v != "" {
		return v
	}
	return "arunika.helpdesk@gmail.com"
}

// VerificationEmailData fills templates/verification_email.html.
type VerificationEmailData struct {
	Name       string
	VerifyLink string
}

// SendVerificationEmail emails a one-time verification link.
//
// It goes through SendGenericEmail (gomail, reads SMTP_PASS) rather than the
// legacy utils.SendEmail, which reads an "SMTP_PASSWORD" variable that is set
// nowhere in this codebase — the same trap that once made every
// password-reset email fail silently.
//
// Callers dispatch this asynchronously and treat failure as non-fatal:
// registration must never fail because mail delivery did.
func SendVerificationEmail(to, name, rawToken string) error {
	tmpl, err := template.ParseFiles("templates/verification_email.html")
	if err != nil {
		return err
	}

	link := fmt.Sprintf("%s/auth/verify-email?token=%s", strings.TrimSuffix(os.Getenv("APP_DOMAIN"), "/"), rawToken)

	var body bytes.Buffer
	if err := tmpl.Execute(&body, VerificationEmailData{Name: name, VerifyLink: link}); err != nil {
		return err
	}
	return SendGenericEmail(to, "Verifikasi Email Arunika", body.String())
}
