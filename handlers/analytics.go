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

// GetGlobal godoc
// @Summary      Obtener analytics globales
// @Description  Retorna estadísticas globales de todos los proyectos y sesiones
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        refresh  query     bool  false  "Forzar recálculo (default: false)"
// @Success      200      {object}  handlers.APIResponse{data=services.GlobalAnalytics}
// @Failure      500      {object}  handlers.APIResponse
// @Router       /analytics/global [get]
// @Security     BasicAuth
func (h *AnalyticsHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	analytics, err := h.analytics.GetGlobal(forceRefresh)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, analytics)
}

// GetSessionRoot godoc
// @Summary      Obtener analytics de session-root
// @Description  Retorna estadísticas detalladas de un session-root específico
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true   "Path del session-root (URL encoded)"
// @Param        refresh   query     bool    false  "Forzar recálculo (default: false)"
// @Success      200       {object}  handlers.APIResponse{data=services.ProjectAnalytics}
// @Failure      400       {object}  handlers.APIResponse
// @Failure      500       {object}  handlers.APIResponse
// @Router       /analytics/session-roots/{rootPath} [get]
// @Security     BasicAuth
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

// Invalidate godoc
// @Summary      Invalidar cache de analytics
// @Description  Invalida el cache de analytics para un session-root o globalmente
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        request  body      object{session_root=string}  false  "Session-root a invalidar (vacío = global)"
// @Success      200      {object}  handlers.APIResponse
// @Router       /analytics/invalidate [post]
// @Security     BasicAuth
func (h *AnalyticsHandler) Invalidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionRoot string `json:"session_root"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.analytics.Invalidate(req.SessionRoot)

	WriteSuccess(w, map[string]string{"message": "Cache invalidado"})
}

// GetCacheStatus godoc
// @Summary      Obtener estado del cache
// @Description  Retorna el estado actual del cache de analytics
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Success      200  {object}  handlers.APIResponse
// @Router       /analytics/cache [get]
// @Security     BasicAuth
func (h *AnalyticsHandler) GetCacheStatus(w http.ResponseWriter, r *http.Request) {
	status := h.analytics.GetCacheStatus()
	WriteSuccess(w, status)
}
