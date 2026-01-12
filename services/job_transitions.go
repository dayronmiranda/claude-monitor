package services

import (
	"fmt"
	"time"
)

// Transition representa una transición de estado válida
type Transition struct {
	From   JobState
	To     JobState
	Event  string
	Guard  func(*Job) bool
	Action func(*Job) error
}

// TransitionTable tabla de transiciones válidas
var TransitionTable = []Transition{
	// CREATED → STARTING
	{
		From:   JobStateCreated,
		To:     JobStateStarting,
		Event:  "START",
		Guard:  canStart,
		Action: actionStart,
	},

	// CREATED → DELETED
	{
		From:   JobStateCreated,
		To:     JobStateDeleted,
		Event:  "DELETE",
		Guard:  nil,
		Action: actionDelete,
	},

	// STARTING → ACTIVE
	{
		From:   JobStateStarting,
		To:     JobStateActive,
		Event:  "READY",
		Guard:  processRunning,
		Action: actionReady,
	},

	// STARTING → ERROR
	{
		From:   JobStateStarting,
		To:     JobStateError,
		Event:  "FAILED",
		Guard:  nil,
		Action: actionFailed,
	},

	// ACTIVE → PAUSED
	{
		From:   JobStateActive,
		To:     JobStatePaused,
		Event:  "PAUSE",
		Guard:  nil,
		Action: actionPause,
	},

	// ACTIVE → STOPPED
	{
		From:   JobStateActive,
		To:     JobStateStopped,
		Event:  "STOP",
		Guard:  nil,
		Action: actionStop,
	},

	// ACTIVE → ERROR
	{
		From:   JobStateActive,
		To:     JobStateError,
		Event:  "ERROR",
		Guard:  nil,
		Action: actionError,
	},

	// PAUSED → ACTIVE
	{
		From:   JobStatePaused,
		To:     JobStateActive,
		Event:  "RESUME",
		Guard:  canResumePaused,
		Action: actionResumePaused,
	},

	// PAUSED → STOPPED
	{
		From:   JobStatePaused,
		To:     JobStateStopped,
		Event:  "STOP",
		Guard:  nil,
		Action: actionStop,
	},

	// PAUSED → ARCHIVED
	{
		From:   JobStatePaused,
		To:     JobStateArchived,
		Event:  "ARCHIVE",
		Guard:  nil,
		Action: actionArchive,
	},

	// STOPPED → STARTING (Resume)
	{
		From:   JobStateStopped,
		To:     JobStateStarting,
		Event:  "RESUME",
		Guard:  canResumeStopped,
		Action: actionResumeStopped,
	},

	// STOPPED → ARCHIVED
	{
		From:   JobStateStopped,
		To:     JobStateArchived,
		Event:  "ARCHIVE",
		Guard:  nil,
		Action: actionArchive,
	},

	// STOPPED → DELETED
	{
		From:   JobStateStopped,
		To:     JobStateDeleted,
		Event:  "DELETE",
		Guard:  nil,
		Action: actionDelete,
	},

	// ARCHIVED → STOPPED (Reopen)
	{
		From:   JobStateArchived,
		To:     JobStateStopped,
		Event:  "REOPEN",
		Guard:  nil,
		Action: actionReopen,
	},

	// ARCHIVED → DELETED
	{
		From:   JobStateArchived,
		To:     JobStateDeleted,
		Event:  "DELETE",
		Guard:  nil,
		Action: actionDelete,
	},

	// ERROR → STARTING (Retry)
	{
		From:   JobStateError,
		To:     JobStateStarting,
		Event:  "RETRY",
		Guard:  canRetry,
		Action: actionRetry,
	},

	// ERROR → DELETED (Discard)
	{
		From:   JobStateError,
		To:     JobStateDeleted,
		Event:  "DISCARD",
		Guard:  nil,
		Action: actionDelete,
	},
}

// ============================================================================
// GUARDS - Condiciones que deben cumplirse para las transiciones
// ============================================================================

