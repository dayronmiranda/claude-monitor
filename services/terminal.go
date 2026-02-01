package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"claude-monitor/pkg/logger"

	"github.com/gorilla/websocket"
)

// TerminalService gestiona las terminales PTY
type TerminalService struct {
	terminals           map[string]Terminal // Interface Terminal
	mu                  sync.RWMutex
	saved               map[string]*SavedTerminal
	savedMu             sync.RWMutex
	sessionsFile        string
	onTerminalEnd       func(id string)
	allowedPathPrefixes []string
}

// SavedTerminal terminal guardada para persistencia
type SavedTerminal struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	WorkDir      string              `json:"work_dir"`
	SessionID    string              `json:"session_id,omitempty"`
	Type         string              `json:"type"`
	Model        string              `json:"model,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	LastAccessAt time.Time           `json:"last_access_at"`
	Status       string              `json:"status"`
	Config       TerminalConfig      `json:"config"`
	ClaudeState  *ClaudeStateSnapshot `json:"claude_state,omitempty"` // Estado Claude extendido
}

// TerminalConfig configuración para crear terminal
type TerminalConfig struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name,omitempty"`
	WorkDir         string   `json:"work_dir"`
	Type            string   `json:"type,omitempty"` // "claude" o "terminal"
	Command         string   `json:"command,omitempty"`
	Model           string   `json:"model,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
	PermissionMode  string   `json:"permission_mode,omitempty"`
	AdditionalDirs  []string `json:"additional_dirs,omitempty"`
	Resume          bool     `json:"resume,omitempty"`
	Continue        bool     `json:"continue,omitempty"`
	DangerouslySkip bool     `json:"dangerously_skip,omitempty"`
}

// TerminalInfo información de terminal para API
type TerminalInfo struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	WorkDir      string              `json:"work_dir"`
	SessionID    string              `json:"session_id,omitempty"`
	Type         string              `json:"type"`
	Status       string              `json:"status"`
	Model        string              `json:"model,omitempty"`
	Active       bool                `json:"active"`
	Clients      int                 `json:"clients"`
	CanResume    bool                `json:"can_resume"`
	StartedAt    time.Time           `json:"started_at,omitempty"`
	CreatedAt    time.Time           `json:"created_at,omitempty"`
	LastAccessAt time.Time           `json:"last_access_at,omitempty"`
	ClaudeState  *ClaudeStateSnapshot `json:"claude_state,omitempty"` // Solo para tipo claude
}

// DirectoryEntry entrada de directorio
type DirectoryEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// NewTerminalService crea una nueva instancia del servicio
func NewTerminalService(dataDir string, allowedPathPrefixes ...string) *TerminalService {
	sessionsFile := filepath.Join(dataDir, "terminals.json")
	ts := &TerminalService{
		terminals:           make(map[string]Terminal),
		saved:               make(map[string]*SavedTerminal),
		sessionsFile:        sessionsFile,
		allowedPathPrefixes: allowedPathPrefixes,
	}
	ts.loadSaved()
	return ts
}

// SetAllowedPathPrefixes configura los prefijos de path permitidos
func (s *TerminalService) SetAllowedPathPrefixes(prefixes []string) {
	s.allowedPathPrefixes = prefixes
}

// SetOnTerminalEnd configura callback cuando termina una terminal
func (s *TerminalService) SetOnTerminalEnd(fn func(id string)) {
	s.onTerminalEnd = fn
}

// loadSaved carga terminales guardadas
func (s *TerminalService) loadSaved() {
	data, err := os.ReadFile(s.sessionsFile)
	if err != nil {
		return
	}

	var terminals []SavedTerminal
	if err := json.Unmarshal(data, &terminals); err != nil {
		logger.Error("Error cargando terminales", "error", err)
		return
	}

	s.savedMu.Lock()
	for _, t := range terminals {
		term := t
		s.saved[t.ID] = &term
	}
	s.savedMu.Unlock()

	logger.Info("Terminales guardadas cargadas", "count", len(terminals))
}

// persistSaved guarda terminales a archivo de forma atómica
func (s *TerminalService) persistSaved() {
	s.savedMu.RLock()
	terminals := make([]SavedTerminal, 0, len(s.saved))
	for _, t := range s.saved {
		terminals = append(terminals, *t)
	}
	s.savedMu.RUnlock()

	data, err := json.MarshalIndent(terminals, "", "  ")
	if err != nil {
		logger.Error("Error serializando terminales", "error", err)
		return
	}

	if err := atomicWriteFile(s.sessionsFile, data, 0600); err != nil {
		logger.Error("Error guardando terminales", "error", err)
	}
}

// atomicWriteFile escribe un archivo de forma atómica
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

// PathValidator función de validación de paths
type PathValidator func(path string) error

// ValidatePath verifica que un path sea válido y esté en prefijos permitidos
func ValidatePath(path string, allowedPrefixes []string) error {
	cleaned := filepath.Clean(path)

	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("path debe ser absoluto: %s", path)
	}

	if strings.Contains(path, "..") {
		logger.Warn("Intento de path traversal detectado", "path", path)
		return fmt.Errorf("path traversal no permitido")
	}

	if len(allowedPrefixes) == 0 {
		return nil
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cleaned, filepath.Clean(prefix)) {
			return nil
		}
	}

	logger.Warn("Path fuera de prefijos permitidos",
		"path", path,
		"allowed_prefixes", allowedPrefixes,
	)
	return fmt.Errorf("path no permitido: %s", path)
}

// generateUUID genera un UUID v4
func generateUUID() string {
	uuid := make([]byte, 16)
	rand.Read(uuid)
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// isValidUUID verifica si un string es UUID válido
func isValidUUID(s string) bool {
	pattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}

// buildClaudeArgs construye argumentos para claude CLI
func buildClaudeArgs(cfg TerminalConfig) []string {
	var args []string

	if cfg.Resume {
		args = append(args, "--resume", cfg.ID)
	} else if cfg.Continue {
		args = append(args, "--continue")
	} else if cfg.ID != "" {
		args = append(args, "--session-id", cfg.ID)
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	if cfg.SystemPrompt != "" {
		args = append(args, "--system-prompt", cfg.SystemPrompt)
	}

	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}

	if len(cfg.AllowedTools) > 0 {
		args = append(args, "--allowed-tools")
		args = append(args, cfg.AllowedTools...)
	}

	if len(cfg.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools")
		args = append(args, cfg.DisallowedTools...)
	}

	if len(cfg.AdditionalDirs) > 0 {
		args = append(args, "--add-dir")
		args = append(args, cfg.AdditionalDirs...)
	}

	if cfg.DangerouslySkip {
		args = append(args, "--dangerously-skip-permissions")
	}

	return args
}

// Create crea una nueva terminal
func (s *TerminalService) Create(cfg TerminalConfig) (*TerminalInfo, error) {
	// Validar path
	if err := ValidatePath(cfg.WorkDir, s.allowedPathPrefixes); err != nil {
		return nil, err
	}

	// Verificar directorio
	info, err := os.Stat(cfg.WorkDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("directorio invalido: %s", cfg.WorkDir)
	}

	// Generar UUID si no se proporciona
	if cfg.ID == "" || !isValidUUID(cfg.ID) {
		cfg.ID = generateUUID()
	}

	// Nombre por defecto
	if cfg.Name == "" {
		cfg.Name = filepath.Base(cfg.WorkDir)
	}

	// Tipo por defecto
	if cfg.Type == "" {
		cfg.Type = "claude"
	}

	// Verificar si ya existe activa
	s.mu.RLock()
	if _, exists := s.terminals[cfg.ID]; exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("terminal %s ya esta activa", cfg.ID)
	}
	s.mu.RUnlock()

	// Construir comando
	var cmd *exec.Cmd
	if cfg.Type == "terminal" {
		shell := GetDefaultShell()
		if cfg.Command != "" {
			args := GetShellExecArgs(cfg.Command)
			cmd = exec.Command(shell, args...)
		} else {
			args := GetShellArgs()
			cmd = exec.Command(shell, args...)
		}
	} else {
		args := buildClaudeArgs(cfg)
		cmd = exec.Command("claude", args...)
	}
	cmd.Dir = cfg.WorkDir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Iniciar PTY
	starter := NewPTYStarter()
	ptyInstance, err := starter.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("error iniciando PTY: %v", err)
	}

	// Crear terminal según tipo
	var terminal Terminal
	if cfg.Type == "claude" {
		tc := NewTerminalClaude(cfg.ID, cfg.Name, cfg.WorkDir, cfg)
		tc.SetCmd(cmd)
		tc.SetPty(ptyInstance)
		tc.SetScreen(NewScreenState(80, 24))
		tc.SetClaudeScreen(NewClaudeAwareScreenHandler(80, 24))
		tc.MarkActive()
		terminal = tc

		// Configurar callbacks
		s.setupClaudeCallbacksNew(tc)
		logger.Debug("TerminalClaude creada", "terminal_id", cfg.ID)
	} else {
		tr := NewTerminalRaw(cfg.ID, cfg.Name, cfg.WorkDir, cfg)
		tr.SetCmd(cmd)
		tr.SetPty(ptyInstance)
		tr.SetScreen(NewScreenState(80, 24))
		tr.Start()
		terminal = tr
		logger.Debug("TerminalRaw creada", "terminal_id", cfg.ID)
	}

	s.mu.Lock()
	s.terminals[cfg.ID] = terminal
	s.mu.Unlock()

	// Guardar
	s.savedMu.Lock()
	s.saved[cfg.ID] = &SavedTerminal{
		ID:           cfg.ID,
		Name:         cfg.Name,
		WorkDir:      cfg.WorkDir,
		SessionID:    cfg.ID,
		Type:         cfg.Type,
		Model:        cfg.Model,
		CreatedAt:    time.Now(),
		LastAccessAt: time.Now(),
		Status:       "running",
		Config:       cfg,
	}
	s.savedMu.Unlock()
	s.persistSaved()

	// Goroutine para leer output
	go s.readLoopNew(terminal)

	// Goroutine para detectar terminación
	go func() {
		cmd.Wait()
		s.cleanupNew(terminal)
	}()

	logger.Get().Terminal("created", cfg.ID, "name", cfg.Name, "work_dir", cfg.WorkDir, "type", cfg.Type)

	return s.toTerminalInfoNew(terminal, true), nil
}

// readLoopNew lee output del PTY usando interface
func (s *TerminalService) readLoopNew(t Terminal) {
	pty := t.GetPty()
	if pty == nil {
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				logger.Error("Error leyendo PTY", "terminal_id", t.GetID(), "error", err)
			}
			break
		}

		// Alimentar screen según tipo
		switch term := t.(type) {
		case *TerminalRaw:
			term.FeedScreen(buf[:n])
		case *TerminalClaude:
			term.FeedScreen(buf[:n])
		}

		t.Broadcast(buf[:n])
	}
}

// ClaudeEventMessage representa un mensaje de evento de Claude enviado via WebSocket
type ClaudeEventMessage struct {
	Type      string      `json:"type"`
	EventType string      `json:"event_type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// StateChangeData datos de cambio de estado
