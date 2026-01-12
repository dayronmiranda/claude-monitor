package services

import (
	"os/exec"
	"os"
	"time"
)

// JobState representa el estado actual del trabajo
type JobState string

const (
	JobStateCreated  JobState = "created"
	JobStateStarting JobState = "starting"
	JobStateActive   JobState = "active"
	JobStatePaused   JobState = "paused"
	JobStateStopped  JobState = "stopped"
	JobStateArchived JobState = "archived"
	JobStateError    JobState = "error"
	JobStateDeleted  JobState = "deleted"
)

// JobError detalles de error
type JobError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	RetryCount int       `json:"retry_count"`
}

// Job representa un trabajo unificado (Terminal + Session)
type Job struct {
	// Identidad
	ID          string    `json:"id"`           // UUID único
	SessionID   string    `json:"session_id"`   // Mismo que ID (compat)
	ProjectPath string    `json:"project_path"` // Proyecto padre
	RealPath    string    `json:"real_path"`    // Path decodificado

	// Configuración
	Name        string `json:"name"`         // Nombre personalizado
	Description string `json:"description"` // Descripción opcional
	WorkDir     string `json:"work_dir"`     // Directorio de ejecución
	Type        string `json:"type"`         // "claude" o "terminal"
	Model       string `json:"model"`        // Modelo Claude

	// Estado
	State       JobState   `json:"state"`        // Estado actual
	CreatedAt   time.Time  `json:"created_at"`   // Creación
	StartedAt   *time.Time `json:"started_at"`   // Primera ejecución
	PausedAt    *time.Time `json:"paused_at"`    // Última pausa
	StoppedAt   *time.Time `json:"stopped_at"`   // Última parada
	ArchivedAt  *time.Time `json:"archived_at"`  // Archivado

	// Métricas de conversación
	MessageCount      int `json:"message_count"`
	UserMessages      int `json:"user_messages"`
	AssistantMessages int `json:"assistant_messages"`

	// Contadores de ciclo de vida
	PauseCount  int `json:"pause_count"`   // Cuántas pausas
	ResumeCount int `json:"resume_count"`  // Cuántas reanudaciones

	// Recursos (si activo)
	PtyID     string `json:"pty_id,omitempty"`       // ID del PTY
	ProcessID int    `json:"process_id,omitempty"`   // PID
	Clients   int    `json:"clients,omitempty"`      // WebSocket clients
	MemoryMB  int    `json:"memory_mb,omitempty"`    // Uso memoria

	// Error handling
	Error *JobError `json:"error,omitempty"`

	// Flags
	IsArchived   bool `json:"is_archived"`
	AutoArchived bool `json:"auto_archived"`

	// Datos internos (no exponer en API)
	Cmd *exec.Cmd `json:"-"`
	Pty *os.File  `json:"-"`
}

// SavedJob representa un job persistido
type SavedJob struct {
	ID                string     `json:"id"`
	SessionID         string     `json:"session_id"`
	ProjectPath       string     `json:"project_path"`
	RealPath          string     `json:"real_path"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	WorkDir           string     `json:"work_dir"`
	Type              string     `json:"type"`
	Model             string     `json:"model"`
	State             JobState   `json:"state"`
	CreatedAt         time.Time  `json:"created_at"`
	StartedAt         *time.Time `json:"started_at"`
	PausedAt          *time.Time `json:"paused_at"`
	StoppedAt         *time.Time `json:"stopped_at"`
	ArchivedAt        *time.Time `json:"archived_at"`
	MessageCount      int        `json:"message_count"`
	UserMessages      int        `json:"user_messages"`
	AssistantMessages int        `json:"assistant_messages"`
	PauseCount        int        `json:"pause_count"`
	ResumeCount       int        `json:"resume_count"`
	Error             *JobError  `json:"error,omitempty"`
	IsArchived        bool       `json:"is_archived"`
	AutoArchived      bool       `json:"auto_archived"`
}

// JobConfig configuración para crear un nuevo job
type JobConfig struct {
	ID          string // UUID opcional, se genera si está vacío
	Name        string
	Description string
	WorkDir     string
	Type        string // "claude" o "terminal"
	ProjectPath string
	RealPath    string
	Model       string
}

// ToSavedJob convierte Job a SavedJob (para persistencia)
func (j *Job) ToSavedJob() *SavedJob {
	return &SavedJob{
		ID:                j.ID,
		SessionID:         j.SessionID,
		ProjectPath:       j.ProjectPath,
		RealPath:          j.RealPath,
		Name:              j.Name,
		Description:       j.Description,
		WorkDir:           j.WorkDir,
		Type:              j.Type,
		Model:             j.Model,
		State:             j.State,
		CreatedAt:         j.CreatedAt,
		StartedAt:         j.StartedAt,
		PausedAt:          j.PausedAt,
		StoppedAt:         j.StoppedAt,
		ArchivedAt:        j.ArchivedAt,
		MessageCount:      j.MessageCount,
		UserMessages:      j.UserMessages,
		AssistantMessages: j.AssistantMessages,
		PauseCount:        j.PauseCount,
		ResumeCount:       j.ResumeCount,
		Error:             j.Error,
		IsArchived:        j.IsArchived,
		AutoArchived:      j.AutoArchived,
	}
}

// FromSavedJob convierte SavedJob a Job
func FromSavedJob(s *SavedJob) *Job {
	return &Job{
		ID:                s.ID,
		SessionID:         s.SessionID,
		ProjectPath:       s.ProjectPath,
		RealPath:          s.RealPath,
		Name:              s.Name,
		Description:       s.Description,
		WorkDir:           s.WorkDir,
		Type:              s.Type,
		Model:             s.Model,
		State:             s.State,
		CreatedAt:         s.CreatedAt,
		StartedAt:         s.StartedAt,
		PausedAt:          s.PausedAt,
		StoppedAt:         s.StoppedAt,
		ArchivedAt:        s.ArchivedAt,
		MessageCount:      s.MessageCount,
		UserMessages:      s.UserMessages,
		AssistantMessages: s.AssistantMessages,
		PauseCount:        s.PauseCount,
		ResumeCount:       s.ResumeCount,
		Error:             s.Error,
		IsArchived:        s.IsArchived,
		AutoArchived:      s.AutoArchived,
	}
}
