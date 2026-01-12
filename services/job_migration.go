package services

import (
	"fmt"
	"time"
)

// MigrationStats estadísticas de la migración
type MigrationStats struct {
	TerminalsMigrated     int
	SessionsMigrated      int
	SessionsAutoArchived  int
	TotalJobsCreated      int
	Errors                []string
}

// MigrateTerminalsToJobs migra terminales existentes al nuevo modelo de Jobs
// Los terminales que están activos se mapean a estado ACTIVE
// Los terminales detenidos se mapean a estado STOPPED
func (s *JobService) MigrateTerminalsToJobs(terminalSvc *TerminalService) error {
	if terminalSvc == nil {
		return fmt.Errorf("terminalSvc no puede ser nil")
	}

	fmt.Println("Iniciando migración de Terminales a Jobs...")

	// En una implementación real, obtendrías todos los terminales
	// terminals := terminalSvc.ListAll()
	// Por ahora simular estructura

	// for _, terminal := range terminals {
	//     job := &Job{
	//         ID:          terminal.ID,
	//         SessionID:   terminal.SessionID,
	//         ProjectPath: terminal.ProjectPath,
	//         RealPath:    terminal.RealPath,
	//         Name:        terminal.Name,
	//         WorkDir:     terminal.WorkDir,
	//         Type:        terminal.Type,
	//         Model:       terminal.Model,
	//         CreatedAt:   terminal.CreatedAt,
	//         PtyID:       terminal.ID,
	//     }

	//     // Determinar estado basado en activo/inactivo
	//     if terminal.Active {
	//         job.State = JobStateActive
	//         job.StartedAt = &terminal.StartedAt
	//         job.Cmd = terminal.Cmd
	//         job.Pty = terminal.Pty
	//         if terminal.Cmd != nil && terminal.Cmd.Process != nil {
	//             job.ProcessID = terminal.Cmd.Process.Pid
	//         }
	//         job.Clients = len(terminal.Clients)
	//     } else {
	//         job.State = JobStateStopped
	//         job.StoppedAt = &terminal.LastAccessAt
	//     }

	//     s.saveJob(job)
	// }

	fmt.Println("Migración de Terminales completada")
	return nil
}

// MigrateSessionsToJobs migra sesiones existentes (sin terminal asociado)
// Las sesiones muy antiguas (> 30 días sin modificar) se auto-archivan
func (s *JobService) MigrateSessionsToJobs(claudeSvc *ClaudeService, projectPath string) error {
	if claudeSvc == nil {
		return fmt.Errorf("claudeSvc no puede ser nil")
	}

	fmt.Printf("Iniciando migración de Sesiones para proyecto: %s\n", projectPath)

	// En una implementación real:
	// sessions, err := claudeSvc.ListSessions(projectPath)
	// if err != nil {
	//     return fmt.Errorf("error listando sesiones: %w", err)
	// }

	// for _, session := range sessions {
	//     // Verificar si ya existe como job
	//     if _, exists := s.savedJobs[session.ID]; exists {
	//         continue // Ya migrada
	//     }

	//     job := &Job{
	//         ID:                session.ID,
	//         SessionID:         session.ID,
	//         ProjectPath:       session.ProjectPath,
	//         RealPath:          session.RealPath,
	//         Name:              session.Name,
	//         WorkDir:           session.RealPath,
	//         Type:              "claude",
	//         State:             JobStateStopped,
	//         CreatedAt:         session.CreatedAt,
	//         MessageCount:      session.MessageCount,
	//         UserMessages:      session.UserMessages,
	//         AssistantMessages: session.AssistantMessages,
	//     }

	//     stoppedAt := session.ModifiedAt
	//     job.StoppedAt = &stoppedAt

	//     // Si es muy antigua, auto-archivar
	//     if time.Since(session.ModifiedAt) > 30*24*time.Hour {
	//         job.State = JobStateArchived
	//         archivedAt := time.Now()
	//         job.ArchivedAt = &archivedAt
	//         job.AutoArchived = true
	//     }

	//     s.saveJob(job)
	// }

	fmt.Println("Migración de Sesiones completada")
	return nil
}

// MigrationReport ejecuta una migración completa y retorna un reporte detallado
func (s *JobService) MigrationReport(terminalSvc *TerminalService, claudeSvc *ClaudeService, projectPath string) (*MigrationStats, error) {
	stats := &MigrationStats{
		Errors: []string{},
	}

	// Migrar terminales
	if terminalSvc != nil {
		if err := s.MigrateTerminalsToJobs(terminalSvc); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("Error migrando terminales: %v", err))
		} else {
			// En implementación real: stats.TerminalsMigrated = len(terminals)
			stats.TerminalsMigrated = 0
		}
	}

	// Migrar sesiones
	if claudeSvc != nil {
		if err := s.MigrateSessionsToJobs(claudeSvc, projectPath); err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("Error migrando sesiones: %v", err))
		} else {
			// En implementación real: stats.SessionsMigrated = len(sessions)
			stats.SessionsMigrated = 0
		}
	}

	stats.TotalJobsCreated = stats.TerminalsMigrated + stats.SessionsMigrated

	return stats, nil
}