type StateChangeData struct {
	OldState string `json:"old_state"`
	NewState string `json:"new_state"`
}

// PermissionData datos de solicitud de permiso
type PermissionData struct {
	Tool string `json:"tool"`
}

// SlashCommandData datos de slash command
type SlashCommandData struct {
	Command string `json:"command"`
	Args    string `json:"args"`
}

// ToolUseData datos de uso de herramienta
type ToolUseData struct {
	Tool  string `json:"tool"`
	Phase string `json:"phase"`
}

// setupClaudeCallbacksNew configura callbacks para TerminalClaude
func (s *TerminalService) setupClaudeCallbacksNew(tc *TerminalClaude) {
	cs := tc.GetClaudeScreen()
	if cs == nil {
		return
	}

	cs.OnStateChange = func(old, new ClaudeState) {
		logger.Debug("Claude state change", "terminal_id", tc.GetID(), "old", old, "new", new)
		tc.BroadcastClaudeEvent("state", StateChangeData{
			OldState: string(old),
			NewState: string(new),
		})
	}

	cs.OnPermissionPrompt = func(tool string) {
		logger.Debug("Claude permission prompt", "terminal_id", tc.GetID(), "tool", tool)
		tc.BroadcastClaudeEvent("permission", PermissionData{Tool: tool})
	}

	cs.OnSlashCommand = func(cmd string, args string) {
		logger.Debug("Claude slash command", "terminal_id", tc.GetID(), "command", cmd, "args", args)
		tc.BroadcastClaudeEvent("command", SlashCommandData{Command: cmd, Args: args})
	}

	cs.OnCheckpoint = func(cp Checkpoint) {
		logger.Debug("Claude checkpoint", "terminal_id", tc.GetID(), "checkpoint_id", cp.ID)
		tc.BroadcastClaudeEvent("checkpoint", cp)
	}

	cs.OnToolUse = func(tool string, phase string) {
		logger.Debug("Claude tool use", "terminal_id", tc.GetID(), "tool", tool, "phase", phase)
		tc.BroadcastClaudeEvent("tool", ToolUseData{Tool: tool, Phase: phase})
	}
}

