package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

	"claude-monitor/pkg/metrics"
	"claude-monitor/services"
)

// HostHandler maneja endpoints del host
type HostHandler struct {
	startedAt time.Time
	version   string
	hostName  string
	claudeDir string
	terminals *services.TerminalService
	claude    *services.ClaudeService
}

// HostInfo información del host
type HostInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Platform  string    `json:"platform"`
	Arch      string    `json:"arch"`
	ClaudeDir string    `json:"claude_dir"`
	StartedAt time.Time `json:"started_at"`
	Uptime    string    `json:"uptime"`
	Stats     HostStats `json:"stats"`
}

// HostStats estadísticas del host
type HostStats struct {
	ActiveTerminals int `json:"active_terminals"`
	TotalProjects   int `json:"total_projects"`
	TotalSessions   int `json:"total_sessions"`
}

// HealthStatus estado de salud
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusDegraded HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck resultado de un check de salud
type HealthCheck struct {
	Status   HealthStatus `json:"status"`
	Message  string       `json:"message,omitempty"`
	Duration string       `json:"duration,omitempty"`
}

// HealthResponse respuesta del endpoint de salud
type HealthResponse struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Checks    map[string]HealthCheck `json:"checks"`
	Stats     HealthStats            `json:"stats,omitempty"`
}

// HealthStats estadísticas de salud
type HealthStats struct {
	Goroutines      int    `json:"goroutines"`
	HeapAllocMB     int    `json:"heap_alloc_mb"`
	HeapSysMB       int    `json:"heap_sys_mb"`
	NumGC           uint32 `json:"num_gc"`
	ActiveTerminals int    `json:"active_terminals"`
}

// NewHostHandler crea un nuevo handler
func NewHostHandler(hostName, version, claudeDir string, terminals *services.TerminalService, claude *services.ClaudeService) *HostHandler {
	return &HostHandler{
		startedAt: time.Now(),
		version:   version,
		hostName:  hostName,
		claudeDir: claudeDir,
		terminals: terminals,
		claude:    claude,
	}
}

// Get godoc
// @Summary      Obtener información del host
// @Description  Retorna información del servidor incluyendo versión, plataforma y estadísticas
// @Tags         host
// @Accept       json
// @Produce      json
// @Success      200  {object}  handlers.APIResponse{data=handlers.HostInfo}
// @Router       /host [get]
// @Security     BasicAuth
func (h *HostHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Obtener estadísticas
	projects, _ := h.claude.ListProjects()
	terminals := h.terminals.List()

	activeTerminals := 0
	for _, t := range terminals {
		if t.Active {
			activeTerminals++
		}
	}

	totalSessions := 0
	for _, p := range projects {
		totalSessions += p.SessionCount
	}

	hostname := h.hostName
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	info := HostInfo{
		ID:        hostname,
		Name:      hostname,
		Version:   h.version,
		Platform:  runtime.GOOS,
		Arch:      runtime.GOARCH,
		ClaudeDir: h.claudeDir,
		StartedAt: h.startedAt,
		Uptime:    time.Since(h.startedAt).Round(time.Second).String(),
		Stats: HostStats{
			ActiveTerminals: activeTerminals,
			TotalProjects:   len(projects),
			TotalSessions:   totalSessions,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    info,
	})
}

