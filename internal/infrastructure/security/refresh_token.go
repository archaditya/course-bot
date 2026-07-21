package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateRefreshToken returns a random opaque token (given to the client)
// and its SHA-256 hash (what gets stored in Postgres). Refresh tokens are
// never JWTs and never stored in plaintext — see
// docs/08-security.md#jwt-rotation. A plain SHA-256 (not PBKDF2) is
// appropriate here because the token itself already has 256 bits of
// randomness; unlike a password, there's no low-entropy input to slow-hash
// against.
func GenerateRefreshToken() (token string, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("security: generating refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken hashes a client-presented refresh token for lookup
// against the stored hash. Deterministic (unlike password hashing) since a
// refresh token must be looked up by exact value, not verified against a
// per-record salt.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
