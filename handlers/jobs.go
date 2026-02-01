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

// ListJobs godoc
// @Summary      Listar jobs
// @Description  Retorna todos los jobs de un session-root, opcionalmente filtrados por estado
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true   "Path del session-root (URL encoded)"
// @Param        state     query     string  false  "Filtrar por estado (created, starting, active, paused, stopped, archived, error)"
// @Success      200       {object}  handlers.APIResponse{data=[]services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Failure      500       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs [get]
// @Security     BasicAuth
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	if rootPath == "" {
		WriteBadRequest(w, "root path requerido")
		return
	}

	// Obtener parámetro de filtro por estado
	state := r.URL.Query().Get("state")

	var jobs []*services.Job
	var err error

	if state != "" {
		// Filtrar por estado
		jobState := services.JobState(state)
		jobs, err = h.jobService.ListByState(rootPath, jobState)
	} else {
		// Listar todos
		jobs, err = h.jobService.List(rootPath)
	}

	if err != nil {
		WriteInternalError(w, err.Error())
		return
	}

	WriteSuccess(w, jobs)
}

// CreateJob godoc
// @Summary      Crear job
// @Description  Crea un nuevo job (trabajo de Claude o terminal)
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        request   body      object{name=string,description=string,work_dir=string,type=string,model=string,id=string}  true  "Configuración del job"
// @Success      201       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Failure      500       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs [post]
// @Security     BasicAuth
func (h *JobsHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	rootPath := URLParamDecoded(r, "rootPath")
	if rootPath == "" {
		WriteBadRequest(w, "root path requerido")
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
		ProjectPath: rootPath,
		RealPath:    rootPath, // En producción: decodificar
		Model:       reqBody.Model,
	}

	job, err := h.jobService.Create(cfg)
	if err != nil {
		WriteInternalError(w, fmt.Sprintf("Error creando job: %v", err))
		return
	}

	WriteCreated(w, job)
}

// GetJob godoc
// @Summary      Obtener job
// @Description  Retorna información detallada de un job
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Failure      404       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID} [get]
// @Security     BasicAuth
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

// DeleteJob godoc
// @Summary      Eliminar job
// @Description  Elimina un job permanentemente
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID} [delete]
// @Security     BasicAuth
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

// StartJob godoc
// @Summary      Iniciar job
// @Description  Inicia la ejecución de un job
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/start [post]
// @Security     BasicAuth
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

// PauseJob godoc
// @Summary      Pausar job
// @Description  Pausa la ejecución de un job activo
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/pause [post]
// @Security     BasicAuth
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

// ResumeJob godoc
// @Summary      Reanudar job
// @Description  Reanuda la ejecución de un job pausado
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/resume [post]
// @Security     BasicAuth
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

// StopJob godoc
// @Summary      Detener job
// @Description  Detiene la ejecución de un job activo o pausado
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/stop [post]
// @Security     BasicAuth
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

// ArchiveJob godoc
// @Summary      Archivar job
// @Description  Archiva un job detenido para ocultarlo de la lista principal
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/archive [post]
// @Security     BasicAuth
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

// RetryJob godoc
// @Summary      Reintentar job
// @Description  Reintenta la ejecución de un job en estado de error
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse{data=services.Job}
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/retry [post]
// @Security     BasicAuth
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

// DiscardJob godoc
// @Summary      Descartar job
// @Description  Descarta un job en estado de error (lo elimina)
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/discard [post]
// @Security     BasicAuth
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

// GetJobMessages godoc
// @Summary      Obtener mensajes de job
// @Description  Retorna los mensajes de conversación de un job
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Failure      404       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/messages [get]
// @Security     BasicAuth
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

// GetJobActions godoc
// @Summary      Obtener acciones de job
// @Description  Retorna las acciones/transiciones válidas para el estado actual del job
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string  true  "Path del session-root (URL encoded)"
// @Param        jobID     path      string  true  "ID del job"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Failure      404       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/{jobID}/actions [get]
// @Security     BasicAuth
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

// BatchDeleteJobs godoc
// @Summary      Eliminar jobs en lote
// @Description  Elimina múltiples jobs a la vez
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string            true  "Path del session-root (URL encoded)"
// @Param        request   body      object{ids=[]string}  true  "IDs de jobs a eliminar"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/batch/delete [post]
// @Security     BasicAuth
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

// BatchJobAction godoc
// @Summary      Ejecutar acción en lote
// @Description  Ejecuta una acción (start, pause, resume, stop, archive, delete) en múltiples jobs
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        rootPath  path      string                       true  "Path del session-root (URL encoded)"
// @Param        request   body      object{ids=[]string,action=string}  true  "IDs y acción a ejecutar"
// @Success      200       {object}  handlers.APIResponse
// @Failure      400       {object}  handlers.APIResponse
// @Router       /session-roots/{rootPath}/jobs/batch/action [post]
// @Security     BasicAuth
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