// cleanupNew limpia recursos de una terminal
func (s *TerminalService) cleanupNew(t Terminal) {
	// Notificar y cerrar clientes
	clients := make(map[*websocket.Conn]bool)
	switch term := t.(type) {
	case *TerminalRaw:
		clients = term.GetClients()
		term.ClearClients()
	case *TerminalClaude:
		clients = term.GetClients()
		term.ClearClients()
	}

	for client := range clients {
		client.WriteJSON(map[string]string{
			"type":    "closed",
			"message": "Terminal terminada",
		})
		client.Close()
	}

	// Cerrar PTY
	if pty := t.GetPty(); pty != nil {
		pty.Close()
	}

	id := t.GetID()

	s.mu.Lock()
	delete(s.terminals, id)
	s.mu.Unlock()

	// Actualizar estado guardado
	s.savedMu.Lock()
	if saved, ok := s.saved[id]; ok {
		saved.Status = "stopped"
		saved.LastAccessAt = time.Now()

		// Guardar estado Claude si aplica
		if tc, ok := t.(*TerminalClaude); ok {
			saved.ClaudeState = tc.GetClaudeStateSnapshot()
		}
	}
	s.savedMu.Unlock()
	s.persistSaved()

	if s.onTerminalEnd != nil {
		s.onTerminalEnd(id)
	}

	logger.Get().Terminal("terminated", id)
}

