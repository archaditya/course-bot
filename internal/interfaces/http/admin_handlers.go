package http

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/redis"
)

type AdminHandler struct {
	users repository.UserRepository
	cache *redis.Cache
}

func NewAdminHandler(users repository.UserRepository, cache *redis.Cache) *AdminHandler {
	return &AdminHandler{users: users, cache: cache}
}

func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/stats", h.getStats)
	mux.HandleFunc("GET /admin/users", h.getUsers)
	mux.HandleFunc("PATCH /admin/users/{id}/status", h.updateUserStatus)
	mux.HandleFunc("PATCH /admin/users/{id}/role", h.updateUserRole)
}

func (h *AdminHandler) getStats(w http.ResponseWriter, r *http.Request) {
	if h.cache != nil {
		if cached, ok := h.cache.GetRaw(r.Context(), "cache:admin:stats"); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(cached))
			return
		}
	}

	stats, err := h.users.GetSystemStats(r.Context())
	if err != nil {
		log.Printf("api: admin get stats error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not fetch system stats.")
		return
	}

	jsonBytes, _ := json.Marshal(stats)
	if h.cache != nil && len(jsonBytes) > 0 {
		h.cache.SetRaw(r.Context(), "cache:admin:stats", string(jsonBytes), 30*time.Second)
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) getUsers(w http.ResponseWriter, r *http.Request) {
	if h.cache != nil {
		if cached, ok := h.cache.GetRaw(r.Context(), "cache:admin:users"); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(cached))
			return
		}
	}

	users, err := h.users.ListUsersWithUsageStats(r.Context())
	if err != nil {
		log.Printf("api: admin get users error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not fetch users list.")
		return
	}

	respObj := map[string]any{"items": users}
	jsonBytes, _ := json.Marshal(respObj)
	if h.cache != nil && len(jsonBytes) > 0 {
		h.cache.SetRaw(r.Context(), "cache:admin:users", string(jsonBytes), 30*time.Second)
	}

	writeJSON(w, http.StatusOK, respObj)
}

type updateUserStatusRequest struct {
	IsDisabled bool `json:"is_disabled"`
}

func (h *AdminHandler) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var req updateUserStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON body.")
		return
	}

	if err := h.users.UpdateStatus(r.Context(), userID, req.IsDisabled); err != nil {
		notFoundOrInternal(w, err, "USER_NOT_FOUND", "User not found.")
		return
	}

	if h.cache != nil {
		h.cache.DelRaw(r.Context(), "cache:admin:users")
		h.cache.DelRaw(r.Context(), "cache:admin:stats")
	}

	w.WriteHeader(http.StatusNoContent)
}

type updateUserRoleRequest struct {
	Role string `json:"role"`
}

func (h *AdminHandler) updateUserRole(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var req updateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Role != "user" && req.Role != "admin") {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "role must be 'user' or 'admin'.")
		return
	}

	if err := h.users.UpdateRole(r.Context(), userID, entities.UserRole(req.Role)); err != nil {
		notFoundOrInternal(w, err, "USER_NOT_FOUND", "User not found.")
		return
	}

	if h.cache != nil {
		h.cache.DelRaw(r.Context(), "cache:admin:users")
		h.cache.DelRaw(r.Context(), "cache:admin:stats")
	}

	w.WriteHeader(http.StatusNoContent)
}
