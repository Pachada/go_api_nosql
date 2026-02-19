package token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewRefreshToken generates a cryptographically random 64-character hex token.
func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
