package http

import (
	"context"
	"net/http"
	"strings"

	"archadilm/internal/infrastructure/security"
)

type contextKey string

const claimsContextKey contextKey = "access_claims"

// RequireAuth verifies the Bearer access token on every request to a
// protected route and injects its claims into the request context. This is
// the only place JWT verification happens — handlers never parse tokens
// themselves, per docs/08-security.md#authorization (auth logic testable
// independent of HTTP, and in one place).
func RequireAuth(signingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing or malformed Authorization header.")
				return
			}

			claims, err := security.ParseAccessToken(signingKey, token)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Invalid or expired access token.")
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves the verified access token claims set by
// RequireAuth. Only ever called from within a chain wrapped by RequireAuth;
// callers can safely assume `ok` is true there.
func ClaimsFromContext(ctx context.Context) (*security.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*security.AccessClaims)
	return claims, ok
}
