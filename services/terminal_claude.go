package services

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"claude-monitor/pkg/logger"

	"github.com/gorilla/websocket"
)

// TerminalClaude implementa ClaudeTerminal con lógica de estados, checkpoints y métricas
type TerminalClaude struct {
	// Campos base
	id        string
	name      string
	workDir   string
	sessionID string
	status    string
	startedAt time.Time
	config    TerminalConfig

	cmd          *exec.Cmd
	pty          PTY
	screen       *ScreenState
	claudeScreen *ClaudeAwareScreenHandler

	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex

	// Máquina de estados (migrada de Job)
	state      TerminalState
	pausedAt   *time.Time
	stoppedAt  *time.Time
	archivedAt *time.Time

	// Métricas de conversación
	messageCount      int
	userMessages      int
	assistantMessages int

	// Contadores de ciclo de vida
	pauseCount  int
	resumeCount int

	// Error handling
	termError *TerminalError

	// Flags
	isArchived   bool
	autoArchived bool

	mu sync.RWMutex
}

// NewTerminalClaude crea una nueva terminal Claude
func NewTerminalClaude(id, name, workDir string, cfg TerminalConfig) *TerminalClaude {
	return &TerminalClaude{
		id:        id,
		name:      name,
		workDir:   workDir,
		sessionID: id,
		status:    "created",
		state:     TerminalStateCreated,
		config:    cfg,
		clients:   make(map[*websocket.Conn]bool),
	}
}

// --- Implementación de Terminal ---

func (t *TerminalClaude) GetID() string {
	return t.id
}

func (t *TerminalClaude) GetType() string {
	return "claude"
}

func (t *TerminalClaude) GetName() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

func (t *TerminalClaude) GetWorkDir() string {
	return t.workDir
}

func (t *TerminalClaude) GetSessionID() string {
	return t.sessionID
}

func (t *TerminalClaude) GetStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *TerminalClaude) GetStartedAt() time.Time {
	return t.startedAt
}

func (t *TerminalClaude) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Transición de estado
	if t.state != TerminalStateCreated && t.state != TerminalStateStopped {
		return fmt.Errorf("no se puede iniciar terminal en estado %s", t.state)
	}

	t.state = TerminalStateStarting
	t.status = "starting"

	return nil
}

func (t *TerminalClaude) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TerminalStateActive && t.state != TerminalStatePaused {
		return fmt.Errorf("no se puede detener terminal en estado %s", t.state)
	}

	now := time.Now()
	t.stoppedAt = &now
	t.state = TerminalStateStopped
	t.status = "stopped"

	if t.pty != nil {
		t.pty.Close()
	}

	return nil
}

func (t *TerminalClaude) Kill() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	signaler := NewProcessSignaler()
	return signaler.Terminate(t.cmd)
}

func (t *TerminalClaude) IsActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status == "running" && t.state == TerminalStateActive
}

func (t *TerminalClaude) Write(data []byte) (int, error) {
	if t.pty == nil {
		return 0, nil
	}
	return t.pty.Write(data)
}

func (t *TerminalClaude) Resize(cols, rows uint16) error {
	if t.pty == nil {
		return nil
	}

	if err := t.pty.Resize(cols, rows); err != nil {
		return err
	}

	if t.screen != nil {
		t.screen.Resize(int(cols), int(rows))
	}

	if t.claudeScreen != nil {
		t.claudeScreen.Resize(int(cols), int(rows))
	}

	return nil
}

