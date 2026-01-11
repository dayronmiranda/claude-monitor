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

// List GET /api/terminals
func (h *TerminalsHandler) List(w http.ResponseWriter, r *http.Request) {
	terminals := h.terminals.List()
	json.NewEncoder(w).Encode(SuccessWithMeta(terminals, &APIMeta{Total: len(terminals)}))
}

// Create POST /api/terminals
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

// Get GET /api/terminals/{id}
func (h *TerminalsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := extractTerminalID(r.URL.Path)

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("terminal id requerido"))
		return
	}

	terminal, err := h.terminals.Get(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(terminal))
}

// Delete DELETE /api/terminals/{id}
func (h *TerminalsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := extractTerminalID(r.URL.Path)

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("terminal id requerido"))
		return
	}

	if err := h.terminals.Delete(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(map[string]string{
		"message": "Terminal eliminada",
	}))
}

// Kill POST /api/terminals/{id}/kill
func (h *TerminalsHandler) Kill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(extractTerminalID(r.URL.Path), "/kill")

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("terminal id requerido"))
		return
	}

	if err := h.terminals.Kill(id); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(map[string]string{
		"message": "Terminal terminada",
	}))
}

// Resume POST /api/terminals/{id}/resume
func (h *TerminalsHandler) Resume(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(extractTerminalID(r.URL.Path), "/resume")

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("terminal id requerido"))
		return
	}

	terminal, err := h.terminals.Resume(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(terminal))
}

// Resize POST /api/terminals/{id}/resize
func (h *TerminalsHandler) Resize(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(extractTerminalID(r.URL.Path), "/resize")

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("terminal id requerido"))
		return
	}

	var req struct {
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse("JSON invalido"))
		return
	}

	if err := h.terminals.Resize(id, req.Rows, req.Cols); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(SuccessResponse(map[string]string{
		"message": "Terminal redimensionada",
	}))
}

// WebSocket GET /api/terminals/{id}/ws
func (h *TerminalsHandler) WebSocket(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(extractTerminalID(r.URL.Path), "/ws")

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

// ListDir GET /api/filesystem/dir
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

// extractTerminalID extrae el ID de terminal de la URL
func extractTerminalID(urlPath string) string {
	path := strings.TrimPrefix(urlPath, "/api/terminals/")
	// Quitar sufijos de acciones
	path = strings.TrimSuffix(path, "/kill")
	path = strings.TrimSuffix(path, "/resume")
	path = strings.TrimSuffix(path, "/resize")
	path = strings.TrimSuffix(path, "/ws")
	return path
}
