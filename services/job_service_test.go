package services

import (
	"testing"
	"time"
)

// TestJobStateTransitions verifica que todas las transiciones de estado válidas funcionan
func TestJobStateTransitions(t *testing.T) {
	// Crear servicio mock
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	// Crear un job inicial
	cfg := JobConfig{
		Name:    "Test Job",
		WorkDir: "/tmp",
		Type:    "claude",
	}

	job, err := js.Create(cfg)
	if err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	tests := []struct {
		name        string
		currentState JobState
		event       string
		expectError bool
		expectState JobState
	}{
		// CREATED → STARTING
		{
			name:         "Start from CREATED",
			currentState: JobStateCreated,
			event:        "START",
			expectError:  false,
			expectState:  JobStateStarting,
		},
		// STARTING → ACTIVE
		{
			name:         "Ready from STARTING",
			currentState: JobStateStarting,
			event:        "READY",
			expectError:  true, // Guard: processRunning es false
			expectState:  JobStateStarting,
		},
		// STARTING → ERROR
		{
			name:         "Fail from STARTING",
			currentState: JobStateStarting,
			event:        "FAILED",
			expectError:  false,
			expectState:  JobStateError,
		},
		// CREATED → DELETE
		{
			name:         "Delete from CREATED",
			currentState: JobStateCreated,
			event:        "DELETE",
			expectError:  false,
			expectState:  JobStateDeleted,
		},
		// ERROR → RETRY
		{
			name:         "Retry from ERROR",
			currentState: JobStateError,
			event:        "RETRY",
			expectError:  false,
			expectState:  JobStateStarting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Resetear job a estado requerido
			job.State = tt.currentState
			js.saveJob(job)

			// Intentar transición
			err := js.Transition(job.ID, tt.event)

			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}

			// Verificar estado final
			updatedJob, _ := js.Get(job.ID)
			if !tt.expectError && updatedJob.State != tt.expectState {
				t.Errorf("Expected state %s, got %s", tt.expectState, updatedJob.State)
			}
		})
	}
}

// TestJobLifecycle verifica un flujo completo: CREATED → ACTIVE → PAUSED → ACTIVE → STOPPED → ARCHIVED
func TestJobLifecycle(t *testing.T) {
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	cfg := JobConfig{
		Name:    "Lifecycle Test",
		WorkDir: "/tmp",
		Type:    "claude",
	}

	job, _ := js.Create(cfg)
	if job.State != JobStateCreated {
		t.Errorf("Expected CREATED, got %s", job.State)
	}

	// CREATED → STARTING
	js.Start(job.ID)
	job, _ = js.Get(job.ID)
	if job.State != JobStateStarting {
		t.Errorf("Expected STARTING, got %s", job.State)
	}

	// Simular que el proceso está corriendo
	job.Cmd = &mockCommand{running: true}
	js.saveJob(job)

	// STARTING → ACTIVE
	js.Transition(job.ID, "READY")
	job, _ = js.Get(job.ID)
	if job.State != JobStateActive {
		t.Errorf("Expected ACTIVE, got %s", job.State)
	}

	// ACTIVE → PAUSED
	js.Pause(job.ID)
	job, _ = js.Get(job.ID)
	if job.State != JobStatePaused {
		t.Errorf("Expected PAUSED, got %s", job.State)
	}
	if job.PauseCount != 1 {
		t.Errorf("Expected pause_count=1, got %d", job.PauseCount)
	}

	// PAUSED → ACTIVE
	js.Resume(job.ID)
	job, _ = js.Get(job.ID)
	if job.State != JobStateActive {
		t.Errorf("Expected ACTIVE, got %s", job.State)
	}
	if job.ResumeCount != 1 {
		t.Errorf("Expected resume_count=1, got %d", job.ResumeCount)
	}

	// ACTIVE → STOPPED
	js.Stop(job.ID)
	job, _ = js.Get(job.ID)
	if job.State != JobStateStopped {
		t.Errorf("Expected STOPPED, got %s", job.State)
	}

	// STOPPED → ARCHIVED
	js.Archive(job.ID)
	job, _ = js.Get(job.ID)
	if job.State != JobStateArchived {
		t.Errorf("Expected ARCHIVED, got %s", job.State)
	}
	if !job.IsArchived {
		t.Errorf("Expected is_archived=true, got false")
	}
}