// canStart valida que el job puede iniciarse
func canStart(j *Job) bool {
	if j.WorkDir == "" {
		return false
	}
	// En producción: verificar espacio en disco > 100MB
	// diskSpace, _ := getDiskSpace(j.WorkDir)
	// return diskSpace > 100*1024*1024
	return true
}

// processRunning verifica que el proceso está ejecutándose
func processRunning(j *Job) bool {
	return j.Cmd != nil && j.Cmd.Process != nil
}

// canResumePaused verifica que se puede reanudar desde PAUSED
func canResumePaused(j *Job) bool {
	if j.PausedAt == nil {
		return false
	}
	// No reanudar si han pasado > 24 horas
	return time.Since(*j.PausedAt) < 24*time.Hour
}

// canResumeStopped verifica que se puede reanudar desde STOPPED
func canResumeStopped(j *Job) bool {
	if j.StoppedAt == nil {
		return false
	}
	// Solo reanudar si han pasado < 7 días
	// y el contexto de sesión sigue siendo válido
	return time.Since(*j.StoppedAt) < 7*24*time.Hour
}

// canRetry verifica que se puede reintentar
func canRetry(j *Job) bool {
	if j.Error == nil {
		return false
	}
	// Máximo 3 intentos
	return j.Error.RetryCount < 3
}

// ============================================================================
// ACTIONS - Acciones que se ejecutan cuando ocurre una transición
// ============================================================================

// actionStart ejecuta acciones al transicionar a STARTING
func actionStart(j *Job) error {
	// En una implementación real:
	// - Crear PTY
	// - Iniciar proceso
	// - Set timeout para pasar a ERROR si toma > 5 segundos
	// Por ahora, solo loguear
	fmt.Printf("Job %s: Iniciando proceso...\n", j.ID)
	return nil
}

// actionReady ejecuta acciones al pasar a ACTIVE
func actionReady(j *Job) error {
	now := time.Now()
	j.StartedAt = &now

	// En una implementación real:
	// - Crear archivo JSONL
	// - Registrar en tabla de sesiones
	// - Iniciar monitoreo de recursos
	fmt.Printf("Job %s: Proceso activo\n", j.ID)
	return nil
}

// actionPause ejecuta acciones al pausar
func actionPause(j *Job) error {
	now := time.Now()
	j.PausedAt = &now
	j.PauseCount++

	// En una implementación real:
	// - Send SIGSTOP al proceso
	// - Mantener PTY abierto
	fmt.Printf("Job %s: Pausado (pauses: %d)\n", j.ID, j.PauseCount)
	return nil
}

// actionResumePaused ejecuta acciones al reanudar desde PAUSED a ACTIVE
func actionResumePaused(j *Job) error {
	j.ResumeCount++

	// En una implementación real:
	// - Send SIGCONT al proceso
	// - Update resume_time
	fmt.Printf("Job %s: Reanudado desde pausa (resumes: %d)\n", j.ID, j.ResumeCount)
	return nil
}

// actionStop ejecuta acciones al detener
func actionStop(j *Job) error {
	now := time.Now()
	j.StoppedAt = &now

	// En una implementación real:
	// - Send SIGTERM al proceso
	// - Close PTY
	// - Finalize JSONL (escribir metadata)
	fmt.Printf("Job %s: Detenido\n", j.ID)
	return nil
}

// actionResumeStopped ejecuta acciones al reanudar desde STOPPED a STARTING
func actionResumeStopped(j *Job) error {
	j.ResumeCount++

	// En una implementación real:
	// - Verificar que session_context es válido
	// - Reuse session_id (no crear uno nuevo)
	// - Append to JSONL (continuar conversación)
	fmt.Printf("Job %s: Reanudando desde detención (resumes: %d)\n", j.ID, j.ResumeCount)
	return nil
}

// actionArchive ejecuta acciones al archivar
func actionArchive(j *Job) error {
	now := time.Now()
	j.ArchivedAt = &now
	j.IsArchived = true

	// En una implementación real:
	// - Compress JSONL si > 1MB
	// - Move to archive storage
	// - Release memory cache
	fmt.Printf("Job %s: Archivado\n", j.ID)
	return nil
}

