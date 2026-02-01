package services

import (
	"time"

	"github.com/gorilla/websocket"
)

// TerminalSnapshot representa el estado completo de una pantalla de terminal
type TerminalSnapshot struct {
	Content           string   `json:"content"`
	Display           []string `json:"display"`
	CursorX           int      `json:"cursor_x"`
	CursorY           int      `json:"cursor_y"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	InAlternateScreen bool     `json:"in_alternate_screen"`
	History           []string `json:"history,omitempty"`
}

// Terminal es la interfaz común para todos los tipos de terminal
type Terminal interface {
	// Identidad
	GetID() string
	GetType() string
	GetName() string
	GetWorkDir() string
	GetSessionID() string
	GetStatus() string
	GetStartedAt() time.Time

	// Ciclo de vida
	Start() error
	Stop() error
	Kill() error
	IsActive() bool

	// I/O
	Write(data []byte) (int, error)
	Resize(cols, rows uint16) error

	// Estado de pantalla
	GetSnapshot() *TerminalSnapshot
	GetScreen() *ScreenState

	// Clientes WebSocket
	AddClient(conn *websocket.Conn)
	RemoveClient(conn *websocket.Conn)
	GetClientCount() int
	Broadcast(data []byte)

	// PTY
	GetPty() PTY
	GetCmd() interface{} // *exec.Cmd

	// Config
	GetConfig() TerminalConfig
	GetModel() string
}

// ClaudeTerminal es la interfaz extendida para terminales Claude
// Incluye máquina de estados, checkpoints, eventos y métricas
type ClaudeTerminal interface {
	Terminal

	// Estado de Claude
	GetClaudeScreen() *ClaudeAwareScreenHandler
	GetClaudeState() *ClaudeStateInfo

	// Checkpoints y eventos
	GetCheckpoints() []Checkpoint
	GetEvents() []HookEvent
	AddCheckpoint(id, tool string, files []string)
	AddEvent(eventType HookEventType, tool string, data interface{})

	// Máquina de estados (migrada de Job)
	GetState() TerminalState
	Pause() error
	Resume() error
	Archive() error

	// Métricas de conversación
	GetMessageCount() int
	GetUserMessages() int
	GetAssistantMessages() int

	// Timestamps de estado
	GetPausedAt() *time.Time
	GetStoppedAt() *time.Time
	GetArchivedAt() *time.Time

	// Contadores
	GetPauseCount() int
	GetResumeCount() int

	// Error handling
	GetError() *TerminalError
	SetError(err *TerminalError)
}

// TerminalState representa el estado del ciclo de vida de una terminal Claude
type TerminalState string

const (
	TerminalStateCreated  TerminalState = "created"
	TerminalStateStarting TerminalState = "starting"
	TerminalStateActive   TerminalState = "active"
	TerminalStatePaused   TerminalState = "paused"
	TerminalStateStopped  TerminalState = "stopped"
	TerminalStateArchived TerminalState = "archived"
	TerminalStateError    TerminalState = "error"
)

// TerminalError detalles de error para terminal Claude
type TerminalError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	RetryCount int       `json:"retry_count"`
}

// TerminalBase contiene campos comunes para todas las terminales
type TerminalBase struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	WorkDir   string                   `json:"work_dir"`
	SessionID string                   `json:"session_id,omitempty"`
	Status    string                   `json:"status"`
	Type      string                   `json:"type"`
	StartedAt time.Time                `json:"started_at"`
	Config    TerminalConfig           `json:"-"`
	Clients   map[*websocket.Conn]bool `json:"-"`
}

// ClaudeStateSnapshot representa el estado extendido para TerminalInfo
type ClaudeStateSnapshot struct {
	State             TerminalState `json:"state"`
	MessageCount      int           `json:"message_count"`
	UserMessages      int           `json:"user_messages"`
	AssistantMessages int           `json:"assistant_messages"`
	PauseCount        int           `json:"pause_count"`
	ResumeCount       int           `json:"resume_count"`
	PausedAt          *time.Time    `json:"paused_at,omitempty"`
	StoppedAt         *time.Time    `json:"stopped_at,omitempty"`
	ArchivedAt        *time.Time    `json:"archived_at,omitempty"`
	Error             *TerminalError `json:"error,omitempty"`
}
