package handlers

import (
	"encoding/json"
	"net/http"

	"claude-monitor/services"
)

// SessionRootsHandler maneja endpoints de session-roots
// Un session-root es un directorio donde se han ejecutado sesiones de Claude
type SessionRootsHandler struct {
	claude    *services.ClaudeService
	analytics *services.AnalyticsService
}

// NewSessionRootsHandler crea un nuevo handler
func NewSessionRootsHandler(claude *services.ClaudeService, analytics *services.AnalyticsService) *SessionRootsHandler {
	return &SessionRootsHandler{
		claude:    claude,
		analytics: analytics,
	}
}

// List GET /api/session-roots
func (h *SessionRootsHandler) List(w http.ResponseWriter, r *http.Request) {
	roots, err := h.claude.ListProjects()
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(roots, &APIMeta{Total: len(roots)}))
}

// Get GET /api/session-roots/{rootPath}
func (h *SessionRootsHandler) Get(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "rootPath")
	if path == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	root, err := h.claude.GetProject(path)
	if err != nil {
		WriteNotFound(w, "session-root")
		return
	}

	// Obtener estadísticas adicionales
	sessions, _ := h.claude.ListSessions(path)

	var totalMessages int
	var totalSize int64
	var emptySessions int

	for _, s := range sessions {
		totalMessages += s.MessageCount
		totalSize += s.SizeBytes
		if s.MessageCount == 0 {
			emptySessions++
		}
	}

	response := map[string]interface{}{
		"id":             root.ID,
		"path":           root.Path,
		"real_path":      root.RealPath,
		"session_count":  root.SessionCount,
		"last_modified":  root.LastModified,
		"total_messages": totalMessages,
		"total_size":     totalSize,
		"empty_sessions": emptySessions,
	}

	WriteSuccess(w, response)
}

// Delete DELETE /api/session-roots/{rootPath}
func (h *SessionRootsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "rootPath")
	if path == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	if err := h.claude.DeleteProject(path); err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	// Invalidar cache
	h.analytics.Invalidate(path)

	WriteSuccess(w, map[string]string{"message": "Session-root eliminado"})
}

// GetActivity GET /api/session-roots/{rootPath}/activity
func (h *SessionRootsHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "rootPath")
	if path == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	activity, err := h.claude.GetProjectActivity(path)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, activity)
}
