package handlers

import (
	"net/http"

	"github.com/cirvee/referral-backend/internal/cache"
	"github.com/cirvee/referral-backend/internal/database"
)

type HealthHandler struct {
	db    *database.DB
	cache *cache.Cache
}

func NewHealthHandler(db *database.DB, cache *cache.Cache) *HealthHandler {
	return &HealthHandler{db: db, cache: cache}
}

// Health godoc
// @Summary Health check
// @Description Check if the service is healthy
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := map[string]string{
		"status":   "healthy",
		"database": "ok",
		"cache":    "ok",
	}

	if err := h.db.Health(ctx); err != nil {
		response["status"] = "unhealthy"
		response["database"] = "error"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := h.cache.Health(ctx); err != nil {
		response["status"] = "unhealthy"
		response["cache"] = "error"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	respondJSON(w, http.StatusOK, response)
}
