package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"claude-monitor/services"
)

// ProjectsHandler maneja endpoints de proyectos
type ProjectsHandler struct {
	claude    *services.ClaudeService
	analytics *services.AnalyticsService
}

// NewProjectsHandler crea un nuevo handler
func NewProjectsHandler(claude *services.ClaudeService, analytics *services.AnalyticsService) *ProjectsHandler {
	return &ProjectsHandler{
		claude:    claude,
		analytics: analytics,
	}
}

// List GET /api/projects
func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.claude.ListProjects()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(projects, &APIMeta{Total: len(projects)}))
}

// Get GET /api/projects/{path}
func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Extraer path del proyecto de la URL
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")

	// Verificar si es una subruta
	if strings.Contains(path, "/") {
		// Es una subruta como /api/projects/{path}/sessions
		return
	}

	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	project, err := h.claude.GetProject(path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
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
		"id":             project.ID,
		"path":           project.Path,
		"real_path":      project.RealPath,
		"session_count":  project.SessionCount,
		"last_modified":  project.LastModified,
		"total_messages": totalMessages,
		"total_size":     totalSize,
		"empty_sessions": emptySessions,
	}

	json.NewEncoder(w).Encode(SuccessResponse(response))
}

// Delete DELETE /api/projects/{path}
func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")

	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	if err := h.claude.DeleteProject(path); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	// Invalidar cache
	h.analytics.Invalidate(path)

	json.NewEncoder(w).Encode(SuccessResponse(map[string]string{
		"message": "Proyecto eliminado",
	}))
}

// GetActivity GET /api/projects/{path}/activity
func (h *ProjectsHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	path := extractProjectPath(r.URL.Path, "/activity")

	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	activity, err := h.claude.GetProjectActivity(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(activity))
}

// extractProjectPath extrae el path del proyecto de una URL
func extractProjectPath(urlPath, suffix string) string {
	path := strings.TrimPrefix(urlPath, "/api/projects/")
	path = strings.TrimSuffix(path, suffix)
	return path
}
