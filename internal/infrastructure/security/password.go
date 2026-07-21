// Package security implements password hashing and JWT signing using only
// the Go standard library. golang.org/x/crypto (bcrypt/scrypt) would be the
// usual choice, but this environment's network egress can't resolve
// golang.org's module-discovery redirect, so we hand-roll PBKDF2-HMAC-SHA256
// (a standard, well-documented construction — RFC 2898) instead of vendoring
// a crypto library blind. Swap this for x/crypto's bcrypt with a one-file
// change the moment golang.org is reachable; nothing outside this package
// knows the difference, since callers only ever see Hash/Verify.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 210_000 // OWASP 2023 minimum recommendation for PBKDF2-HMAC-SHA256
	saltBytes        = 16
	keyBytes         = 32
)

// pbkdf2 derives a key of length `keyLen` from `password` and `salt` using
// HMAC-SHA256 as the PRF, per RFC 2898.
func pbkdf2(password, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	dk := make([]byte, 0, numBlocks*hashLen)
	buf := make([]byte, 4)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf)
		u := prf.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iterations; i++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

// HashPassword returns an encoded string of the form
// "pbkdf2-sha256$<iterations>$<salt-b64>$<hash-b64>" suitable for storing in
// User.PasswordHash. The iteration count is embedded so it can be raised
// later without invalidating existing hashes.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("security: generating salt: %w", err)
	}
	hash := pbkdf2([]byte(password), salt, pbkdf2Iterations, keyBytes)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s",
		pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword reports whether `password` matches the encoded hash
// produced by HashPassword, using a constant-time comparison.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false, fmt.Errorf("security: unrecognized password hash format")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, fmt.Errorf("security: invalid iteration count: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, fmt.Errorf("security: invalid salt encoding: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, fmt.Errorf("security: invalid hash encoding: %w", err)
	}
	got := pbkdf2([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
