package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"claude-monitor/services"
)

// JobsHandler maneja todos los endpoints relacionados con trabajos
type JobsHandler struct {
	jobService *services.JobService
}

// NewJobsHandler crea un nuevo handler de trabajos
func NewJobsHandler(jobService *services.JobService) *JobsHandler {
	return &JobsHandler{
		jobService: jobService,
	}
}

// ============================================================================
// GET /api/projects/{projectPath}/jobs - Lista todos los jobs
// ============================================================================
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	projectPath := URLParamDecoded(r, "projectPath")
	if projectPath == "" {
		WriteBadRequest(w, "project path requerido")
		return
	}

	// Obtener parámetro de filtro por estado
	state := r.URL.Query().Get("state")

	var jobs []*services.Job
	var err error

	if state != "" {
		// Filtrar por estado
		jobState := services.JobState(state)
		jobs, err = h.jobService.ListByState(projectPath, jobState)
	} else {
		// Listar todos
		jobs, err = h.jobService.List(projectPath)
	}

	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, jobs)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs - Crear nuevo job
// ============================================================================
func (h *JobsHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	projectPath := URLParamDecoded(r, "projectPath")
	if projectPath == "" {
		WriteBadRequest(w, "project path requerido")
		return
	}

	var reqBody struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		WorkDir     string `json:"work_dir"`
		Type        string `json:"type"` // "claude" o "terminal"
		Model       string `json:"model"`
		ID          string `json:"id"` // Opcional
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		WriteBadRequest(w, "JSON inválido")
		return
	}

	if reqBody.WorkDir == "" {
		WriteBadRequest(w, "work_dir requerido")
		return
	}

	if reqBody.Type != "claude" && reqBody.Type != "terminal" {
		WriteBadRequest(w, "type debe ser 'claude' o 'terminal'")
		return
	}

	cfg := services.JobConfig{
		ID:          reqBody.ID,
		Name:        reqBody.Name,
		Description: reqBody.Description,
		WorkDir:     reqBody.WorkDir,
		Type:        reqBody.Type,
		ProjectPath: projectPath,
		RealPath:    projectPath, // En producción: decodificar
		Model:       reqBody.Model,
	}

	job, err := h.jobService.Create(cfg)
	if err != nil {
		WriteInternalError(w, fmt.Sprintf("Error creando job: %v", err))
		return
	}

	WriteCreated(w, job)
}

// ============================================================================
// GET /api/projects/{projectPath}/jobs/{jobID} - Obtener info de un job
// ============================================================================
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	job, err := h.jobService.Get(jobID)
	if err != nil {
		WriteNotFound(w, "job")
		return
	}

	WriteSuccess(w, job)
}

// ============================================================================
// DELETE /api/projects/{projectPath}/jobs/{jobID} - Eliminar un job
// ============================================================================
func (h *JobsHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Delete(jobID); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error eliminando job: %v", err))
		return
	}

	WriteSuccess(w, map[string]string{"id": jobID, "state": "deleted"})
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/start - Iniciar un job
// ============================================================================
func (h *JobsHandler) StartJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Transition(jobID, "START"); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error iniciando job: %v", err))
		return
	}

	job, _ := h.jobService.Get(jobID)
	WriteSuccess(w, job)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/pause - Pausar un job
// ============================================================================
func (h *JobsHandler) PauseJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Pause(jobID); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error pausando job: %v", err))
		return
	}

	job, _ := h.jobService.Get(jobID)
	WriteSuccess(w, job)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/resume - Reanudar un job
// ============================================================================
func (h *JobsHandler) ResumeJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Resume(jobID); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error reanudando job: %v", err))
		return
	}

	job, _ := h.jobService.Get(jobID)
	WriteSuccess(w, job)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/stop - Detener un job
// ============================================================================
func (h *JobsHandler) StopJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Stop(jobID); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error deteniendo job: %v", err))
		return
	}

	job, _ := h.jobService.Get(jobID)
	WriteSuccess(w, job)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/archive - Archivar un job
// ============================================================================
func (h *JobsHandler) ArchiveJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Archive(jobID); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error archivando job: %v", err))
		return
	}

	job, _ := h.jobService.Get(jobID)
	WriteSuccess(w, job)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/retry - Reintentar un job en ERROR
