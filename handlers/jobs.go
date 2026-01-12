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

// Response estructura genérica para respuestas API
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ============================================================================
// GET /api/projects/{path}/jobs - Lista todos los jobs
// ============================================================================
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	projectPath := r.PathValue("path")
	if projectPath == "" {
		http.Error(w, "project path requerido", http.StatusBadRequest)
		return
	}

	// Obtener parámetro de filtro por estado
	state := r.URL.Query().Get("state")

	w.Header().Set("Content-Type", "application/json")

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
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    jobs,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs - Crear nuevo job
// ============================================================================
func (h *JobsHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	projectPath := r.PathValue("path")
	if projectPath == "" {
		http.Error(w, "project path requerido", http.StatusBadRequest)
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
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if reqBody.WorkDir == "" {
		http.Error(w, "work_dir requerido", http.StatusBadRequest)
		return
	}

	if reqBody.Type != "claude" && reqBody.Type != "terminal" {
		http.Error(w, "type debe ser 'claude' o 'terminal'", http.StatusBadRequest)
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
		http.Error(w, fmt.Sprintf("Error creando job: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// GET /api/projects/{path}/jobs/{id} - Obtener info de un job
// ============================================================================
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.Get(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job no encontrado: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// DELETE /api/projects/{path}/jobs/{id} - Eliminar un job
// ============================================================================
func (h *JobsHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Delete(jobID); err != nil {
		http.Error(w, fmt.Sprintf("Error eliminando job: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    map[string]string{"id": jobID, "state": "deleted"},
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/start - Iniciar un job
// ============================================================================
func (h *JobsHandler) StartJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	// Usar transición de estado
	if err := h.jobService.Transition(jobID, "START"); err != nil {
		http.Error(w, fmt.Sprintf("Error iniciando job: %v", err), http.StatusBadRequest)
		return
	}

	job, _ := h.jobService.Get(jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/pause - Pausar un job
// ============================================================================
func (h *JobsHandler) PauseJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Pause(jobID); err != nil {
		http.Error(w, fmt.Sprintf("Error pausando job: %v", err), http.StatusBadRequest)
		return
	}

	job, _ := h.jobService.Get(jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/resume - Reanudar un job
// ============================================================================
func (h *JobsHandler) ResumeJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Resume(jobID); err != nil {
		http.Error(w, fmt.Sprintf("Error reanudando job: %v", err), http.StatusBadRequest)
		return
	}

	job, _ := h.jobService.Get(jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/stop - Detener un job
// ============================================================================
func (h *JobsHandler) StopJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Stop(jobID); err != nil {
		http.Error(w, fmt.Sprintf("Error deteniendo job: %v", err), http.StatusBadRequest)
		return
	}

	job, _ := h.jobService.Get(jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/archive - Archivar un job
// ============================================================================
func (h *JobsHandler) ArchiveJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Archive(jobID); err != nil {
		http.Error(w, fmt.Sprintf("Error archivando job: %v", err), http.StatusBadRequest)
		return
	}

	job, _ := h.jobService.Get(jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/retry - Reintentar un job en ERROR
// ============================================================================
func (h *JobsHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Transition(jobID, "RETRY"); err != nil {
		http.Error(w, fmt.Sprintf("Error reintentando job: %v", err), http.StatusBadRequest)
		return
	}

	job, _ := h.jobService.Get(jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    job,
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/{id}/discard - Descartar un job en ERROR
// ============================================================================
func (h *JobsHandler) DiscardJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	if err := h.jobService.Transition(jobID, "DISCARD"); err != nil {
		http.Error(w, fmt.Sprintf("Error descartando job: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    map[string]string{"id": jobID, "state": "deleted"},
	})
}

// ============================================================================
// GET /api/projects/{path}/jobs/{id}/messages - Obtener mensajes de conversación
// ============================================================================
func (h *JobsHandler) GetJobMessages(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.Get(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job no encontrado: %v", err), http.StatusNotFound)
		return
	}

	// En implementación real: leer mensajes del archivo JSONL
	messages := map[string]interface{}{
		"id":                   job.ID,
		"message_count":        job.MessageCount,
		"user_messages":        job.UserMessages,
		"assistant_messages":   job.AssistantMessages,
		"messages":             []interface{}{}, // Placeholder
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    messages,
	})
}

// ============================================================================
// GET /api/projects/{path}/jobs/{id}/actions - Obtener acciones disponibles
// ============================================================================
func (h *JobsHandler) GetJobActions(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id requerido", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.Get(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job no encontrado: %v", err), http.StatusNotFound)
		return
	}

	actions := services.GetValidTransitions(job.State)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"id":       job.ID,
			"state":    job.State,
			"actions":  actions,
		},
	})
}

// ============================================================================
// ROUTING - Registrar todas las rutas
// ============================================================================

// RegisterJobsRoutes registra todas las rutas de jobs en un mux
func RegisterJobsRoutes(mux *http.ServeMux, handler *JobsHandler) {
	// Listar jobs
	mux.HandleFunc("GET /api/projects/{path}/jobs", handler.ListJobs)

	// Crear job
	mux.HandleFunc("POST /api/projects/{path}/jobs", handler.CreateJob)

	// CRUD individual
	mux.HandleFunc("GET /api/projects/{path}/jobs/{id}", handler.GetJob)
	mux.HandleFunc("DELETE /api/projects/{path}/jobs/{id}", handler.DeleteJob)

	// Transiciones de estado
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/start", handler.StartJob)
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/pause", handler.PauseJob)
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/resume", handler.ResumeJob)
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/stop", handler.StopJob)
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/archive", handler.ArchiveJob)

	// Error handling
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/retry", handler.RetryJob)
	mux.HandleFunc("POST /api/projects/{path}/jobs/{id}/discard", handler.DiscardJob)

	// Información
	mux.HandleFunc("GET /api/projects/{path}/jobs/{id}/messages", handler.GetJobMessages)
	mux.HandleFunc("GET /api/projects/{path}/jobs/{id}/actions", handler.GetJobActions)
}

// ============================================================================
// BATCH OPERATIONS - Operaciones en lote
// ============================================================================

// ============================================================================
// POST /api/projects/{path}/jobs/batch/delete - Eliminar múltiples jobs
// ============================================================================
func (h *JobsHandler) BatchDeleteJobs(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		IDs []string `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if len(reqBody.IDs) == 0 {
		http.Error(w, "ids no puede estar vacío", http.StatusBadRequest)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": len(errors) == 0,
		"data": map[string]interface{}{
			"deleted": deleted,
			"total":   len(reqBody.IDs),
			"errors":  errors,
		},
	})
}

// ============================================================================
// POST /api/projects/{path}/jobs/batch/action - Ejecutar acción en lote
// ============================================================================
func (h *JobsHandler) BatchJobAction(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"` // start, pause, stop, etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if len(reqBody.IDs) == 0 {
		http.Error(w, "ids no puede estar vacío", http.StatusBadRequest)
		return
	}

	if reqBody.Action == "" {
		http.Error(w, "action requerido", http.StatusBadRequest)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": len(errors) == 0,
		"data": map[string]interface{}{
			"action":    reqBody.Action,
			"succeeded": succeeded,
			"total":     len(reqBody.IDs),
			"errors":    errors,
		},
	})
}