// List lista todas las terminales
func (s *TerminalService) List() []TerminalInfo {
	var list []TerminalInfo

	// Terminales activas
	s.mu.RLock()
	activeIDs := make(map[string]bool)
	for _, t := range s.terminals {
		activeIDs[t.GetID()] = true
		list = append(list, *s.toTerminalInfoNew(t, true))
	}
	s.mu.RUnlock()

	// Terminales guardadas no activas
	s.savedMu.RLock()
	for _, t := range s.saved {
		if !activeIDs[t.ID] {
			list = append(list, TerminalInfo{
				ID:           t.ID,
				Name:         t.Name,
				WorkDir:      t.WorkDir,
				SessionID:    t.SessionID,
				Type:         t.Type,
				Status:       t.Status,
				Model:        t.Model,
				Active:       false,
				CanResume:    t.Type == "claude",
				CreatedAt:    t.CreatedAt,
				LastAccessAt: t.LastAccessAt,
				ClaudeState:  t.ClaudeState,
			})
		}
	}
	s.savedMu.RUnlock()

	return list
}

// Get obtiene una terminal
func (s *TerminalService) Get(id string) (*TerminalInfo, error) {
	// Buscar en activas
	s.mu.RLock()
	if t, ok := s.terminals[id]; ok {
		s.mu.RUnlock()
		return s.toTerminalInfoNew(t, true), nil
	}
	s.mu.RUnlock()

	// Buscar en guardadas
	s.savedMu.RLock()
	if t, ok := s.saved[id]; ok {
		s.savedMu.RUnlock()
		return &TerminalInfo{
			ID:           t.ID,
			Name:         t.Name,
			WorkDir:      t.WorkDir,
			SessionID:    t.SessionID,
			Type:         t.Type,
			Status:       t.Status,
			Model:        t.Model,
			Active:       false,
			CanResume:    t.Type == "claude",
			CreatedAt:    t.CreatedAt,
			LastAccessAt: t.LastAccessAt,
			ClaudeState:  t.ClaudeState,
		}, nil
	}
	s.savedMu.RUnlock()

	return nil, fmt.Errorf("terminal no encontrada: %s", id)
}

