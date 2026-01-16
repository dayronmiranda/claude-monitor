package handlers

import (
	"encoding/json"
	"net/http"

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
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, analytics)
}

// GetProject GET /api/analytics/projects/{projectPath}
func (h *AnalyticsHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "projectPath")
	if path == "" {
		WriteBadRequest(w, "project path requerido")
		return
	}

	forceRefresh := r.URL.Query().Get("refresh") == "true"

	analytics, err := h.analytics.GetProject(path, forceRefresh)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, analytics)
}

// Invalidate POST /api/analytics/invalidate
func (h *AnalyticsHandler) Invalidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.analytics.Invalidate(req.Project)

	WriteSuccess(w, map[string]string{"message": "Cache invalidado"})
}

// GetCacheStatus GET /api/analytics/cache
func (h *AnalyticsHandler) GetCacheStatus(w http.ResponseWriter, r *http.Request) {
	status := h.analytics.GetCacheStatus()
	WriteSuccess(w, status)
}
