package token

import (
	"crypto/rand"
	"encoding/hex"
)

// NewRefreshToken generates a cryptographically random 64-character hex token.
func NewRefreshToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
