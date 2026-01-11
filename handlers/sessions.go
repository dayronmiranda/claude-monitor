package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"claude-monitor/services"
)

// SessionsHandler maneja endpoints de sesiones Claude
type SessionsHandler struct {
	claude    *services.ClaudeService
	terminals *services.TerminalService
	analytics *services.AnalyticsService
}

// NewSessionsHandler crea un nuevo handler
func NewSessionsHandler(claude *services.ClaudeService, terminals *services.TerminalService, analytics *services.AnalyticsService) *SessionsHandler {
	return &SessionsHandler{
		claude:    claude,
		terminals: terminals,
		analytics: analytics,
	}
}

// List GET /api/projects/{path}/sessions
func (h *SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	path := extractSessionsProjectPath(r.URL.Path)

	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	sessions, err := h.claude.ListSessions(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(sessions, &APIMeta{Total: len(sessions)}))
}

// Get GET /api/projects/{path}/sessions/{id}
func (h *SessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectPath, sessionID := extractSessionParams(r.URL.Path)

	if projectPath == "" || sessionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path y session id requeridos"))
		return
	}

	session, err := h.claude.GetSession(projectPath, sessionID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(session))
}

// GetMessages GET /api/projects/{path}/sessions/{id}/messages
func (h *SessionsHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	projectPath, sessionID := extractSessionMessagesParams(r.URL.Path)

	if projectPath == "" || sessionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path y session id requeridos"))
		return
	}

	messages, err := h.claude.GetSessionMessages(projectPath, sessionID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(messages, &APIMeta{Total: len(messages)}))
}

// Delete DELETE /api/projects/{path}/sessions/{id}
func (h *SessionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectPath, sessionID := extractSessionParams(r.URL.Path)

	if projectPath == "" || sessionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path y session id requeridos"))
		return
	}

	if err := h.claude.DeleteSession(projectPath, sessionID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	// Eliminar del registro de terminales si existe
	h.terminals.RemoveFromSaved(sessionID)

	// Invalidar cache
	h.analytics.Invalidate(projectPath)

	json.NewEncoder(w).Encode(SuccessResponse(map[string]string{
		"message": "Sesion eliminada",
	}))
}

// DeleteMultiple POST /api/projects/{path}/sessions/delete
func (h *SessionsHandler) DeleteMultiple(w http.ResponseWriter, r *http.Request) {
	projectPath := extractSessionsProjectPath(r.URL.Path)
	projectPath = strings.TrimSuffix(projectPath, "/delete")

	if projectPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	var req struct {
		SessionIDs []string `json:"session_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("JSON invalido"))
		return
	}

	deleted, _ := h.claude.DeleteMultipleSessions(projectPath, req.SessionIDs)

	// Eliminar del registro de terminales
	for _, id := range req.SessionIDs {
		h.terminals.RemoveFromSaved(id)
	}

	// Invalidar cache
	h.analytics.Invalidate(projectPath)

	json.NewEncoder(w).Encode(SuccessResponse(map[string]interface{}{
		"deleted": deleted,
	}))
}

// CleanEmpty POST /api/projects/{path}/sessions/clean
func (h *SessionsHandler) CleanEmpty(w http.ResponseWriter, r *http.Request) {
	projectPath := extractSessionsProjectPath(r.URL.Path)
	projectPath = strings.TrimSuffix(projectPath, "/clean")

	if projectPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	deleted, err := h.claude.CleanEmptySessions(projectPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	// Invalidar cache
	h.analytics.Invalidate(projectPath)

	json.NewEncoder(w).Encode(SuccessResponse(map[string]interface{}{
		"deleted": deleted,
	}))
}

// Import POST /api/projects/{path}/sessions/import
func (h *SessionsHandler) Import(w http.ResponseWriter, r *http.Request) {
	projectPath := extractSessionsProjectPath(r.URL.Path)
	projectPath = strings.TrimSuffix(projectPath, "/import")

	if projectPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path requerido"))
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("JSON invalido"))
		return
	}

	session, err := h.claude.GetSession(projectPath, req.SessionID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	name := req.Name
	if name == "" {
		name = session.FirstMessage
		if len(name) > 50 {
			name = name[:50]
		}
		if name == "" {
			name = req.SessionID[:8]
		}
	}

	// Marcar como importado en el servicio de terminales
	h.terminals.MarkAsImported(req.SessionID, name, session.RealPath)

	json.NewEncoder(w).Encode(SuccessResponse(map[string]interface{}{
		"session_id": req.SessionID,
		"name":       name,
		"project":    projectPath,
		"work_dir":   session.RealPath,
	}))
}

// Rename PUT /api/projects/{path}/sessions/{id}/rename
func (h *SessionsHandler) Rename(w http.ResponseWriter, r *http.Request) {
	projectPath, sessionID := extractSessionRenameParams(r.URL.Path)

	if projectPath == "" || sessionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("project path y session id requeridos"))
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("JSON invalido"))
		return
	}

	// Verificar que la sesión existe
	session, err := h.claude.GetSession(projectPath, sessionID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse("sesion no encontrada"))
		return
	}

	// Guardar el nombre
	if err := services.SetSessionName(sessionID, req.Name); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	session.Name = req.Name
	json.NewEncoder(w).Encode(SuccessResponse(session))
}

// extractSessionRenameParams extrae project path y session id de URL de rename
func extractSessionRenameParams(urlPath string) (projectPath, sessionID string) {
	// /api/projects/{path}/sessions/{id}/rename
	path := strings.TrimPrefix(urlPath, "/api/projects/")
	path = strings.TrimSuffix(path, "/rename")
	parts := strings.Split(path, "/sessions/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// extractSessionsProjectPath extrae el path del proyecto de la URL de sesiones
func extractSessionsProjectPath(urlPath string) string {
	// /api/projects/{path}/sessions
	path := strings.TrimPrefix(urlPath, "/api/projects/")
	idx := strings.Index(path, "/sessions")
	if idx > 0 {
		return path[:idx]
	}
	return ""
}

// extractSessionParams extrae project path y session id
func extractSessionParams(urlPath string) (projectPath, sessionID string) {
	// /api/projects/{path}/sessions/{id}
	path := strings.TrimPrefix(urlPath, "/api/projects/")
	parts := strings.Split(path, "/sessions/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// extractSessionMessagesParams extrae project path y session id de URL de messages
func extractSessionMessagesParams(urlPath string) (projectPath, sessionID string) {
	// /api/projects/{path}/sessions/{id}/messages
	path := strings.TrimPrefix(urlPath, "/api/projects/")
	path = strings.TrimSuffix(path, "/messages")
	parts := strings.Split(path, "/sessions/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