func (t *TerminalClaude) GetSnapshot() *TerminalSnapshot {
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

func (t *TerminalClaude) GetScreen() *ScreenState {
	return t.screen
}

func (t *TerminalClaude) AddClient(conn *websocket.Conn) {
	t.clientsMu.Lock()
	t.clients[conn] = true
	t.clientsMu.Unlock()
}

func (t *TerminalClaude) RemoveClient(conn *websocket.Conn) {
	t.clientsMu.Lock()
	delete(t.clients, conn)
	t.clientsMu.Unlock()
}

func (t *TerminalClaude) GetClientCount() int {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()
	return len(t.clients)
}

func (t *TerminalClaude) Broadcast(data []byte) {
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

func (t *TerminalClaude) GetPty() PTY {
	return t.pty
}

func (t *TerminalClaude) GetCmd() interface{} {
	return t.cmd
}

func (t *TerminalClaude) GetConfig() TerminalConfig {
	return t.config
}

func (t *TerminalClaude) GetModel() string {
	return t.config.Model
}

// --- Implementación de ClaudeTerminal ---

func (t *TerminalClaude) GetClaudeScreen() *ClaudeAwareScreenHandler {
	return t.claudeScreen
}

func (t *TerminalClaude) GetClaudeState() *ClaudeStateInfo {
	if t.claudeScreen == nil {
		return nil
	}
	state := t.claudeScreen.GetClaudeState()
	return &state
}

func (t *TerminalClaude) GetCheckpoints() []Checkpoint {
	if t.claudeScreen == nil {
		return nil
	}
	return t.claudeScreen.GetCheckpoints()
}

func (t *TerminalClaude) GetEvents() []HookEvent {
	if t.claudeScreen == nil {
		return nil
	}
	return t.claudeScreen.GetEventHistory()
}

func (t *TerminalClaude) AddCheckpoint(id, tool string, files []string) {
	if t.claudeScreen != nil {
		t.claudeScreen.AddCheckpoint(id, tool, files)
	}
}

func (t *TerminalClaude) AddEvent(eventType HookEventType, tool string, data interface{}) {
	if t.claudeScreen != nil {
		t.claudeScreen.AddEvent(eventType, tool, data)
	}
}

func (t *TerminalClaude) GetState() TerminalState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *TerminalClaude) Pause() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TerminalStateActive {
		return fmt.Errorf("solo se pueden pausar terminales ACTIVE, actual: %s", t.state)
	}

	now := time.Now()
	t.pausedAt = &now
	t.pauseCount++
	t.state = TerminalStatePaused
	t.status = "paused"

	logger.Debug("Terminal pausada", "terminal_id", t.id, "pause_count", t.pauseCount)
	return nil
}

func (t *TerminalClaude) Resume() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.state {
	case TerminalStatePaused:
		// Verificar límite de tiempo pausada (24 horas)
		if t.pausedAt != nil && time.Since(*t.pausedAt) > 24*time.Hour {
			return fmt.Errorf("terminal pausada por más de 24 horas, debe reiniciar")
		}
		t.resumeCount++
		t.state = TerminalStateActive
		t.status = "running"
		logger.Debug("Terminal reanudada desde pausa", "terminal_id", t.id, "resume_count", t.resumeCount)

	case TerminalStateStopped:
		// Verificar límite de tiempo detenida (7 días)
		if t.stoppedAt != nil && time.Since(*t.stoppedAt) > 7*24*time.Hour {
			return fmt.Errorf("terminal detenida por más de 7 días, contexto expirado")
		}
		t.resumeCount++
		t.state = TerminalStateStarting
		t.status = "starting"
		logger.Debug("Terminal reanudada desde stop", "terminal_id", t.id, "resume_count", t.resumeCount)

	default:
		return fmt.Errorf("no se puede reanudar terminal en estado %s", t.state)
	}

	return nil
}

func (t *TerminalClaude) Archive() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TerminalStateStopped && t.state != TerminalStatePaused {
		return fmt.Errorf("solo se pueden archivar terminales STOPPED o PAUSED, actual: %s", t.state)
	}

	now := time.Now()
	t.archivedAt = &now
	t.isArchived = true
	t.state = TerminalStateArchived
	t.status = "archived"

	logger.Debug("Terminal archivada", "terminal_id", t.id)
	return nil
}

func (t *TerminalClaude) GetMessageCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.messageCount
}

func (t *TerminalClaude) GetUserMessages() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.userMessages
}

func (t *TerminalClaude) GetAssistantMessages() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.assistantMessages
}

func (t *TerminalClaude) GetPausedAt() *time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.pausedAt
}

func (t *TerminalClaude) GetStoppedAt() *time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stoppedAt
}

func (t *TerminalClaude) GetArchivedAt() *time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.archivedAt
}

func (t *TerminalClaude) GetPauseCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.pauseCount
}

func (t *TerminalClaude) GetResumeCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.resumeCount
}

func (t *TerminalClaude) GetError() *TerminalError {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.termError
}

func (t *TerminalClaude) SetError(err *TerminalError) {
	t.mu.Lock()
	t.termError = err
	if err != nil {
		t.state = TerminalStateError
		t.status = "error"
	}
	t.mu.Unlock()
}

// --- Métodos internos para TerminalService ---

// SetCmd establece el comando
func (t *TerminalClaude) SetCmd(cmd *exec.Cmd) {
	t.cmd = cmd
}

// SetPty establece el PTY
func (t *TerminalClaude) SetPty(pty PTY) {
	t.pty = pty
}

// SetScreen establece el screen state
func (t *TerminalClaude) SetScreen(screen *ScreenState) {
	t.screen = screen
}

// SetClaudeScreen establece el ClaudeAwareScreenHandler
func (t *TerminalClaude) SetClaudeScreen(cs *ClaudeAwareScreenHandler) {
	t.claudeScreen = cs
}

// SetStatus cambia el estado
func (t *TerminalClaude) SetStatus(status string) {
	t.mu.Lock()
	t.status = status
	t.mu.Unlock()
}

