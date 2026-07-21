// Package http holds HTTP handlers (the "interfaces" layer). Handlers do
// request/response marshaling and call into application/ use cases; they
// must never contain business logic or authorization decisions themselves
// — those live in application/ so they're testable independent of HTTP, per
// docs/08-security.md#authorization.
package http

import (
	"encoding/json"
	"net/http"
)

const apiVersion = "0.1.0"

// healthzResponse matches docs/10-api-contracts.md#status--health exactly.
type healthzResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

// errorResponse matches the single error shape from
// docs/10-api-contracts.md#conventions.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes the standard error envelope. Kept here (not duplicated
// per-handler) so every endpoint's error shape stays byte-for-byte
// consistent with docs/10-api-contracts.md#conventions.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: errorBody{Code: code, Message: message}})
}

// Dependencies bundles everything the router needs to wire up route groups.
// Constructed once in cmd/api/main.go and passed in here so router.go stays
// the single place that knows how the HTTP surface maps to handlers.
type Dependencies struct {
	JWTSigningKey   string
	AuthHandler     *AuthHandler
	ProjectHandler  *ProjectHandler
	CourseHandler   *CourseHandler
	UploadHandler   *UploadHandler
	ChatHandler     *ChatHandler
}

// NewRouter wires up the Go API's HTTP surface — see
// docs/10-api-contracts.md for the full contract this maps to. Public
// routes (health, auth) are unprotected; everything else requires a valid
// access token via RequireAuth.
func NewRouter(deps Dependencies) http.Handler {
	public := http.NewServeMux()
	public.HandleFunc("GET /healthz", handleHealthz)
	deps.AuthHandler.Register(public)

	protected := http.NewServeMux()
	deps.ProjectHandler.Register(protected)
	deps.CourseHandler.Register(protected)
	deps.UploadHandler.Register(protected)
	deps.ChatHandler.Register(protected)

	top := http.NewServeMux()
	top.Handle("/", public)

	// Protected route prefixes
	auth := RequireAuth(deps.JWTSigningKey)
	top.Handle("/projects", auth(protected))
	top.Handle("/projects/", auth(protected))
	top.Handle("/courses/", auth(protected))
	top.Handle("/conversations", auth(protected))
	top.Handle("/conversations/", auth(protected))

	return top
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthzResponse{
		Status:  "ok",
		Service: "api",
		Version: apiVersion,
	})
}