// Health godoc
// @Summary      Health check del servicio
// @Description  Retorna el estado de salud del servicio con checks de filesystem, memoria, goroutines y terminales
// @Tags         host
// @Accept       json
// @Produce      json
// @Success      200  {object}  handlers.HealthResponse
// @Failure      503  {object}  handlers.HealthResponse
// @Router       /health [get]
func (h *HostHandler) Health(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]HealthCheck)
	overallStatus := HealthStatusHealthy

	// Check 1: Filesystem access
	fsCheck := h.checkFilesystem()
	checks["filesystem"] = fsCheck
	if fsCheck.Status != HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check 2: Goroutines
	grCheck := h.checkGoroutines()
	checks["goroutines"] = grCheck
	if grCheck.Status == HealthStatusUnhealthy {
		overallStatus = HealthStatusUnhealthy
	} else if grCheck.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check 3: Memory
	memCheck := h.checkMemory()
	checks["memory"] = memCheck
	if memCheck.Status == HealthStatusUnhealthy {
		overallStatus = HealthStatusUnhealthy
	} else if memCheck.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check 4: Terminals
	termCheck := h.checkTerminals()
	checks["terminals"] = termCheck
	if termCheck.Status != HealthStatusHealthy && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	terminals := h.terminals.List()
	activeCount := 0
	for _, t := range terminals {
		if t.Active {
			activeCount++
		}
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startedAt).Round(time.Second).String(),
		Checks:    checks,
		Stats: HealthStats{
			Goroutines:      runtime.NumGoroutine(),
			HeapAllocMB:     int(m.HeapAlloc / 1024 / 1024),
			HeapSysMB:       int(m.HeapSys / 1024 / 1024),
			NumGC:           m.NumGC,
			ActiveTerminals: activeCount,
		},
	}

	// Status code basado en estado
	statusCode := http.StatusOK
	if overallStatus == HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// Ready godoc
// @Summary      Readiness probe para Kubernetes
// @Description  Verifica si el servicio está listo para recibir tráfico
// @Tags         host
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /ready [get]
func (h *HostHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Simple check - filesystem accessible
	if _, err := os.Stat(h.claudeDir); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
			"reason": "claude_dir not accessible",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

// checkFilesystem verifica acceso al filesystem
func (h *HostHandler) checkFilesystem() HealthCheck {
	start := time.Now()
	var result HealthCheck

	if _, err := os.Stat(h.claudeDir); err != nil {
		result = HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "claude_dir not accessible: " + err.Error(),
		}
	} else {
		result = HealthCheck{Status: HealthStatusHealthy}
	}

	duration := time.Since(start)
	result.Duration = duration.String()
	metrics.RecordHealthCheck("filesystem", duration, string(result.Status))
	return result
}

// checkGoroutines verifica cantidad de goroutines
func (h *HostHandler) checkGoroutines() HealthCheck {
	start := time.Now()
	count := runtime.NumGoroutine()
	var result HealthCheck

	// Thresholds
	if count > 10000 {
		result = HealthCheck{
			Status:  HealthStatusUnhealthy,
			Message: "too many goroutines",
		}
	} else if count > 1000 {
		result = HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "high goroutine count",
		}
	} else {
		result = HealthCheck{Status: HealthStatusHealthy}
	}

	duration := time.Since(start)
	result.Duration = duration.String()
	metrics.RecordHealthCheck("goroutines", duration, string(result.Status))
	return result
}

// checkMemory verifica uso de memoria
func (h *HostHandler) checkMemory() HealthCheck {
	start := time.Now()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var result HealthCheck

	heapMB := m.HeapAlloc / 1024 / 1024

	// Thresholds (ajustar según necesidad)
	if heapMB > 2048 { // >2GB
		result = HealthCheck{
			Status:  HealthStatusUnhealthy,
			Message: "heap memory too high",
		}
	} else if heapMB > 512 { // >512MB
		result = HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "high memory usage",
		}
	} else {
		result = HealthCheck{Status: HealthStatusHealthy}
	}

	duration := time.Since(start)
	result.Duration = duration.String()
	metrics.RecordHealthCheck("memory", duration, string(result.Status))
	return result
}

// checkTerminals verifica estado de terminales
func (h *HostHandler) checkTerminals() HealthCheck {
	start := time.Now()
	terminals := h.terminals.List()
	activeCount := 0
	for _, t := range terminals {
		if t.Active {
			activeCount++
		}
	}
	var result HealthCheck

	if activeCount > 50 {
		result = HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "many active terminals",
		}
	} else {
		result = HealthCheck{Status: HealthStatusHealthy}
	}

	duration := time.Since(start)
	result.Duration = duration.String()
	metrics.RecordHealthCheck("terminals", duration, string(result.Status))
	return result
}
