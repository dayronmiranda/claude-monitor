package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// List GET /api/session-roots/{rootPath}/sessions
func (h *SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	path := URLParamDecoded(r, "rootPath")
	if path == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	sessions, err := h.claude.ListSessions(path)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(sessions, &APIMeta{Total: len(sessions)}))
}

// Get GET /api/session-roots/{rootPath}/sessions/{sessionID}
func (h *SessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	sessionID := URLParam(r, "sessionID")

	if rootPath == "" || sessionID == "" {
		WriteBadRequest(w, "root path y session id requeridos")
		return
	}

	session, err := h.claude.GetSession(rootPath, sessionID)
	if err != nil {
		WriteNotFound(w, "sesion")
		return
	}

	WriteSuccess(w, session)
}

// GetMessages GET /api/session-roots/{rootPath}/sessions/{sessionID}/messages
func (h *SessionsHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	sessionID := URLParam(r, "sessionID")

	if rootPath == "" || sessionID == "" {
		WriteBadRequest(w, "root path y session id requeridos")
		return
	}

	messages, err := h.claude.GetSessionMessages(rootPath, sessionID)
	if err != nil {
		WriteNotFound(w, "sesion")
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(messages, &APIMeta{Total: len(messages)}))
}

// GetRealTimeMessages GET /api/session-roots/{rootPath}/sessions/{sessionID}/messages/realtime?from=N
func (h *SessionsHandler) GetRealTimeMessages(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	sessionID := URLParam(r, "sessionID")

	if rootPath == "" || sessionID == "" {
		WriteBadRequest(w, "root path y session id requeridos")
		return
	}

	// Obtener línea de inicio desde query parameter
	fromLine := 0
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if n, err := strconv.Atoi(fromStr); err == nil && n >= 0 {
			fromLine = n
		}
	}

	messages, err := h.claude.GetSessionMessagesFromLine(rootPath, sessionID, fromLine)
	if err != nil {
		WriteNotFound(w, "sesion")
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(messages, &APIMeta{Total: len(messages)}))
}

// Delete DELETE /api/session-roots/{rootPath}/sessions/{sessionID}
func (h *SessionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	sessionID := URLParam(r, "sessionID")

	if rootPath == "" || sessionID == "" {
		WriteBadRequest(w, "root path y session id requeridos")
		return
	}

	if err := h.claude.DeleteSession(rootPath, sessionID); err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	// Eliminar del registro de terminales si existe
	h.terminals.RemoveFromSaved(sessionID)

	// Invalidar cache
	h.analytics.Invalidate(rootPath)

	WriteSuccess(w, map[string]string{"message": "Sesion eliminada"})
}

// DeleteMultiple POST /api/session-roots/{rootPath}/sessions/delete
func (h *SessionsHandler) DeleteMultiple(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	if rootPath == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	var req struct {
		SessionIDs []string `json:"session_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "JSON invalido")
		return
	}

	deleted, _ := h.claude.DeleteMultipleSessions(rootPath, req.SessionIDs)

	// Eliminar del registro de terminales
	for _, id := range req.SessionIDs {
		h.terminals.RemoveFromSaved(id)
	}

	// Invalidar cache
	h.analytics.Invalidate(rootPath)

	WriteSuccess(w, map[string]interface{}{"deleted": deleted})
}

// CleanEmpty POST /api/session-roots/{rootPath}/sessions/clean
func (h *SessionsHandler) CleanEmpty(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	if rootPath == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	deleted, err := h.claude.CleanEmptySessions(rootPath)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	// Invalidar cache
	h.analytics.Invalidate(rootPath)

	WriteSuccess(w, map[string]interface{}{"deleted": deleted})
}

// Import POST /api/session-roots/{rootPath}/sessions/import
func (h *SessionsHandler) Import(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	if rootPath == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "JSON invalido")
		return
	}

	session, err := h.claude.GetSession(rootPath, req.SessionID)
	if err != nil {
		WriteNotFound(w, "sesion")
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

	WriteSuccess(w, map[string]interface{}{
		"session_id":   req.SessionID,
		"name":         name,
		"session_root": rootPath,
		"work_dir":     session.RealPath,
	})
}

// Rename PUT /api/session-roots/{rootPath}/sessions/{sessionID}/rename
func (h *SessionsHandler) Rename(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	sessionID := URLParam(r, "sessionID")

	if rootPath == "" || sessionID == "" {
		WriteBadRequest(w, "root path y session id requeridos")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "JSON invalido")
		return
	}

	// Verificar que la sesión existe
	session, err := h.claude.GetSession(rootPath, sessionID)
	if err != nil {
		WriteNotFound(w, "sesion")
		return
	}

	// Guardar el nombre
	if err := services.SetSessionName(sessionID, req.Name); err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	session.Name = req.Name
	WriteSuccess(w, session)
}
