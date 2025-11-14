package services

import (
	"arunika_backend/config"
	"bytes"
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
	"html/template"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
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
	m.SetHeader("From", "no-reply@arunika.com")
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
		log.Println("Error sending OTP email:", err)
		return err
	}

	return nil
}

func generateOtp() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func saveOtpToRedis(email string, otp string) error {
	key := fmt.Sprintf("otp:%s", email) // You can also use user ID or phone number
	expiration := 5 * time.Minute

	err := config.RDB.Set(context.Background(), key, otp, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to save OTP to Redis: %w", err)
	}
	return nil
}