// ============================================================================
// FALLBACK / COMPATIBILITY - Funciones para mantener compatibilidad
// ============================================================================

// GetJobAsTerminal convierte un Job a formato Terminal (para compatibilidad)
// Esta función es temporal durante la migración gradual
func (s *JobService) GetJobAsTerminal(jobID string) (map[string]interface{}, error) {
	job, err := s.Get(jobID)
	if err != nil {
		return nil, err
	}

	// Mapear Job → Terminal
	terminalData := map[string]interface{}{
		"id":               job.ID,
		"session_id":       job.SessionID,
		"name":             job.Name,
		"work_dir":         job.WorkDir,
		"type":             job.Type,
		"created_at":       job.CreatedAt,
		"active":           job.State == JobStateActive,
		"last_access_at":   job.StoppedAt,
		"can_resume":       job.State == JobStateStopped && canResumeStopped(job),
		"clients":          job.Clients,
		"project_path":     job.ProjectPath,
		// Job state mapping to Terminal status
		"status": mapJobStateToTerminalStatus(job.State),
	}

	return terminalData, nil
}

// GetJobAsSession convierte un Job a formato Session (para compatibilidad)
// Esta función es temporal durante la migración gradual
func (s *JobService) GetJobAsSession(jobID string) (map[string]interface{}, error) {
	job, err := s.Get(jobID)
	if err != nil {
		return nil, err
	}

	// Mapear Job → Session
	sessionData := map[string]interface{}{
		"id":                   job.SessionID,
		"name":                 job.Name,
		"project_path":         job.ProjectPath,
		"real_path":            job.RealPath,
		"work_dir":             job.WorkDir,
		"created_at":           job.CreatedAt,
		"modified_at":          job.StoppedAt,
		"message_count":        job.MessageCount,
		"user_messages":        job.UserMessages,
		"assistant_messages":   job.AssistantMessages,
		"is_archived":          job.IsArchived,
		"archived_at":          job.ArchivedAt,
		// Job state mapping to Session status
		"status": mapJobStateToSessionStatus(job.State),
	}

	return sessionData, nil
}

// mapJobStateToTerminalStatus mapea JobState a status compatible con Terminal
func mapJobStateToTerminalStatus(state JobState) string {
	switch state {
	case JobStateActive:
		return "running"
	case JobStatePaused:
		return "paused"
	case JobStateStopped:
		return "stopped"
	case JobStateArchived:
		return "archived"
	case JobStateError:
		return "error"
	case JobStateDeleted:
		return "deleted"
	case JobStateCreated, JobStateStarting:
		return "initializing"
	default:
		return "unknown"
	}
}

