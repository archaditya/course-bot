package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"archadilm/internal/application/auth"
	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type AuthHandler struct{ svc *auth.Service }

func NewAuthHandler(svc *auth.Service) *AuthHandler { return &AuthHandler{svc: svc} }
func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/signup", h.signUp)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/refresh", h.refresh)
}

type signUpRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}
type userResponse struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}
type tokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         userResponse `json:"user"`
}

func toUserResponse(user *entities.User) userResponse {
	return userResponse{ID: user.ID, FullName: user.FullName, Email: user.Email}
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func (h *AuthHandler) signUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON.")
		return
	}
	if strings.TrimSpace(req.FullName) == "" || strings.TrimSpace(req.Email) == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "full_name, email and password are required.")
		return
	}
	if len(req.Password) < 8 {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "password must be at least 8 characters.")
		return
	}
	user, err := h.svc.SignUp(r.Context(), req.FullName, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			WriteError(w, http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists.")
			return
		}
		log.Printf("api: signup error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create account.")
		return
	}
	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON.")
		return
	}
	user, tokens, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Incorrect email or password.")
			return
		}
		log.Printf("api: login error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not log in.")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, User: toUserResponse(user)})
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil || req.RefreshToken == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "refresh_token is required.")
		return
	}
	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidRefresh) {
			WriteError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token is invalid or expired.")
			return
		}
		log.Printf("api: refresh error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not refresh session.")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}{tokens.AccessToken, tokens.RefreshToken})
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	user, err := h.svc.Profile(r.Context(), claims.UserID)
	if err != nil {
		notFoundOrInternal(w, err, "USER_NOT_FOUND", "User not found.")
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func notFoundOrInternal(w http.ResponseWriter, err error, notFoundCode, notFoundMessage string) {
	if errors.Is(err, repository.ErrNotFound) {
		WriteError(w, http.StatusNotFound, notFoundCode, notFoundMessage)
		return
	}
	log.Printf("api: internal error: %v", err)
	WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
