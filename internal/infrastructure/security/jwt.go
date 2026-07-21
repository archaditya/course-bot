package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AccessClaims is the payload of an access token — see
// docs/08-security.md#jwt-rotation. Kept minimal on purpose: the token
// carries identity only, never authorization data, since MVP authorization
// is a single implicit "owner" role resolved server-side per
// docs/08-security.md#authorization.
type AccessClaims struct {
	UserID      string `json:"sub"`
	WorkspaceID string `json:"workspace_id"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// SignAccessToken issues a short-lived HS256 JWT per
// docs/08-security.md#jwt-rotation. `ttl` is expected to be minutes, not
// hours — this is the access token, not the refresh token.
func SignAccessToken(signingKey string, userID, workspaceID string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		IssuedAt:    now.Unix(),
		ExpiresAt:   now.Add(ttl).Unix(),
	}
	return signJWT(signingKey, claims)
}

func signJWT(signingKey string, claims AccessClaims) (string, error) {
	header := jwtHeader{Alg: "HS256", Typ: "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64(headerJSON) + "." + b64(claimsJSON)
	sig := sign(signingKey, signingInput)
	return signingInput + "." + sig, nil
}

// ParseAccessToken verifies the signature and expiry of an access token and
// returns its claims. Callers (the HTTP auth middleware) reject the request
// on any error rather than trying to recover a partial identity.
func ParseAccessToken(signingKey string, token string) (*AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("security: malformed token")
	}
	signingInput := parts[0] + "." + parts[1]
	expectedSig := sign(signingKey, signingInput)
	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(parts[2])) != 1 {
		return nil, fmt.Errorf("security: invalid token signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("security: invalid token claims encoding: %w", err)
	}
	var claims AccessClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("security: invalid token claims: %w", err)
	}
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("security: token expired")
	}
	return &claims, nil
}

func sign(signingKey, signingInput string) string {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func b64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
