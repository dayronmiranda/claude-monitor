package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	apierrors "claude-monitor/pkg/errors"
	"claude-monitor/pkg/logger"
	"claude-monitor/pkg/validator"
	"claude-monitor/services"
)

// TerminalsHandler maneja endpoints de terminales
type TerminalsHandler struct {
	terminals           *services.TerminalService
	upgrader            websocket.Upgrader
	allowedPathPrefixes []string
}

// NewTerminalsHandler crea un nuevo handler
func NewTerminalsHandler(terminals *services.TerminalService, allowedPathPrefixes []string) *TerminalsHandler {
	return &TerminalsHandler{
		terminals:           terminals,
		allowedPathPrefixes: allowedPathPrefixes,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// List godoc
// @Summary      Listar terminales
// @Description  Retorna lista de todas las terminales activas y guardadas
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Success      200  {object}  handlers.APIResponse
// @Router       /terminals [get]
// @Security     BasicAuth
func (h *TerminalsHandler) List(w http.ResponseWriter, r *http.Request) {
	terminals := h.terminals.List()
	json.NewEncoder(w).Encode(SuccessWithMeta(terminals, &APIMeta{Total: len(terminals)}))
}

// Create godoc
// @Summary      Crear terminal
// @Description  Crea una nueva terminal PTY (tipo claude o terminal estándar)
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        request  body      services.TerminalConfig  true  "Configuración de terminal"
// @Success      201      {object}  handlers.APIResponse{data=services.TerminalInfo}
// @Failure      400      {object}  handlers.APIResponse
// @Failure      409      {object}  handlers.APIResponse
// @Failure      500      {object}  handlers.APIResponse
// @Router       /terminals [post]
// @Security     BasicAuth
func (h *TerminalsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Validar request
	req, err := validator.DecodeAndValidate(r, validator.ValidateTerminalConfig)
	if err != nil {
		if apiErr, ok := err.(*apierrors.APIError); ok {
			apierrors.WriteError(w, apiErr)
		} else {
			WriteBadRequest(w, err.Error())
		}
		return
	}

	// Convertir a TerminalConfig
	cfg := services.TerminalConfig{
		ID:              req.ID,
		Name:            req.Name,
		WorkDir:         req.WorkDir,
		Type:            req.Type,
		Model:           req.Model,
		Resume:          req.Resume,
		Continue:        req.Continue,
		AllowedTools:    req.AllowedTools,
		DisallowedTools: req.DisallowedTools,
	}

	terminal, err := h.terminals.Create(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "no permitido") || strings.Contains(err.Error(), "invalido") {
			WriteBadRequest(w, err.Error())
		} else if strings.Contains(err.Error(), "ya esta activa") {
			WriteConflict(w, err.Error())
		} else {
			WriteInternalError(w, err.Error())
		}
		return
	}

	WriteCreated(w, terminal)
}

// Get godoc
// @Summary      Obtener terminal
// @Description  Retorna información de una terminal específica
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  handlers.APIResponse{data=services.TerminalInfo}
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID} [get]
// @Security     BasicAuth
func (h *TerminalsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	terminal, err := h.terminals.Get(id)
	if err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	WriteSuccess(w, terminal)
}

// Delete godoc
// @Summary      Eliminar terminal
// @Description  Elimina una terminal guardada (no activa)
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  handlers.APIResponse
// @Failure      400         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID} [delete]
// @Security     BasicAuth
func (h *TerminalsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	if err := h.terminals.Delete(id); err != nil {
		WriteBadRequest(w, err.Error())
		return
	}

	WriteSuccess(w, map[string]string{"message": "Terminal eliminada"})
}

// Kill godoc
// @Summary      Matar terminal
// @Description  Termina forzosamente una terminal activa
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  handlers.APIResponse
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/kill [post]
// @Security     BasicAuth
func (h *TerminalsHandler) Kill(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	if err := h.terminals.Kill(id); err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	WriteSuccess(w, map[string]string{"message": "Terminal terminada"})
}

// Resume godoc
// @Summary      Reanudar terminal
// @Description  Reanuda una terminal guardada
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  handlers.APIResponse{data=services.TerminalInfo}
// @Failure      400         {object}  handlers.APIResponse
// @Failure      500         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/resume [post]
// @Security     BasicAuth
func (h *TerminalsHandler) Resume(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	terminal, err := h.terminals.Resume(id)
	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, terminal)
}

// Resize godoc
// @Summary      Redimensionar terminal
// @Description  Cambia el tamaño de una terminal activa
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Param        request     body      object{rows=int,cols=int}  true  "Nuevas dimensiones"
// @Success      200         {object}  handlers.APIResponse
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/resize [post]
// @Security     BasicAuth
func (h *TerminalsHandler) Resize(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	var req struct {
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "JSON invalido")
		return
	}

	if err := h.terminals.Resize(id, req.Rows, req.Cols); err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	WriteSuccess(w, map[string]string{"message": "Terminal redimensionada"})
}

