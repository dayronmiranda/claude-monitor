package handlers

import (
	"encoding/json"
	"net/http"

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
		WriteInternalError(w, err.Error())
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(projects, &APIMeta{Total: len(projects)}))
}

// Get GET /api/projects/{projectPath}
func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "projectPath")
	if path == "" {
		WriteBadRequest(w, "project path requerido")
		return
	}

	project, err := h.claude.GetProject(path)
	if err != nil {
		WriteNotFound(w, "proyecto")
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

	WriteSuccess(w, response)
}

// Delete DELETE /api/projects/{projectPath}
func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "projectPath")
	if path == "" {
		WriteBadRequest(w, "project path requerido")
		return
	}

	if err := h.claude.DeleteProject(path); err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	// Invalidar cache
	h.analytics.Invalidate(path)

	WriteSuccess(w, map[string]string{"message": "Proyecto eliminado"})
}

// GetActivity GET /api/projects/{projectPath}/activity
func (h *ProjectsHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "projectPath")
	if path == "" {
		WriteBadRequest(w, "project path requerido")
		return
	}

	activity, err := h.claude.GetProjectActivity(path)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, activity)
}
