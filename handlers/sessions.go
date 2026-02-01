package handlers

import (
	"encoding/json"
	"net/http"
	"os"
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

// List godoc
// @Summary      Listar sesiones
// @Description  Retorna todas las sesiones de Claude en un session-root
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Failure      500       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions [get]
// @Security     BasicAuth
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

// Get godoc
// @Summary      Obtener sesión
// @Description  Retorna información detallada de una sesión específica
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath   path      string  true  "Path del session-root (URL encoded)"
// @Param        sessionID  path      string  true  "ID de la sesión"
// @Success      200        {object}  handlers.APIResponse{data=services.ClaudeSession}
// @Failure      400        {object}  handlers.APIResponse
// @Failure      404        {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/{sessionID} [get]
// @Security     BasicAuth
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

// GetMessages godoc
// @Summary      Obtener mensajes de sesión
// @Description  Retorna todos los mensajes de una sesión
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath   path      string  true  "Path del session-root (URL encoded)"
// @Param        sessionID  path      string  true  "ID de la sesión"
// @Success      200        {object}  map[string]interface{}
// @Failure      400        {object}  handlers.APIResponse
// @Failure      404        {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/{sessionID}/messages [get]
// @Security     BasicAuth
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

// GetRealTimeMessages godoc
// @Summary      Obtener mensajes en tiempo real
// @Description  Retorna mensajes nuevos desde una línea específica (para polling)
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath   path      string  true   "Path del session-root (URL encoded)"
// @Param        sessionID  path      string  true   "ID de la sesión"
// @Param        from       query     int     false  "Línea desde donde obtener mensajes (default: 0)"
// @Success      200        {object}  map[string]interface{}
// @Failure      400        {object}  handlers.APIResponse
// @Failure      404        {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/{sessionID}/messages/realtime [get]
// @Security     BasicAuth
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

// Delete godoc
// @Summary      Eliminar sesión
// @Description  Elimina una sesión y sus mensajes
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath   path      string  true  "Path del session-root (URL encoded)"
// @Param        sessionID  path      string  true  "ID de la sesión"
// @Success      200        {object}  handlers.APIResponse
// @Failure      400        {object}  handlers.APIResponse
// @Failure      500        {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/{sessionID} [delete]
// @Security     BasicAuth
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

// DeleteMultiple godoc
// @Summary      Eliminar múltiples sesiones
// @Description  Elimina varias sesiones a la vez
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string                         true  "Path del session-root (URL encoded)"
// @Param        request   body      object{session_ids=[]string}   true  "IDs de sesiones a eliminar"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/delete [post]
// @Security     BasicAuth
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

// CleanEmpty godoc
// @Summary      Limpiar sesiones vacías
// @Description  Elimina todas las sesiones sin mensajes
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Failure      500       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/clean [post]
// @Security     BasicAuth
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

// Import godoc
// @Summary      Importar sesión a terminales
// @Description  Marca una sesión para que aparezca en la lista de terminales guardadas
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string                           true  "Path del session-root (URL encoded)"
// @Param        request   body      object{session_id=string,name=string}  true  "Datos de importación"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Failure      404       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/import [post]
// @Security     BasicAuth
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

// Rename godoc
// @Summary      Renombrar sesión
// @Description  Cambia el nombre personalizado de una sesión
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath   path      string               true  "Path del session-root (URL encoded)"
// @Param        sessionID  path      string               true  "ID de la sesión"
// @Param        request    body      object{name=string}  true  "Nuevo nombre"
// @Success      200        {object}  handlers.APIResponse{data=services.ClaudeSession}
// @Failure      400        {object}  handlers.APIResponse
// @Failure      404        {object}  handlers.APIResponse
// @Failure      500        {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/{sessionID}/rename [put]
// @Security     BasicAuth
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

// Move godoc
// @Summary      Mover sesión a otro directorio
// @Description  Mueve una sesión a otro directorio, actualizando todas las rutas internas del JSONL
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        rootPath   path      string                    true  "Path del session-root actual (URL encoded)"
// @Param        sessionID  path      string                    true  "ID de la sesión"
// @Param        request    body      object{new_path=string}   true  "Nueva ruta absoluta del proyecto"
// @Success      200        {object}  handlers.APIResponse
// @Failure      400        {object}  handlers.APIResponse
// @Failure      404        {object}  handlers.APIResponse
// @Failure      409        {object}  handlers.APIResponse
// @Failure      500        {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/sessions/{sessionID}/move [post]
// @Security     BasicAuth
func (h *SessionsHandler) Move(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	sessionID := URLParam(r, "sessionID")

	if rootPath == "" || sessionID == "" {
		WriteBadRequest(w, "root path y session id requeridos")
		return
	}

	var req struct {
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "JSON invalido")
		return
	}

	if req.NewPath == "" {
		WriteBadRequest(w, "new_path es requerido")
		return
	}

	if err := h.claude.MoveSession(rootPath, sessionID, req.NewPath); err != nil {
		if err == os.ErrInvalid {
			WriteBadRequest(w, "new_path debe ser una ruta absoluta")
			return
		}
		if err == os.ErrExist {
			WriteConflict(w, "la sesion ya esta en ese directorio")
			return
		}
		if os.IsNotExist(err) {
			WriteNotFound(w, "sesion")
			return
		}
		WriteInternalError(w, err.Error())
		return
	}

	// Invalidar cache
	h.analytics.Invalidate(rootPath)
	h.analytics.Invalidate(services.EncodeProjectPath(req.NewPath))

	WriteSuccess(w, nil)
}
