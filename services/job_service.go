package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JobService gestiona los trabajos unificados
type JobService struct {
	activeJobs  map[string]*Job      // Jobs activos en memoria
	savedJobs   map[string]*SavedJob  // Jobs persistidos
	claudeSvc   *ClaudeService        // Para acceso a sessions
	terminalSvc *TerminalService      // Para acceso a terminales
	jobsDir     string                // Directorio para persistir jobs
	mu          sync.RWMutex
	savedMu     sync.RWMutex
}

// NewJobService crea una nueva instancia de JobService
func NewJobService() *JobService {
	return &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
	}
}

// SetJobsDir establece el directorio donde se persisten los jobs
func (s *JobService) SetJobsDir(jobsDir string) {
	s.jobsDir = jobsDir
	// Crear directorio si no existe
	os.MkdirAll(jobsDir, 0755)
}

// SetServices establece los servicios que depende JobService
func (s *JobService) SetServices(claudeSvc *ClaudeService, terminalSvc *TerminalService) {
	s.claudeSvc = claudeSvc
	s.terminalSvc = terminalSvc
}

// Create crea un nuevo job
func (s *JobService) Create(cfg JobConfig) (*Job, error) {
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("work_dir es requerido")
	}

	// Generar UUID si no existe
	if cfg.ID == "" {
		cfg.ID = generateUUID()
	}

	job := &Job{
		ID:          cfg.ID,
		SessionID:   cfg.ID,
		ProjectPath: cfg.ProjectPath,
		RealPath:    cfg.RealPath,
		Name:        cfg.Name,
		Description: cfg.Description,
		WorkDir:     cfg.WorkDir,
		Type:        cfg.Type,
		Model:       cfg.Model,
		State:       JobStateCreated,
		CreatedAt:   time.Now(),
	}

	// Guardar
	s.saveJob(job)

	return job, nil
}

// Get obtiene un job por ID
func (s *JobService) Get(id string) (*Job, error) {
	s.mu.RLock()
	activeJob, exists := s.activeJobs[id]
	s.mu.RUnlock()

	if exists {
		return activeJob, nil
	}

	// Buscar en saved jobs
	s.savedMu.RLock()
	savedJob, exists := s.savedJobs[id]
	s.savedMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job no encontrado: %s", id)
	}

	return FromSavedJob(savedJob), nil
}

// List lista todos los jobs de un proyecto
func (s *JobService) List(projectPath string) ([]*Job, error) {
	s.mu.RLock()
	activeJobs := make([]*Job, 0)
	for _, job := range s.activeJobs {
		if job.ProjectPath == projectPath {
			activeJobs = append(activeJobs, job)
		}
	}
	s.mu.RUnlock()

	s.savedMu.RLock()
	savedJobs := make([]*SavedJob, 0)
	for _, job := range s.savedJobs {
		if job.ProjectPath == projectPath {
			savedJobs = append(savedJobs, job)
		}
	}
	s.savedMu.RUnlock()

	// Combinar (active tiene prioridad sobre saved)
	jobs := make([]*Job, 0)
	jobMap := make(map[string]*Job)

	// Agregar active jobs
	for _, job := range activeJobs {
		jobMap[job.ID] = job
	}

	// Agregar saved jobs si no existen en active
	for _, savedJob := range savedJobs {
		if _, exists := jobMap[savedJob.ID]; !exists {
			jobMap[savedJob.ID] = FromSavedJob(savedJob)
		}
	}

	// Convertir map a slice
	for _, job := range jobMap {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// ListByState lista jobs filtrados por estado
func (s *JobService) ListByState(projectPath string, state JobState) ([]*Job, error) {
	allJobs, err := s.List(projectPath)
	if err != nil {
		return nil, err
	}

	filtered := make([]*Job, 0)
	for _, job := range allJobs {
		if job.State == state {
			filtered = append(filtered, job)
		}
	}

	return filtered, nil
}

// Start inicia un job
func (s *JobService) Start(id string) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}

	if job.State != JobStateCreated {
		return fmt.Errorf("solo se pueden iniciar jobs en estado CREATED, actual: %s", job.State)
	}

	// Transicionar a STARTING
	return s.Transition(id, "START")
}

// Pause pausa un job activo
func (s *JobService) Pause(id string) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}

	if job.State != JobStateActive {
		return fmt.Errorf("solo se pueden pausar jobs ACTIVE, actual: %s", job.State)
	}

	return s.Transition(id, "PAUSE")
}

// Resume reanuda un job pausado o detenido
func (s *JobService) Resume(id string) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}

	switch job.State {
	case JobStatePaused:
		return s.Transition(id, "RESUME")
	case JobStateStopped:
		return s.Transition(id, "RESUME")
	default:
		return fmt.Errorf("no se puede reanudar job en estado %s", job.State)
	}
}

// Stop detiene un job activo o pausado
func (s *JobService) Stop(id string) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}

	switch job.State {
	case JobStateActive:
		return s.Transition(id, "STOP")
	case JobStatePaused:
		return s.Transition(id, "STOP")
	default:
		return fmt.Errorf("no se puede detener job en estado %s", job.State)
	}
}

// Archive archiva un job detenido
func (s *JobService) Archive(id string) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}

	if job.State != JobStateStopped {
		return fmt.Errorf("solo se pueden archivar jobs STOPPED, actual: %s", job.State)
	}

	return s.Transition(id, "ARCHIVE")
}

// Delete elimina un job
func (s *JobService) Delete(id string) error {
	job, err := s.Get(id)
	if err != nil {
		return err
	}

	// Solo se pueden eliminar jobs en ciertos estados
	switch job.State {
	case JobStateCreated, JobStateStopped, JobStateArchived, JobStateError:
		return s.Transition(id, "DELETE")
	default:
		return fmt.Errorf("no se puede eliminar job en estado %s", job.State)
	}
}

// saveJob persiste un job
func (s *JobService) saveJob(job *Job) error {
	s.mu.Lock()
	s.activeJobs[job.ID] = job
	s.mu.Unlock()

	s.savedMu.Lock()
	s.savedJobs[job.ID] = job.ToSavedJob()
	s.savedMu.Unlock()

	// Persistir a disco
	return s.persistJob(job)
}

// persistJob guarda un job a disco
func (s *JobService) persistJob(job *Job) error {
	filePath := filepath.Join(s.jobsDir, job.ID+".json")

	data, err := json.MarshalIndent(job.ToSavedJob(), "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling job: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("error escribiendo job a disco: %w", err)
	}

	return nil
}

// LoadJobsFromDisk carga todos los jobs persistidos desde disco
func (s *JobService) LoadJobsFromDisk() error {
	entries, err := os.ReadDir(s.jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directorio no existe, es ok
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.jobsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error leyendo job %s: %v\n", entry.Name(), err)
			continue
		}

		var savedJob SavedJob
		if err := json.Unmarshal(data, &savedJob); err != nil {
			fmt.Printf("Error unmarshaling job %s: %v\n", entry.Name(), err)
			continue
		}

		s.savedMu.Lock()
		s.savedJobs[savedJob.ID] = &savedJob
		s.savedMu.Unlock()
	}

	return nil
}