// TestJobResume verifica que se puede reanudar desde STOPPED preservando context
func TestJobResume(t *testing.T) {
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	cfg := JobConfig{
		Name:    "Resume Test",
		WorkDir: "/tmp",
		Type:    "claude",
	}

	job, _ := js.Create(cfg)
	originalID := job.ID

	// Mover a ACTIVE y luego STOPPED
	job.Cmd = &mockCommand{running: true}
	js.saveJob(job)
	js.Transition(job.ID, "START")
	js.Transition(job.ID, "READY")
	js.Stop(job.ID)

	job, _ = js.Get(job.ID)
	if job.State != JobStateStopped {
		t.Fatalf("Expected STOPPED, got %s", job.State)
	}

	// Reanudar
	js.Resume(job.ID)
	job, _ = js.Get(job.ID)

	// Verificar que el ID de sesión se preserva
	if job.SessionID != originalID {
		t.Errorf("Expected SessionID=%s, got %s", originalID, job.SessionID)
	}

	// Verificar que la transición es a STARTING (no CREATED)
	if job.State != JobStateStarting {
		t.Errorf("Expected STARTING, got %s", job.State)
	}

	// Verificar contador de reanudaciones
	if job.ResumeCount != 1 {
		t.Errorf("Expected resume_count=1, got %d", job.ResumeCount)
	}
}

// TestJobAutoArchive verifica que jobs antiguos se auto-archivan
func TestJobAutoArchive(t *testing.T) {
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	cfg := JobConfig{
		Name:    "AutoArchive Test",
		WorkDir: "/tmp",
		Type:    "claude",
	}

	job, _ := js.Create(cfg)

	// Simular que se detuvo hace más de 7 días
	stoppedTime := time.Now().Add(-8 * 24 * time.Hour)
	job.State = JobStateStopped
	job.StoppedAt = &stoppedTime
	js.saveJob(job)

	// Ejecutar auto-archive
	js.AutoArchiveOldJobs()

	// Verificar que está archivado
	job, _ = js.Get(job.ID)
	if job.State != JobStateArchived {
		t.Errorf("Expected ARCHIVED, got %s", job.State)
	}
	if !job.AutoArchived {
		t.Errorf("Expected auto_archived=true, got false")
	}
}