// actionReopen ejecuta acciones al reabrir desde ARCHIVED
func actionReopen(j *Job) error {
	j.IsArchived = false
	j.ArchivedAt = nil

	// En una implementación real:
	// - Decompress JSONL si estaba comprimido
	// - Make editable again
	// - Clear archived_at
	fmt.Printf("Job %s: Reabierto desde archivo\n", j.ID)
	return nil
}

// actionError ejecuta acciones al entrar en ERROR
func actionError(j *Job) error {
	if j.Error == nil {
		j.Error = &JobError{
			Code:       "UNKNOWN",
			Message:    "Error desconocido",
			Timestamp:  time.Now(),
			RetryCount: 0,
		}
	}

	// En una implementación real:
	// - Log error detallado
	// - Notificar usuario
	// - Auto-retry después de 30 seg (máx 3 veces)
	fmt.Printf("Job %s: Error - %s (intent %d)\n", j.ID, j.Error.Message, j.Error.RetryCount)
	return nil
}

// actionFailed ejecuta acciones cuando falla STARTING → ERROR
func actionFailed(j *Job) error {
	if j.Error == nil {
		j.Error = &JobError{
			Code:       "START_FAILED",
			Message:    "Falló al iniciar el proceso",
			Timestamp:  time.Now(),
			RetryCount: 0,
		}
	} else {
		j.Error.RetryCount = 0
	}

	// En una implementación real:
	// - Kill any lingering process
	// - Log error
	// - Save error_log file
	fmt.Printf("Job %s: Falló al iniciar\n", j.ID)
	return nil
}

// actionRetry ejecuta acciones al reintentar desde ERROR
func actionRetry(j *Job) error {
	if j.Error != nil {
		j.Error.RetryCount++
	}

	// En una implementación real:
	// - Clear error state
	// - Increment retry counter
	// - Set timeout para reintentar
	fmt.Printf("Job %s: Reintentando (intento %d)\n", j.ID, j.Error.RetryCount)
	return nil
}

// actionDelete ejecuta acciones al eliminar
func actionDelete(j *Job) error {
	// En una implementación real:
	// - Delete JSONL file
	// - Delete job record
	// - Remove from all indexes
	// - Free disk space
	// - Close PTY if open
	fmt.Printf("Job %s: Eliminado\n", j.ID)
	return nil
}

// ============================================================================
// TRANSITION METHOD - Aplicar transición atomáticamente
// ============================================================================

// Transition aplica una transición de estado validando y ejecutando acciones
func (s *JobService) Transition(jobID, event string) error {
	job, err := s.Get(jobID)
	if err != nil {
		return err
	}

	// Buscar transición válida
	for _, t := range TransitionTable {
		if t.From == job.State && t.Event == event {
			// Verificar guard
			if t.Guard != nil && !t.Guard(job) {
				return fmt.Errorf("transición bloqueada: %s -> %s (guard failed)", t.From, t.To)
			}

			// Ejecutar acción
			if err := t.Action(job); err != nil {
				return fmt.Errorf("error en acción de transición: %w", err)
			}

			// Cambiar estado
			job.State = t.To

			// Persistir
			return s.saveJob(job)
		}
	}

	return fmt.Errorf("transición inválida: %s en estado %s", event, job.State)
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// GetValidTransitions retorna todas las transiciones válidas desde un estado actual
func GetValidTransitions(currentState JobState) []string {
	var transitions []string
	for _, t := range TransitionTable {
		if t.From == currentState {
			transitions = append(transitions, t.Event)
		}
	}
	return transitions
}

// CanTransition verifica si una transición es válida sin ejecutarla
func CanTransition(currentState JobState, event string) bool {
	for _, t := range TransitionTable {
		if t.From == currentState && t.Event == event {
			return true
		}
	}
	return false
}

// GetNextState retorna el estado al que se transicionaría (sin ejecutar)
func GetNextState(currentState JobState, event string) (JobState, error) {
	for _, t := range TransitionTable {
		if t.From == currentState && t.Event == event {
			return t.To, nil
		}
	}
	return "", fmt.Errorf("no existe transición: %s -> %s", currentState, event)
}
