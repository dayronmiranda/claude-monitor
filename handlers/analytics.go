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

// GetSessionRoot GET /api/analytics/session-roots/{rootPath}
func (h *AnalyticsHandler) GetSessionRoot(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "rootPath")
	if path == "" {
		WriteBadRequest(w, "root path requerido")
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
		SessionRoot string `json:"session_root"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.analytics.Invalidate(req.SessionRoot)

	WriteSuccess(w, map[string]string{"message": "Cache invalidado"})
}

// GetCacheStatus GET /api/analytics/cache
func (h *AnalyticsHandler) GetCacheStatus(w http.ResponseWriter, r *http.Request) {
	status := h.analytics.GetCacheStatus()
	WriteSuccess(w, status)
}
