package utils

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateJWT_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars!!")
	defer os.Unsetenv("JWT_SECRET")

	tokenString, refreshToken, err := GenerateJWT("user-123", "test@example.com")

	require.NoError(t, err)
	assert.NotEmpty(t, tokenString, "access token should not be empty")
	assert.NotEmpty(t, refreshToken, "refresh token should not be empty")
}

func TestGenerateJWT_MissingSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	_, _, err := GenerateJWT("user-123", "test@example.com")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestGenerateJWT_TokenClaims(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars!!"
	os.Setenv("JWT_SECRET", secret)
	defer os.Unsetenv("JWT_SECRET")

	userID := "user-abc-123"
	email := "user@example.com"

	tokenString, _, err := GenerateJWT(userID, email)
	require.NoError(t, err)

	// Parse the token and verify its claims
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	claims, ok := token.Claims.(jwt.MapClaims)
	require.True(t, ok)

	assert.Equal(t, userID, claims["sub"])
	assert.Equal(t, email, claims["email"])
	assert.NotEmpty(t, claims["jti"])
	assert.NotEmpty(t, claims["refresh_token"])

	// Token should expire in ~15 minutes
	exp := int64(claims["exp"].(float64))
	assert.True(t, exp > time.Now().Unix(), "token should not be already expired")
	assert.True(t, exp <= time.Now().Add(16*time.Minute).Unix(), "token expiry should be within 16 minutes")
}

func TestGenerateJWT_UniqueRefreshTokens(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars!!")
	defer os.Unsetenv("JWT_SECRET")

	_, rt1, _ := GenerateJWT("user-1", "a@b.com")
	_, rt2, _ := GenerateJWT("user-1", "a@b.com")

	assert.NotEqual(t, rt1, rt2, "each call should produce a unique refresh token")
}

func TestGenerateJWT_UniqueJTI(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars!!"
	os.Setenv("JWT_SECRET", secret)
	defer os.Unsetenv("JWT_SECRET")

	tok1, _, _ := GenerateJWT("u", "e@e.com")
	tok2, _, _ := GenerateJWT("u", "e@e.com")

	parseJTI := func(ts string) string {
		token, _ := jwt.Parse(ts, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		claims := token.Claims.(jwt.MapClaims)
		return claims["jti"].(string)
	}

	assert.NotEqual(t, parseJTI(tok1), parseJTI(tok2), "each token must have a unique jti")
}

func TestGenerateJWT_InvalidSignature(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars!!")
	defer os.Unsetenv("JWT_SECRET")

	tokenString, _, err := GenerateJWT("user-123", "test@example.com")
	require.NoError(t, err)

	// Attempt to parse with a different secret — should fail
	_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("wrong-secret-key-at-least-32-chars!!"), nil
	})
	assert.Error(t, err, "parsing with wrong secret should return an error")
}

func TestGenerateJWT_ExpiredTokenRejected(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars!!"

	// Build a token that expired 1 hour ago
	claims := jwt.MapClaims{
		"sub":   "user-123",
		"email": "test@example.com",
		"jti":   "some-jti",
		"exp":   time.Now().Add(-1 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)

	// Parsing an expired token should return an error
	_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.Error(t, err, "expired token should be rejected")
}

func TestGenerateJWT_MissingClaimsDetected(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars!!"

	// Build a minimal token without exp or jti
	claims := jwt.MapClaims{
		"sub":   "user-123",
		"email": "test@example.com",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)

	// Token without exp is still parseable by default (jwt lib doesn't require exp unless asked)
	parsed, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err)

	parsedClaims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)

	// Verify that jti and exp are indeed absent
	_, hasExp := parsedClaims["exp"]
	_, hasJTI := parsedClaims["jti"]
	assert.False(t, hasExp, "exp claim should be missing from this token")
	assert.False(t, hasJTI, "jti claim should be missing from this token")
}
