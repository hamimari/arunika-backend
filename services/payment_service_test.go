package services

import (
	"crypto/sha512"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func computeSignature(orderID, statusCode, grossAmount, serverKey string) string {
	h := sha512.New()
	h.Write([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(h.Sum(nil))
}

// ─── ValidateWebhookSignature ─────────────────────────────────────────────────

func TestValidateWebhookSignature_Valid(t *testing.T) {
	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	sig := computeSignature("ORDER-001", "200", "49000.00", "testkey")
	assert.True(t, ValidateWebhookSignature("ORDER-001", "200", "49000.00", sig))
}

func TestValidateWebhookSignature_Invalid(t *testing.T) {
	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")
	assert.False(t, ValidateWebhookSignature("ORDER-001", "200", "49000.00", "wrongsig"))
}

func TestValidateWebhookSignature_WrongKey(t *testing.T) {
	os.Setenv("MIDTRANS_SERVER_KEY", "correctkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	// Signature computed with a different key — should fail.
	sig := computeSignature("ORDER-001", "200", "49000.00", "wrongkey")
	assert.False(t, ValidateWebhookSignature("ORDER-001", "200", "49000.00", sig))
}
