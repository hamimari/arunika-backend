package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken hashes an emailed one-time token before it is persisted, so a
// database read alone (backup leak, insider access, SQL injection elsewhere)
// never yields a directly usable token — only the value emailed to the user
// does. Verification hashes the incoming token the same way and looks up by
// the hash.
//
// Used for both password-reset and email-verification tokens; the tokens are
// unguessable UUIDv4 values, so a plain SHA-256 (no salt, no stretching) is
// appropriate — there is no low-entropy secret here to brute-force.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashResetToken is the original name for HashToken, kept so the
// password-reset call sites and their tests read unchanged.
func HashResetToken(token string) string {
	return HashToken(token)
}