// Resume reanuda una terminal
func (s *TerminalService) Resume(id string) (*TerminalInfo, error) {
	s.savedMu.RLock()
	saved, ok := s.saved[id]
	s.savedMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada: %s", id)
	}

	cfg := saved.Config
	cfg.Resume = true

	return s.Create(cfg)
}

// Kill termina una terminal
func (s *TerminalService) Kill(id string) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada o no activa: %s", id)
	}

	return terminal.Kill()
}

// Delete elimina una terminal guardada
func (s *TerminalService) Delete(id string) error {
	s.mu.RLock()
	if _, ok := s.terminals[id]; ok {
		s.mu.RUnlock()
		return fmt.Errorf("no se puede eliminar terminal activa: %s", id)
	}
	s.mu.RUnlock()

	s.savedMu.Lock()
	delete(s.saved, id)
	s.savedMu.Unlock()
	s.persistSaved()

	return nil
}

// Write escribe datos a una terminal
func (s *TerminalService) Write(id string, data []byte) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	_, err := terminal.Write(data)
	return err
}

// Resize redimensiona una terminal
func (s *TerminalService) Resize(id string, rows, cols uint16) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	return terminal.Resize(cols, rows)
}

// Pause pausa una terminal Claude
func (s *TerminalService) Pause(id string) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return fmt.Errorf("pause solo disponible para terminales claude: %s", id)
	}

	return tc.Pause()
}

// ResumeFromPause reanuda una terminal Claude pausada
func (s *TerminalService) ResumeFromPause(id string) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return fmt.Errorf("resume solo disponible para terminales claude: %s", id)
	}

	return tc.Resume()
}

// Archive archiva una terminal Claude
func (s *TerminalService) Archive(id string) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return fmt.Errorf("archive solo disponible para terminales claude: %s", id)
	}

	return tc.Archive()
}

// GetClaudeState retorna el estado de Claude para una terminal
func (s *TerminalService) GetClaudeState(id string) (*ClaudeStateInfo, error) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada o no activa: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return nil, fmt.Errorf("claude state no disponible (terminal no es de tipo claude): %s", id)
	}

	return tc.GetClaudeState(), nil
}

// GetClaudeCheckpoints retorna los checkpoints de una terminal Claude
func (s *TerminalService) GetClaudeCheckpoints(id string) ([]Checkpoint, error) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return nil, fmt.Errorf("checkpoints no disponibles (terminal no es de tipo claude): %s", id)
	}

	return tc.GetCheckpoints(), nil
}

// GetClaudeEvents retorna el historial de eventos de una terminal Claude
func (s *TerminalService) GetClaudeEvents(id string) ([]HookEvent, error) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return nil, fmt.Errorf("eventos no disponibles (terminal no es de tipo claude): %s", id)
	}

	return tc.GetEvents(), nil
}

