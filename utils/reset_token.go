package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashResetToken hashes a password-reset token before it is persisted, so a
// database read alone (backup leak, insider access, SQL injection elsewhere)
// never yields a directly usable reset token — only the value emailed to the
// user does. Verification hashes the incoming token the same way and looks
// up by the hash.
func HashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
