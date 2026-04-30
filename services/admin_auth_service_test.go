package services

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	plain := "admin123"
	hash, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		t.Errorf("hash does not match plain password: %v", err)
	}
}

func TestHashPassword_DifferentEachTime(t *testing.T) {
	h1, _ := HashPassword("secret")
	h2, _ := HashPassword("secret")
	if h1 == h2 {
		t.Error("expected different hashes due to salt, got identical hashes")
	}
}