// AddClaudeCheckpoint agrega un checkpoint manualmente
func (s *TerminalService) AddClaudeCheckpoint(id string, checkpointID string, tool string, files []string) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return fmt.Errorf("terminal no es de tipo claude: %s", id)
	}

	tc.AddCheckpoint(checkpointID, tool, files)
	return nil
}

// AddClaudeEvent agrega un evento manualmente
func (s *TerminalService) AddClaudeEvent(id string, eventType HookEventType, tool string, data interface{}) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return fmt.Errorf("terminal no es de tipo claude: %s", id)
	}

	tc.AddEvent(eventType, tool, data)
	return nil
}

// GetSnapshot retorna el estado actual de la pantalla de una terminal
func (s *TerminalService) GetSnapshot(id string) (*TerminalSnapshot, error) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada o no activa: %s", id)
	}

	return terminal.GetSnapshot(), nil
}

// GetTerminalState retorna el estado de máquina de estados de una terminal Claude
func (s *TerminalService) GetTerminalState(id string) (*ClaudeStateSnapshot, error) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return nil, fmt.Errorf("state no disponible para terminales raw: %s", id)
	}

	return tc.GetClaudeStateSnapshot(), nil
}

// GetMessages retorna métricas de mensajes de una terminal Claude
func (s *TerminalService) GetMessages(id string) (map[string]int, error) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada: %s", id)
	}

	tc, ok := terminal.(*TerminalClaude)
	if !ok {
		return nil, fmt.Errorf("messages no disponible para terminales raw: %s", id)
	}

	return map[string]int{
		"message_count":      tc.GetMessageCount(),
		"user_messages":      tc.GetUserMessages(),
		"assistant_messages": tc.GetAssistantMessages(),
	}, nil
}

// AddClient añade un cliente WebSocket
func (s *TerminalService) AddClient(id string, conn *websocket.Conn) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	terminal.AddClient(conn)
	logger.Get().WebSocket("connected", id)
	return nil
}

// RemoveClient elimina un cliente WebSocket
func (s *TerminalService) RemoveClient(id string, conn *websocket.Conn) {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return
	}

	terminal.RemoveClient(conn)
	logger.Get().WebSocket("disconnected", id)
}

// IsActive verifica si una terminal está activa
func (s *TerminalService) IsActive(id string) bool {
	s.mu.RLock()
	_, ok := s.terminals[id]
	s.mu.RUnlock()
	return ok
}

// toTerminalInfoNew convierte Terminal interface a TerminalInfo
func (s *TerminalService) toTerminalInfoNew(t Terminal, active bool) *TerminalInfo {
	info := &TerminalInfo{
		ID:        t.GetID(),
		Name:      t.GetName(),
		WorkDir:   t.GetWorkDir(),
		SessionID: t.GetSessionID(),
		Type:      t.GetType(),
		Status:    t.GetStatus(),
		Model:     t.GetModel(),
		Active:    active,
		Clients:   t.GetClientCount(),
		CanResume: t.GetType() == "claude",
		StartedAt: t.GetStartedAt(),
	}

	// Añadir estado Claude si aplica
	if tc, ok := t.(*TerminalClaude); ok {
		info.ClaudeState = tc.GetClaudeStateSnapshot()
	}

	return info
}