// TestInvalidTransitions verifica que transiciones inválidas retornan error
func TestInvalidTransitions(t *testing.T) {
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	cfg := JobConfig{
		Name:    "Invalid Transition Test",
		WorkDir: "/tmp",
		Type:    "claude",
	}

	job, _ := js.Create(cfg)

	invalidTests := []struct {
		name    string
		state   JobState
		event   string
		wantErr bool
	}{
		{"CREATED → ACTIVE (invalid)", JobStateCreated, "RESUME", true},
		{"CREATED → PAUSED (invalid)", JobStateCreated, "PAUSE", true},
		{"ARCHIVED → ACTIVE (invalid)", JobStateArchived, "START", true},
		{"DELETED → anything (invalid)", JobStateDeleted, "START", true},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			job.State = tt.state
			js.saveJob(job)

			err := js.Transition(job.ID, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestGetValidTransitions verifica que retorna acciones disponibles
func TestGetValidTransitions(t *testing.T) {
	tests := []struct {
		state    JobState
		expected []string
	}{
		{JobStateCreated, []string{"START", "DELETE"}},
		{JobStateActive, []string{"PAUSE", "STOP", "ERROR"}},
		{JobStatePaused, []string{"RESUME", "STOP", "ARCHIVE"}},
		{JobStateStopped, []string{"RESUME", "ARCHIVE", "DELETE"}},
		{JobStateArchived, []string{"REOPEN", "DELETE"}},
		{JobStateError, []string{"RETRY", "DISCARD"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			actions := GetValidTransitions(tt.state)

			if len(actions) != len(tt.expected) {
				t.Errorf("Expected %d actions, got %d", len(tt.expected), len(actions))
			}

			// Verificar que cada acción esperada existe
			for _, expected := range tt.expected {
				found := false
				for _, action := range actions {
					if action == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Missing action: %s", expected)
				}
			}
		})
	}
}

// TestJobListByState filtra correctamente por estado
func TestJobListByState(t *testing.T) {
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	// Crear varios jobs en diferentes estados
	states := []JobState{JobStateCreated, JobStateActive, JobStateStopped, JobStateArchived}
	for i, state := range states {
		cfg := JobConfig{
			Name:        "Test Job " + string(rune(i)),
			WorkDir:     "/tmp",
			Type:        "claude",
			ProjectPath: "test",
		}
		job, _ := js.Create(cfg)
		job.State = state
		job.ProjectPath = "test"
		js.saveJob(job)
	}

	// Probar filtrado
	for _, state := range states {
		filtered, _ := js.ListByState("test", state)
		for _, job := range filtered {
			if job.State != state {
				t.Errorf("Expected state %s, got %s", state, job.State)
			}
		}
	}
}

// TestValidateJobState verifica integridad de job
func TestValidateJobState(t *testing.T) {
	job := &Job{
		ID:          "",  // Invalid: empty ID
		WorkDir:     "/tmp",
		State:       JobStateActive,
		CreatedAt:   time.Now(),
		StartedAt:   nil, // Invalid: ACTIVE sin StartedAt
	}

	errors := ValidateJobState(job)
	if len(errors) < 2 {
		t.Errorf("Expected at least 2 validation errors, got %d", len(errors))
	}
}

// TestRepairJob intenta corregir jobs inconsistentes
func TestRepairJob(t *testing.T) {
	now := time.Now()
	job := &Job{
		ID:        "test-id",
		WorkDir:   "/tmp",
		State:     JobStateActive,
		CreatedAt: now,
		// Cmd es nil, lo cual es inválido para ACTIVE
	}

	repairs := RepairJob(job)
	if len(repairs) < 1 {
		t.Errorf("Expected repairs, got none")
	}

	if job.State != JobStateStopped {
		t.Errorf("Expected state changed to STOPPED, got %s", job.State)
	}
}

// Mock command para testing
type mockCommand struct {
	running bool
	Process *mockProcess
}

type mockProcess struct {
	Pid int
}

func (m *mockCommand) Run() error {
	return nil
}

func (m *mockCommand) Start() error {
	m.Process = &mockProcess{Pid: 12345}
	return nil
}

func (m *mockCommand) Wait() error {
	return nil
}

func (m *mockCommand) Kill() error {
	m.running = false
	return nil
}

// Benchmark para transiciones de estado
func BenchmarkJobTransition(b *testing.B) {
	js := &JobService{
		activeJobs:  make(map[string]*Job),
		savedJobs:   make(map[string]*SavedJob),
		claudeSvc:   nil,
		terminalSvc: nil,
		jobsDir:     "/tmp/jobs_test",
	}

	cfg := JobConfig{
		Name:    "Bench Job",
		WorkDir: "/tmp",
		Type:    "claude",
	}

	job, _ := js.Create(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		js.Start(job.ID)
		js.Pause(job.ID)
		js.Resume(job.ID)
		js.Stop(job.ID)

		job, _ = js.Get(job.ID)
		job.State = JobStateCreated
		js.saveJob(job)
	}
}
