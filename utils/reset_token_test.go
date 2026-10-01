package utils

import "testing"

func TestHashResetToken_Deterministic(t *testing.T) {
	a := HashResetToken("same-token")
	b := HashResetToken("same-token")
	if a != b {
		t.Fatalf("expected the same input to hash the same way, got %q and %q", a, b)
	}
}

func TestHashResetToken_DifferentInputsDifferentHashes(t *testing.T) {
	a := HashResetToken("token-a")
	b := HashResetToken("token-b")
	if a == b {
		t.Fatalf("expected different inputs to hash differently, both were %q", a)
	}
}

func TestHashResetToken_NeverEqualsItsInput(t *testing.T) {
	token := "raw-reset-token"
	if HashResetToken(token) == token {
		t.Fatal("hash must not equal the raw token")
	}
}

func TestHashResetToken_IsHexSHA256Length(t *testing.T) {
	hash := HashResetToken("anything")
	if len(hash) != 64 {
		t.Fatalf("expected a 64-character hex SHA-256 digest, got length %d: %q", len(hash), hash)
	}
	for _, r := range hash {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			t.Fatalf("expected only lowercase hex characters, found %q in %q", r, hash)
		}
	}
}