// ListDirectory lista contenido de un directorio
func ListDirectory(path string, allowedPrefixes []string) ([]DirectoryEntry, error) {
	if path == "" {
		path = "/"
	}
	path = filepath.Clean(path)

	if err := ValidatePath(path, allowedPrefixes); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []DirectoryEntry

	if path != "/" {
		result = append(result, DirectoryEntry{
			Name:  "..",
			Path:  filepath.Dir(path),
			IsDir: true,
		})
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, ".") && !entry.IsDir() {
			continue
		}

		result = append(result, DirectoryEntry{
			Name:  name,
			Path:  filepath.Join(path, name),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// GetSavedConfig obtiene la config guardada de una terminal
func (s *TerminalService) GetSavedConfig(id string) (*TerminalConfig, error) {
	s.savedMu.RLock()
	saved, ok := s.saved[id]
	s.savedMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("terminal no encontrada: %s", id)
	}

	return &saved.Config, nil
}

// MarkAsImported marca una terminal como importada
func (s *TerminalService) MarkAsImported(sessionID, name, workDir string) {
	s.savedMu.Lock()
	if _, exists := s.saved[sessionID]; !exists {
		s.saved[sessionID] = &SavedTerminal{
			ID:           sessionID,
			Name:         name,
			WorkDir:      workDir,
			SessionID:    sessionID,
			Type:         "claude",
			CreatedAt:    time.Now(),
			LastAccessAt: time.Now(),
			Status:       "stopped",
			Config: TerminalConfig{
				ID:      sessionID,
				Name:    name,
				WorkDir: workDir,
				Type:    "claude",
			},
		}
	}
	s.savedMu.Unlock()
	s.persistSaved()
}

// RemoveFromSaved elimina una terminal del registro guardado
func (s *TerminalService) RemoveFromSaved(id string) {
	s.savedMu.Lock()
	delete(s.saved, id)
	s.savedMu.Unlock()
	s.persistSaved()
}

// ShutdownAll termina todas las terminales activas
func (s *TerminalService) ShutdownAll() {
	s.ShutdownAllWithTimeout(5 * time.Second)
}

// ShutdownAllWithTimeout termina todas las terminales con timeout
func (s *TerminalService) ShutdownAllWithTimeout(timeout time.Duration) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.terminals))
	for id := range s.terminals {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	if len(ids) == 0 {
		logger.Info("No hay terminales activas para terminar")
		return
	}

	logger.Info("Terminando terminales activas", "count", len(ids))

	done := make(chan string, len(ids))
	signaler := NewProcessSignaler()

	for _, id := range ids {
		s.mu.RLock()
		terminal, ok := s.terminals[id]
		s.mu.RUnlock()

		if !ok {
			done <- id
			continue
		}

		// Notificar a clientes
		var clients map[*websocket.Conn]bool
		switch t := terminal.(type) {
		case *TerminalRaw:
			clients = t.GetClients()
		case *TerminalClaude:
			clients = t.GetClients()
		}

		for client := range clients {
			client.WriteJSON(map[string]string{
				"type":    "shutdown",
				"message": "Servidor terminando",
			})
		}

		// Terminar proceso
		cmd := terminal.GetCmd()
		if execCmd, ok := cmd.(*exec.Cmd); ok && execCmd != nil && execCmd.Process != nil {
			logger.Get().Terminal("shutdown", id)
			signaler.Terminate(execCmd)

			go func(c *exec.Cmd, termID string) {
				if c != nil && c.Process != nil {
					c.Wait()
				}
				done <- termID
			}(execCmd, id)
		} else {
			done <- id
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	terminated := 0
	for terminated < len(ids) {
		select {
		case termID := <-done:
			terminated++
			logger.Debug("Terminal terminada", "id", termID, "terminated", terminated, "total", len(ids))
		case <-timer.C:
			s.mu.RLock()
			remaining := len(s.terminals)
			s.mu.RUnlock()

			if remaining > 0 {
				logger.Warn("Timeout esperando terminación, forzando kill", "remaining", remaining)
				s.mu.RLock()
				for _, terminal := range s.terminals {
					if cmd := terminal.GetCmd(); cmd != nil {
						if execCmd, ok := cmd.(*exec.Cmd); ok {
							signaler.Kill(execCmd)
						}
					}
				}
				s.mu.RUnlock()
			}
			return
		}
	}

	logger.Info("Todas las terminales terminadas correctamente")
}

// PersistState persiste el estado actual
func (s *TerminalService) PersistState() {
	s.persistSaved()
	logger.Debug("Estado de terminales persistido")
}