// SetState cambia el estado de la máquina de estados
func (t *TerminalClaude) SetState(state TerminalState) {
	t.mu.Lock()
	t.state = state
	t.mu.Unlock()
}

// MarkActive marca la terminal como activa (después de iniciar PTY)
func (t *TerminalClaude) MarkActive() {
	t.mu.Lock()
	t.state = TerminalStateActive
	t.status = "running"
	t.startedAt = time.Now()
	t.mu.Unlock()
}

// FeedScreen alimenta datos al screen
func (t *TerminalClaude) FeedScreen(data []byte) error {
	if t.screen != nil {
		if err := t.screen.Feed(data); err != nil {
			return err
		}
	}
	if t.claudeScreen != nil {
		return t.claudeScreen.Feed(data)
	}
	return nil
}

// GetClients retorna el mapa de clientes (para cleanup)
func (t *TerminalClaude) GetClients() map[*websocket.Conn]bool {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()

	clients := make(map[*websocket.Conn]bool, len(t.clients))
	for k, v := range t.clients {
		clients[k] = v
	}
	return clients
}

// ClearClients limpia todos los clientes
func (t *TerminalClaude) ClearClients() {
	t.clientsMu.Lock()
	t.clients = make(map[*websocket.Conn]bool)
	t.clientsMu.Unlock()
}

// IncrementMessageCount incrementa contadores de mensajes
func (t *TerminalClaude) IncrementMessageCount(isUser bool) {
	t.mu.Lock()
	t.messageCount++
	if isUser {
		t.userMessages++
	} else {
		t.assistantMessages++
	}
	t.mu.Unlock()
}

// BroadcastClaudeEvent envía un evento de Claude a todos los clientes
func (t *TerminalClaude) BroadcastClaudeEvent(eventType string, data interface{}) {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()

	msg := ClaudeEventMessage{
		Type:      "claude:event",
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	for client := range t.clients {
		if err := client.WriteJSON(msg); err != nil {
			logger.Debug("Error enviando evento claude", "error", err, "event_type", eventType)
		}
	}
}

// GetClaudeStateSnapshot retorna un snapshot del estado de Claude
func (t *TerminalClaude) GetClaudeStateSnapshot() *ClaudeStateSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &ClaudeStateSnapshot{
		State:             t.state,
		MessageCount:      t.messageCount,
		UserMessages:      t.userMessages,
		AssistantMessages: t.assistantMessages,
		PauseCount:        t.pauseCount,
		ResumeCount:       t.resumeCount,
		PausedAt:          t.pausedAt,
		StoppedAt:         t.stoppedAt,
		ArchivedAt:        t.archivedAt,
		Error:             t.termError,
	}
}

// GetIsArchived retorna si está archivada
func (t *TerminalClaude) GetIsArchived() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isArchived
}

// Reopen reabre una terminal archivada
func (t *TerminalClaude) Reopen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TerminalStateArchived {
		return fmt.Errorf("solo se pueden reabrir terminales ARCHIVED, actual: %s", t.state)
	}

	t.isArchived = false
	t.archivedAt = nil
	t.state = TerminalStateStopped
	t.status = "stopped"

	logger.Debug("Terminal reabierta desde archivo", "terminal_id", t.id)
	return nil
}

// CanRetry verifica si se puede reintentar
func (t *TerminalClaude) CanRetry() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.state != TerminalStateError || t.termError == nil {
		return false
	}
	return t.termError.RetryCount < 3
}

// Retry reintenta desde estado de error
func (t *TerminalClaude) Retry() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TerminalStateError {
		return fmt.Errorf("solo se puede reintentar en estado ERROR, actual: %s", t.state)
	}

	if t.termError != nil {
		t.termError.RetryCount++
	}

	t.state = TerminalStateStarting
	t.status = "starting"

	logger.Debug("Terminal reintentando", "terminal_id", t.id, "retry_count", t.termError.RetryCount)
	return nil
}

// GetValidTransitions retorna las transiciones válidas desde el estado actual
func (t *TerminalClaude) GetValidTransitions() []string {
	t.mu.RLock()
	state := t.state
	t.mu.RUnlock()

	switch state {
	case TerminalStateCreated:
		return []string{"START", "DELETE"}
	case TerminalStateStarting:
		return []string{"READY", "FAILED"}
	case TerminalStateActive:
		return []string{"PAUSE", "STOP", "ERROR"}
	case TerminalStatePaused:
		return []string{"RESUME", "STOP", "ARCHIVE"}
	case TerminalStateStopped:
		return []string{"RESUME", "ARCHIVE", "DELETE"}
	case TerminalStateArchived:
		return []string{"REOPEN", "DELETE"}
	case TerminalStateError:
		return []string{"RETRY", "DISCARD"}
	default:
		return []string{}
	}
}