// WebSocket godoc
// @Summary      Conectar WebSocket a terminal
// @Description  Establece conexión WebSocket para interactuar con la terminal en tiempo real
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      101         {string}  string  "Switching Protocols"
// @Failure      400         {string}  string
// @Failure      404         {string}  string
// @Router       /terminals/{terminalID}/ws [get]
// @Security     BasicAuth
func (h *TerminalsHandler) WebSocket(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		http.Error(w, "terminal id requerido", http.StatusBadRequest)
		return
	}

	if !h.terminals.IsActive(id) {
		http.Error(w, "terminal no activa", http.StatusNotFound)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Error upgrading WebSocket", "error", err)
		return
	}

	if err := h.terminals.AddClient(id, conn); err != nil {
		conn.Close()
		return
	}

	// Enviar snapshot inicial al cliente (estado actual de la pantalla)
	if snapshot, err := h.terminals.GetSnapshot(id); err == nil {
		conn.WriteJSON(map[string]interface{}{
			"type":     "snapshot",
			"snapshot": snapshot,
		})
		logger.Debug("Snapshot inicial enviado", "terminal_id", id)
	}

	// Configurar ping/pong
	const (
		pingInterval = 30 * time.Second
		pongTimeout  = 60 * time.Second
	)

	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	// Goroutine para enviar pings
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	defer func() {
		close(done)
		h.terminals.RemoveClient(id, conn)
		conn.Close()
	}()

	for {
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Rows uint16 `json:"rows"`
			Cols uint16 `json:"cols"`
		}

		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Debug("WebSocket closed", "terminal_id", id, "error", err)
			}
			break
		}

		switch msg.Type {
		case "input":
			h.terminals.Write(id, []byte(msg.Data))
		case "resize":
			h.terminals.Resize(id, msg.Rows, msg.Cols)
		}
	}
}

// ListDir godoc
// @Summary      Listar directorio
// @Description  Lista el contenido de un directorio del filesystem (restringido a paths permitidos)
// @Tags         filesystem
// @Accept       json
// @Produce      json
// @Param        path  query     string  false  "Path del directorio (default: /)"
// @Success      200   {object}  handlers.APIResponse
// @Failure      400   {object}  handlers.APIResponse
// @Router       /filesystem/dir [get]
// @Security     BasicAuth
func (h *TerminalsHandler) ListDir(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	entries, err := services.ListDirectory(path, h.allowedPathPrefixes)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(map[string]interface{}{
		"current_path": path,
		"entries":      entries,
	}))
}

// Snapshot godoc
// @Summary      Obtener snapshot de terminal
// @Description  Retorna el estado actual de la pantalla de la terminal
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  handlers.APIResponse
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/snapshot [get]
// @Security     BasicAuth
func (h *TerminalsHandler) Snapshot(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	snapshot, err := h.terminals.GetSnapshot(id)
	if err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	WriteSuccess(w, snapshot)
}

// ClaudeState godoc
// @Summary      Obtener estado de Claude
// @Description  Retorna el estado actual del agente Claude en la terminal (solo terminales tipo claude)
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  handlers.APIResponse
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/claude-state [get]
// @Security     BasicAuth
func (h *TerminalsHandler) ClaudeState(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	state, err := h.terminals.GetClaudeState(id)
	if err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	WriteSuccess(w, state)
}

// ClaudeCheckpoints godoc
// @Summary      Obtener checkpoints de Claude
// @Description  Retorna los checkpoints/puntos de control del agente Claude
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  map[string]interface{}
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/checkpoints [get]
// @Security     BasicAuth
func (h *TerminalsHandler) ClaudeCheckpoints(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	checkpoints, err := h.terminals.GetClaudeCheckpoints(id)
	if err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(checkpoints, &APIMeta{Total: len(checkpoints)}))
}

// ClaudeEvents godoc
// @Summary      Obtener eventos de Claude
// @Description  Retorna los eventos del agente Claude (tool calls, respuestas, etc.)
// @Tags         terminals
// @Accept       json
// @Produce      json
// @Param        terminalID  path      string  true  "ID de la terminal"
// @Success      200         {object}  map[string]interface{}
// @Failure      400         {object}  handlers.APIResponse
// @Failure      404         {object}  handlers.APIResponse
// @Router       /terminals/{terminalID}/events [get]
// @Security     BasicAuth
func (h *TerminalsHandler) ClaudeEvents(w http.ResponseWriter, r *http.Request) {
	id := URLParam(r, "terminalID")
	if id == "" {
		WriteBadRequest(w, "terminal id requerido")
		return
	}

	events, err := h.terminals.GetClaudeEvents(id)
	if err != nil {
		WriteNotFound(w, "terminal")
		return
	}

	json.NewEncoder(w).Encode(SuccessWithMeta(events, &APIMeta{Total: len(events)}))
}