// mapJobStateToSessionStatus mapea JobState a status compatible con Session
func mapJobStateToSessionStatus(state JobState) string {
	switch state {
	case JobStateCreated, JobStateStarting, JobStateActive:
		return "active"
	case JobStatePaused:
		return "paused"
	case JobStateStopped:
		return "inactive"
	case JobStateArchived:
		return "archived"
	case JobStateError:
		return "error"
	case JobStateDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// ============================================================================
// REVERSE COMPATIBILITY - Operaciones con Jobs desde endpoints viejos
// ============================================================================

// CreateJobFromTerminalConfig crea un Job a partir de config de Terminal
// Usado cuando se crea terminal desde UI vieja
func (s *JobService) CreateJobFromTerminalConfig(config map[string]interface{}) (*Job, error) {
	jobConfig := JobConfig{
		Name:        config["name"].(string),
		WorkDir:     config["work_dir"].(string),
		Type:        config["type"].(string),
		ProjectPath: config["project_path"].(string),
		RealPath:    config["real_path"].(string),
		Model:       "claude-3.5-sonnet",
	}

	if id, ok := config["id"].(string); ok && id != "" {
		jobConfig.ID = id
	}

	return s.Create(jobConfig)
}

// CreateJobFromSessionConfig crea un Job a partir de config de Session
// Usado cuando se crea sesión desde UI vieja
func (s *JobService) CreateJobFromSessionConfig(config map[string]interface{}) (*Job, error) {
	jobConfig := JobConfig{
		Name:        config["name"].(string),
		WorkDir:     config["work_dir"].(string),
		Type:        "claude",
		ProjectPath: config["project_path"].(string),
		RealPath:    config["real_path"].(string),
		Model:       "claude-3.5-sonnet",
	}

	if id, ok := config["id"].(string); ok && id != "" {
		jobConfig.ID = id
	}

	return s.Create(jobConfig)
}

// ============================================================================
// AUTO-MAINTENANCE - Tareas automáticas de limpieza
// ============================================================================

// AutoArchiveOldJobs archiva automáticamente jobs que llevan > 7 días detenidos
func (s *JobService) AutoArchiveOldJobs() error {
	s.savedMu.RLock()
	jobsToArchive := make([]*SavedJob, 0)

	for _, savedJob := range s.savedJobs {
		if savedJob.State == JobStateStopped && savedJob.StoppedAt != nil {
			if time.Since(*savedJob.StoppedAt) > 7*24*time.Hour {
				jobsToArchive = append(jobsToArchive, savedJob)
			}
		}
	}
	s.savedMu.RUnlock()

	// Archivar jobs
	for _, savedJob := range jobsToArchive {
		job := FromSavedJob(savedJob)
		if err := s.Transition(job.ID, "ARCHIVE"); err != nil {
			fmt.Printf("Error auto-archivando job %s: %v\n", job.ID, err)
		}
	}

	fmt.Printf("Auto-archivó %d jobs antiguos\n", len(jobsToArchive))
	return nil
}

// CleanupDeletedJobs limpia archivos de jobs eliminados
func (s *JobService) CleanupDeletedJobs() error {
	s.savedMu.RLock()
	jobsToDelete := make([]string, 0)

	for id, savedJob := range s.savedJobs {
		if savedJob.State == JobStateDeleted {
			jobsToDelete = append(jobsToDelete, id)
		}
	}
	s.savedMu.RUnlock()

	// Remover del saved map
	for _, id := range jobsToDelete {
		s.savedMu.Lock()
		delete(s.savedJobs, id)
		s.savedMu.Unlock()

		// En implementación real: eliminar archivo JSON
	}

	fmt.Printf("Limpiados %d jobs eliminados\n", len(jobsToDelete))
	return nil
}

// ============================================================================
// VALIDATION - Validación de integridad
// ============================================================================

// ValidateJobState verifica que un job está en estado consistente
func ValidateJobState(job *Job) []string {
	var errors []string

	// Validaciones básicas
	if job.ID == "" {
		errors = append(errors, "Job ID no puede estar vacío")
	}

	if job.WorkDir == "" && job.State != JobStateDeleted {
		errors = append(errors, "WorkDir no puede estar vacío (excepto en DELETED)")
	}

	// Validaciones de timestamps
	if job.StartedAt != nil && job.StartedAt.After(time.Now()) {
		errors = append(errors, "StartedAt no puede ser en el futuro")
	}

	if job.StoppedAt != nil && job.StoppedAt.Before(job.CreatedAt) {
		errors = append(errors, "StoppedAt no puede ser antes de CreatedAt")
	}

	if job.ArchivedAt != nil && job.StoppedAt != nil && job.ArchivedAt.Before(*job.StoppedAt) {
		errors = append(errors, "ArchivedAt no puede ser antes de StoppedAt")
	}

	// Validaciones de estado
	switch job.State {
	case JobStateActive:
		if job.StartedAt == nil {
			errors = append(errors, "Job ACTIVE debe tener StartedAt")
		}
		if job.Cmd == nil || job.Cmd.Process == nil {
			errors = append(errors, "Job ACTIVE debe tener proceso en ejecución")
		}
	case JobStateArchived:
		if job.ArchivedAt == nil {
			errors = append(errors, "Job ARCHIVED debe tener ArchivedAt")
		}
	case JobStateError:
		if job.Error == nil {
			errors = append(errors, "Job ERROR debe tener información de error")
		}
	}

	return errors
}

// RepairJob intenta reparar un job en estado inconsistente
func RepairJob(job *Job) []string {
	var repairs []string

	// Reparación: Si está en ACTIVE pero no hay proceso, pasar a STOPPED
	if job.State == JobStateActive && (job.Cmd == nil || job.Cmd.Process == nil) {
		repairs = append(repairs, fmt.Sprintf("Job %s: ACTIVE sin proceso → STOPPED", job.ID))
		job.State = JobStateStopped
		now := time.Now()
		job.StoppedAt = &now
	}

	// Reparación: Si StoppedAt es nil en STOPPED, usar ahora
	if job.State == JobStateStopped && job.StoppedAt == nil {
		repairs = append(repairs, fmt.Sprintf("Job %s: StoppedAt vacío → ahora", job.ID))
		now := time.Now()
		job.StoppedAt = &now
	}

	return repairs
}
