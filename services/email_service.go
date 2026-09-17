package services

import (
	"arunika_backend/config"
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
)

type OTPEmailData struct {
	OTP string
}

func SendOTPEmail(to string) error {
	tmpl, err := template.ParseFiles("templates/otp_email.html")
	if err != nil {
		return err
	}

	var body bytes.Buffer
	otp := generateOtp()
	err = tmpl.Execute(&body, OTPEmailData{OTP: otp})
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fromAddress())
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Arunika OTP Code")
	m.SetBody("text/html", body.String())

	_ = godotenv.Load()
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		return err
	}
	err = saveOtpToRedis(to, otp)
	if err != nil {
		return err
	}

	d := gomail.NewDialer(os.Getenv("SMTP_HOST"), port, os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))

	// Optional: log instead of send in dev mode
	if err := d.DialAndSend(m); err != nil {
		slog.Error("failed to send OTP email", "error", err, "to", to)
		return err
	}

	return nil
}

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

func generateOtp() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func saveOtpToRedis(email string, otp string) error {
	key := fmt.Sprintf("otp:%s", email)
	expiration := 5 * time.Minute

	err := config.RDB.Set(context.Background(), key, otp, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to save OTP to Redis: %w", err)
	}
	return nil
}
