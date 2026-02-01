package services

import (
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TerminalRaw implementa Terminal para PTY genérico sin lógica Claude
type TerminalRaw struct {
	id        string
	name      string
	workDir   string
	sessionID string
	status    string
	startedAt time.Time
	config    TerminalConfig

	cmd    *exec.Cmd
	pty    PTY
	screen *ScreenState

	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	mu        sync.RWMutex
}

// NewTerminalRaw crea una nueva terminal raw
func NewTerminalRaw(id, name, workDir string, cfg TerminalConfig) *TerminalRaw {
	return &TerminalRaw{
		id:        id,
		name:      name,
		workDir:   workDir,
		sessionID: id,
		status:    "created",
		config:    cfg,
		clients:   make(map[*websocket.Conn]bool),
	}
}

// --- Implementación de Terminal ---

func (t *TerminalRaw) GetID() string {
	return t.id
}

func (t *TerminalRaw) GetType() string {
	return "terminal"
}

func (t *TerminalRaw) GetName() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

func (t *TerminalRaw) GetWorkDir() string {
	return t.workDir
}

func (t *TerminalRaw) GetSessionID() string {
	return t.sessionID
}

func (t *TerminalRaw) GetStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *TerminalRaw) GetStartedAt() time.Time {
	return t.startedAt
}

func (t *TerminalRaw) Start() error {
	t.mu.Lock()
	t.status = "running"
	t.startedAt = time.Now()
	t.mu.Unlock()
	return nil
}

func (t *TerminalRaw) Stop() error {
	t.mu.Lock()
	t.status = "stopped"
	t.mu.Unlock()

	if t.pty != nil {
		t.pty.Close()
	}
	return nil
}

func (t *TerminalRaw) Kill() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	signaler := NewProcessSignaler()
	return signaler.Terminate(t.cmd)
}

func (t *TerminalRaw) IsActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status == "running"
}

func (t *TerminalRaw) Write(data []byte) (int, error) {
	if t.pty == nil {
		return 0, nil
	}
	return t.pty.Write(data)
}

func (t *TerminalRaw) Resize(cols, rows uint16) error {
	if t.pty == nil {
		return nil
	}

	if err := t.pty.Resize(cols, rows); err != nil {
		return err
	}

	if t.screen != nil {
		t.screen.Resize(int(cols), int(rows))
	}

	return nil
}

func (t *TerminalRaw) GetSnapshot() *TerminalSnapshot {
	if t.screen == nil {
		return nil
	}

	cursorX, cursorY := t.screen.GetCursor()
	width, height := t.screen.GetSize()

	return &TerminalSnapshot{
		Content:           t.screen.Snapshot(),
		Display:           t.screen.GetDisplay(),
		CursorX:           cursorX,
		CursorY:           cursorY,
		Width:             width,
		Height:            height,
		InAlternateScreen: t.screen.IsInAlternateScreen(),
		History:           t.screen.GetHistoryLines(),
	}
}

func (t *TerminalRaw) GetScreen() *ScreenState {
	return t.screen
}

func (t *TerminalRaw) AddClient(conn *websocket.Conn) {
	t.clientsMu.Lock()
	t.clients[conn] = true
	t.clientsMu.Unlock()
}

func (t *TerminalRaw) RemoveClient(conn *websocket.Conn) {
	t.clientsMu.Lock()
	delete(t.clients, conn)
	t.clientsMu.Unlock()
}

func (t *TerminalRaw) GetClientCount() int {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()
	return len(t.clients)
}

func (t *TerminalRaw) Broadcast(data []byte) {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()

	msg := map[string]string{
		"type": "output",
		"data": string(data),
	}

	for client := range t.clients {
		client.WriteJSON(msg)
	}
}

func (t *TerminalRaw) GetPty() PTY {
	return t.pty
}

func (t *TerminalRaw) GetCmd() interface{} {
	return t.cmd
}

func (t *TerminalRaw) GetConfig() TerminalConfig {
	return t.config
}

func (t *TerminalRaw) GetModel() string {
	return t.config.Model
}

// --- Métodos internos para TerminalService ---

// SetCmd establece el comando (usado internamente por TerminalService)
func (t *TerminalRaw) SetCmd(cmd *exec.Cmd) {
	t.cmd = cmd
}

// SetPty establece el PTY (usado internamente por TerminalService)
func (t *TerminalRaw) SetPty(pty PTY) {
	t.pty = pty
}

// SetScreen establece el screen state (usado internamente por TerminalService)
func (t *TerminalRaw) SetScreen(screen *ScreenState) {
	t.screen = screen
}

// SetStatus cambia el estado (usado internamente)
func (t *TerminalRaw) SetStatus(status string) {
	t.mu.Lock()
	t.status = status
	t.mu.Unlock()
}

// FeedScreen alimenta datos al screen
func (t *TerminalRaw) FeedScreen(data []byte) error {
	if t.screen != nil {
		return t.screen.Feed(data)
	}
	return nil
}

// GetClients retorna el mapa de clientes (para cleanup)
func (t *TerminalRaw) GetClients() map[*websocket.Conn]bool {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()

	// Retornar copia
	clients := make(map[*websocket.Conn]bool, len(t.clients))
	for k, v := range t.clients {
		clients[k] = v
	}
	return clients
}

// ClearClients limpia todos los clientes
func (t *TerminalRaw) ClearClients() {
	t.clientsMu.Lock()
	t.clients = make(map[*websocket.Conn]bool)
	t.clientsMu.Unlock()
}