// ============================================================================
func (h *JobsHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Transition(jobID, "RETRY"); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error reintentando job: %v", err))
		return
	}

	job, _ := h.jobService.Get(jobID)
	WriteSuccess(w, job)
}

// ============================================================================
// POST /api/projects/{projectPath}/jobs/{jobID}/discard - Descartar un job en ERROR
// ============================================================================
func (h *JobsHandler) DiscardJob(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	if err := h.jobService.Transition(jobID, "DISCARD"); err != nil {
		WriteBadRequest(w, fmt.Sprintf("Error descartando job: %v", err))
		return
	}

	WriteSuccess(w, map[string]string{"id": jobID, "state": "deleted"})
}

// ============================================================================
// GET /api/projects/{projectPath}/jobs/{jobID}/messages - Obtener mensajes
// ============================================================================
func (h *JobsHandler) GetJobMessages(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	job, err := h.jobService.Get(jobID)
	if err != nil {
		WriteNotFound(w, "job")
		return
	}

	// En implementación real: leer mensajes del archivo JSONL
	messages := map[string]interface{}{
		"id":                 job.ID,
		"message_count":      job.MessageCount,
		"user_messages":      job.UserMessages,
		"assistant_messages": job.AssistantMessages,
		"messages":           []interface{}{}, // Placeholder
	}

	WriteSuccess(w, messages)
}

// ============================================================================
// GET /api/projects/{projectPath}/jobs/{jobID}/actions - Obtener acciones disponibles
// ============================================================================
func (h *JobsHandler) GetJobActions(w http.ResponseWriter, r *http.Request) {
	jobID := URLParam(r, "jobID")
	if jobID == "" {
		WriteBadRequest(w, "job id requerido")
		return
	}

	job, err := h.jobService.Get(jobID)
	if err != nil {
		WriteNotFound(w, "job")
		return
	}

	actions := services.GetValidTransitions(job.State)

	WriteSuccess(w, map[string]interface{}{
		"id":      job.ID,
		"state":   job.State,
		"actions": actions,
	})
}

// ============================================================================
// BATCH OPERATIONS - Operaciones en lote
// ============================================================================

// BatchDeleteJobs POST /api/projects/{projectPath}/jobs/batch/delete
func (h *JobsHandler) BatchDeleteJobs(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		IDs []string `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		WriteBadRequest(w, "JSON inválido")
		return
	}

	if len(reqBody.IDs) == 0 {
		WriteBadRequest(w, "ids no puede estar vacío")
		return
	}

	deleted := 0
	errors := []string{}

	for _, id := range reqBody.IDs {
		if err := h.jobService.Delete(id); err != nil {
			errors = append(errors, fmt.Sprintf("Error deletando %s: %v", id, err))
		} else {
			deleted++
		}
	}

	WriteSuccess(w, map[string]interface{}{
		"deleted": deleted,
		"total":   len(reqBody.IDs),
		"errors":  errors,
	})
}

// BatchJobAction POST /api/projects/{projectPath}/jobs/batch/action
func (h *JobsHandler) BatchJobAction(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"` // start, pause, stop, etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		WriteBadRequest(w, "JSON inválido")
		return
	}

	if len(reqBody.IDs) == 0 {
		WriteBadRequest(w, "ids no puede estar vacío")
		return
	}

	if reqBody.Action == "" {
		WriteBadRequest(w, "action requerido")
		return
	}

	// Mapear action a método de servicio
	succeeded := 0
	errors := []string{}

	for _, id := range reqBody.IDs {
		var err error
		switch strings.ToLower(reqBody.Action) {
		case "start":
			err = h.jobService.Start(id)
		case "pause":
			err = h.jobService.Pause(id)
		case "resume":
			err = h.jobService.Resume(id)
		case "stop":
			err = h.jobService.Stop(id)
		case "archive":
			err = h.jobService.Archive(id)
		case "delete":
			err = h.jobService.Delete(id)
		default:
			err = fmt.Errorf("action no reconocida: %s", reqBody.Action)
		}

		if err != nil {
			errors = append(errors, fmt.Sprintf("Error en %s: %v", id, err))
		} else {
			succeeded++
		}
	}

	WriteSuccess(w, map[string]interface{}{
		"action":    reqBody.Action,
		"succeeded": succeeded,
		"total":     len(reqBody.IDs),
		"errors":    errors,
	})
}
