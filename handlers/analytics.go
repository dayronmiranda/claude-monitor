package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"claude-monitor/services"
)

// AnalyticsHandler maneja endpoints de analytics
type AnalyticsHandler struct {
	analytics *services.AnalyticsService
}

// NewAnalyticsHandler crea un nuevo handler
func NewAnalyticsHandler(analytics *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

// GetGlobal GET /api/analytics/global
func (h *AnalyticsHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	analytics, err := h.analytics.GetGlobal(forceRefresh)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(analytics))
}

// GetProject GET /api/analytics/projects/{path}
func (h *AnalyticsHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/analytics/projects/")

	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	forceRefresh := r.URL.Query().Get("refresh") == "true"

	analytics, err := h.analytics.GetProject(path, forceRefresh)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(analytics))
}

// Invalidate POST /api/analytics/invalidate
func (h *AnalyticsHandler) Invalidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.analytics.Invalidate(req.Project)

	json.NewEncoder(w).Encode(SuccessResponse(map[string]string{
		"message": "Cache invalidado",
	}))
}

// GetCacheStatus GET /api/analytics/cache
func (h *AnalyticsHandler) GetCacheStatus(w http.ResponseWriter, r *http.Request) {
	status := h.analytics.GetCacheStatus()
	json.NewEncoder(w).Encode(SuccessResponse(status))
}
