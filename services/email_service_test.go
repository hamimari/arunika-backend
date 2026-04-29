package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// generateOtp is an unexported helper; we test it indirectly via its output.
func TestGenerateOtp_Format(t *testing.T) {
	for i := 0; i < 20; i++ {
		otp := generateOtp()
		assert.Len(t, otp, 6, "OTP must be exactly 6 digits")
		for _, ch := range otp {
			assert.True(t, ch >= '0' && ch <= '9', "OTP must contain only digits, got %c", ch)
		}
	}
}

func TestGenerateOtp_Randomness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		otp := generateOtp()
		seen[otp] = true
	}
	// With 50 tries over 1,000,000 possible values, the chance of all being
	// the same is negligible — expect at least 2 distinct values.
	assert.Greater(t, len(seen), 1, "generateOtp should produce varying values")
}
