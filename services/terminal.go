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
	"syscall"
	"time"
	"unsafe"

	"claude-monitor/pkg/logger"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// TerminalService gestiona las terminales PTY
type TerminalService struct {
	terminals           map[string]*Terminal
	mu                  sync.RWMutex
	saved               map[string]*SavedTerminal
	savedMu             sync.RWMutex
	sessionsFile        string
	onTerminalEnd       func(id string)
	allowedPathPrefixes []string
}

// Terminal representa una terminal PTY activa
type Terminal struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	WorkDir   string                   `json:"work_dir"`
	SessionID string                   `json:"session_id,omitempty"`
	Status    string                   `json:"status"`
	Type      string                   `json:"type"` // "claude" o "terminal"
	StartedAt time.Time                `json:"started_at"`
	Cmd       *exec.Cmd                `json:"-"`
	Pty       *os.File                 `json:"-"`
	Clients   map[*websocket.Conn]bool `json:"-"`
	ClientsMu sync.RWMutex             `json:"-"`
	Config    TerminalConfig           `json:"-"`
	Screen    *ScreenState             `json:"-"` // Estado de pantalla virtual (go-ansiterm)
}

// SavedTerminal terminal guardada para persistencia
type SavedTerminal struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	WorkDir      string         `json:"work_dir"`
	SessionID    string         `json:"session_id,omitempty"`
	Type         string         `json:"type"`
	Model        string         `json:"model,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	LastAccessAt time.Time      `json:"last_access_at"`
	Status       string         `json:"status"`
	Config       TerminalConfig `json:"config"`
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
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	WorkDir      string    `json:"work_dir"`
	SessionID    string    `json:"session_id,omitempty"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Model        string    `json:"model,omitempty"`
	Active       bool      `json:"active"`
	Clients      int       `json:"clients"`
	CanResume    bool      `json:"can_resume"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	LastAccessAt time.Time `json:"last_access_at,omitempty"`
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
		terminals:           make(map[string]*Terminal),
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

	// Escritura atómica: escribir a archivo temporal y renombrar
	if err := atomicWriteFile(s.sessionsFile, data, 0600); err != nil {
		logger.Error("Error guardando terminales", "error", err)
	}
}

// atomicWriteFile escribe un archivo de forma atómica
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// Crear archivo temporal en el mismo directorio
	tmpPath := path + ".tmp"

	// Escribir al archivo temporal
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	// Escribir datos
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	// Sync para asegurar que los datos están en disco
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Rename atómico
	return os.Rename(tmpPath, path)
}

// PathValidator función de validación de paths
type PathValidator func(path string) error

// ValidatePath verifica que un path sea válido y esté en prefijos permitidos
func ValidatePath(path string, allowedPrefixes []string) error {
	// Limpiar el path
	cleaned := filepath.Clean(path)

	// Debe ser absoluto
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("path debe ser absoluto: %s", path)
	}

	// Detectar path traversal
	if strings.Contains(path, "..") {
		logger.Warn("Intento de path traversal detectado", "path", path)
		return fmt.Errorf("path traversal no permitido")
	}

	// Si no hay prefijos configurados, permitir todos
	if len(allowedPrefixes) == 0 {
		return nil
	}

	// Verificar que está dentro de los prefijos permitidos
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
		shell := "/bin/bash"
		if cfg.Command != "" {
			cmd = exec.Command(shell, "-c", cfg.Command)
		} else {
			cmd = exec.Command(shell, "-l")
		}
	} else {
		args := buildClaudeArgs(cfg)
		cmd = exec.Command("claude", args...)
	}
	cmd.Dir = cfg.WorkDir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Iniciar PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("error iniciando PTY: %v", err)
	}

	terminal := &Terminal{
		ID:        cfg.ID,
		Name:      cfg.Name,
		WorkDir:   cfg.WorkDir,
		SessionID: cfg.ID,
		Status:    "running",
		Type:      cfg.Type,
		StartedAt: time.Now(),
		Cmd:       cmd,
		Pty:       ptmx,
		Clients:   make(map[*websocket.Conn]bool),
		Config:    cfg,
		Screen:    NewScreenState(80, 24), // Pantalla virtual para tracking de estado
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
	go s.readLoop(terminal)

	// Goroutine para detectar terminación
	go func() {
		cmd.Wait()
		s.cleanup(terminal)
	}()

	logger.Get().Terminal("created", cfg.ID, "name", cfg.Name, "work_dir", cfg.WorkDir, "type", cfg.Type)

	return s.toTerminalInfo(terminal, true), nil
}

// readLoop lee output del PTY
func (s *TerminalService) readLoop(t *Terminal) {
	buf := make([]byte, 4096)
	for {
		n, err := t.Pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				logger.Error("Error leyendo PTY", "terminal_id", t.ID, "error", err)
			}
			break
		}

		// Alimentar el estado de pantalla virtual con go-ansiterm
		if t.Screen != nil {
			if feedErr := t.Screen.Feed(buf[:n]); feedErr != nil {
				logger.Debug("Error feeding screen state", "terminal_id", t.ID, "error", feedErr)
			}
		}

		s.broadcast(t, buf[:n])
	}
}

// broadcast envía datos a todos los clientes
func (s *TerminalService) broadcast(t *Terminal, data []byte) {
	t.ClientsMu.RLock()
	defer t.ClientsMu.RUnlock()

	msg := map[string]string{
		"type": "output",
		"data": string(data),
	}

	for client := range t.Clients {
		client.WriteJSON(msg)
	}
}

// cleanup limpia recursos de una terminal
func (s *TerminalService) cleanup(t *Terminal) {
	t.ClientsMu.Lock()
	for client := range t.Clients {
		client.WriteJSON(map[string]string{
			"type":    "closed",
			"message": "Terminal terminada",
		})
		client.Close()
	}
	t.Clients = make(map[*websocket.Conn]bool)
	t.ClientsMu.Unlock()

	if t.Pty != nil {
		t.Pty.Close()
	}

	s.mu.Lock()
	delete(s.terminals, t.ID)
	s.mu.Unlock()

	// Actualizar estado
	s.savedMu.Lock()
	if saved, ok := s.saved[t.ID]; ok {
		saved.Status = "stopped"
		saved.LastAccessAt = time.Now()
	}
	s.savedMu.Unlock()
	s.persistSaved()

	if s.onTerminalEnd != nil {
		s.onTerminalEnd(t.ID)
	}

	logger.Get().Terminal("terminated", t.ID)
}

// List lista todas las terminales
func (s *TerminalService) List() []TerminalInfo {
	var list []TerminalInfo

	// Terminales activas
	s.mu.RLock()
	activeIDs := make(map[string]bool)
	for _, t := range s.terminals {
		activeIDs[t.ID] = true
		list = append(list, *s.toTerminalInfo(t, true))
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
		return s.toTerminalInfo(t, true), nil
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

	if terminal.Cmd != nil && terminal.Cmd.Process != nil {
		terminal.Cmd.Process.Signal(syscall.SIGTERM)
	}

	return nil
}

// Delete elimina una terminal guardada
func (s *TerminalService) Delete(id string) error {
	// Verificar que no esté activa
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

	_, err := terminal.Pty.Write(data)
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

	ws := struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{Row: rows, Col: cols}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		terminal.Pty.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return errno
	}

	// Actualizar también el estado de pantalla virtual
	if terminal.Screen != nil {
		terminal.Screen.Resize(int(cols), int(rows))
	}

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

	if terminal.Screen == nil {
		return nil, fmt.Errorf("screen state no disponible para terminal: %s", id)
	}

	cursorX, cursorY := terminal.Screen.GetCursor()
	width, height := terminal.Screen.GetSize()

	return &TerminalSnapshot{
		Content:           terminal.Screen.Snapshot(),
		Display:           terminal.Screen.GetDisplay(),
		CursorX:           cursorX,
		CursorY:           cursorY,
		Width:             width,
		Height:            height,
		InAlternateScreen: terminal.Screen.IsInAlternateScreen(),
		History:           terminal.Screen.GetHistoryLines(),
	}, nil
}

// TerminalSnapshot representa el estado completo de una pantalla de terminal
type TerminalSnapshot struct {
	Content           string   `json:"content"`             // Texto plano de la pantalla
	Display           []string `json:"display"`             // Líneas individuales
	CursorX           int      `json:"cursor_x"`            // Posición X del cursor
	CursorY           int      `json:"cursor_y"`            // Posición Y del cursor
	Width             int      `json:"width"`               // Ancho de la pantalla
	Height            int      `json:"height"`              // Alto de la pantalla
	InAlternateScreen bool     `json:"in_alternate_screen"` // Si está en modo vim/htop
	History           []string `json:"history,omitempty"`   // Historial de scroll
}

// AddClient añade un cliente WebSocket
func (s *TerminalService) AddClient(id string, conn *websocket.Conn) error {
	s.mu.RLock()
	terminal, ok := s.terminals[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal no encontrada: %s", id)
	}

	terminal.ClientsMu.Lock()
	terminal.Clients[conn] = true
	terminal.ClientsMu.Unlock()

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

	terminal.ClientsMu.Lock()
	delete(terminal.Clients, conn)
	terminal.ClientsMu.Unlock()

	logger.Get().WebSocket("disconnected", id)
}

// IsActive verifica si una terminal está activa
func (s *TerminalService) IsActive(id string) bool {
	s.mu.RLock()
	_, ok := s.terminals[id]
	s.mu.RUnlock()
	return ok
}

// toTerminalInfo convierte Terminal a TerminalInfo
func (s *TerminalService) toTerminalInfo(t *Terminal, active bool) *TerminalInfo {
	t.ClientsMu.RLock()
	clients := len(t.Clients)
	t.ClientsMu.RUnlock()

	return &TerminalInfo{
		ID:        t.ID,
		Name:      t.Name,
		WorkDir:   t.WorkDir,
		SessionID: t.SessionID,
		Type:      t.Type,
		Status:    t.Status,
		Model:     t.Config.Model,
		Active:    active,
		Clients:   clients,
		CanResume: t.Type == "claude",
		StartedAt: t.StartedAt,
	}
}

// ListDirectory lista contenido de un directorio
func ListDirectory(path string, allowedPrefixes []string) ([]DirectoryEntry, error) {
	if path == "" {
		path = "/"
	}
	path = filepath.Clean(path)

	// Validar path
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

// MarkAsImported marca una terminal como importada (para sincronizar con Claude sessions)
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

// ShutdownAll termina todas las terminales activas ordenadamente
func (s *TerminalService) ShutdownAll() {
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

	for _, id := range ids {
		s.mu.RLock()
		terminal, ok := s.terminals[id]
		s.mu.RUnlock()

		if !ok {
			continue
		}

		// Notificar a clientes
		terminal.ClientsMu.RLock()
		for client := range terminal.Clients {
			client.WriteJSON(map[string]string{
				"type":    "shutdown",
				"message": "Servidor terminando",
			})
		}
		terminal.ClientsMu.RUnlock()

		// Enviar SIGTERM al proceso
		if terminal.Cmd != nil && terminal.Cmd.Process != nil {
			logger.Get().Terminal("shutdown", id)
			terminal.Cmd.Process.Signal(syscall.SIGTERM)
		}
	}

	// Esperar un poco para que terminen
	time.Sleep(2 * time.Second)

	// Force kill si aún quedan
	s.mu.RLock()
	remaining := len(s.terminals)
	s.mu.RUnlock()

	if remaining > 0 {
		logger.Warn("Forzando terminación de terminales restantes", "count", remaining)
		s.mu.RLock()
		for _, terminal := range s.terminals {
			if terminal.Cmd != nil && terminal.Cmd.Process != nil {
				terminal.Cmd.Process.Kill()
			}
		}
		s.mu.RUnlock()
	}
}

// PersistState persiste el estado actual
func (s *TerminalService) PersistState() {
	s.persistSaved()
	logger.Debug("Estado de terminales persistido")
}
