package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"course-assistant/internal/application/auth"
	"course-assistant/internal/domain/repository"
)

type AuthHandler struct {
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/signup", h.signUp)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/refresh", h.refresh)
}

type signUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (h *AuthHandler) signUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON.")
		return
	}
	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "email and password are required.")
		return
	}
	if len(req.Password) < 8 {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "password must be at least 8 characters.")
		return
	}

	user, err := h.svc.SignUp(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			WriteError(w, http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists.")
			return
		}
		log.Printf("api: signup error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create account.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(userResponse{ID: user.ID, Email: user.Email})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         userResponse `json:"user"`
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         userResponse{ID: user.ID, Email: user.Email},
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
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

// notFoundOrInternal is shared by project/course handlers: translate
// repository.ErrNotFound into 404, everything else into a generic 500 so a
// raw DB error never leaks to the client.
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
