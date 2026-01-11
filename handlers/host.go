package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

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
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
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

// Get GET /api/host
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

// Health GET /api/health
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

// Ready GET /api/ready (para k8s readiness probe)
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
	if _, err := os.Stat(h.claudeDir); err != nil {
		return HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "claude_dir not accessible: " + err.Error(),
		}
	}
	return HealthCheck{Status: HealthStatusHealthy}
}

// checkGoroutines verifica cantidad de goroutines
func (h *HostHandler) checkGoroutines() HealthCheck {
	count := runtime.NumGoroutine()

	// Thresholds
	if count > 10000 {
		return HealthCheck{
			Status:  HealthStatusUnhealthy,
			Message: "too many goroutines",
		}
	}
	if count > 1000 {
		return HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "high goroutine count",
		}
	}
	return HealthCheck{Status: HealthStatusHealthy}
}

// checkMemory verifica uso de memoria
func (h *HostHandler) checkMemory() HealthCheck {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	heapMB := m.HeapAlloc / 1024 / 1024

	// Thresholds (ajustar según necesidad)
	if heapMB > 2048 { // >2GB
		return HealthCheck{
			Status:  HealthStatusUnhealthy,
			Message: "heap memory too high",
		}
	}
	if heapMB > 512 { // >512MB
		return HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "high memory usage",
		}
	}
	return HealthCheck{Status: HealthStatusHealthy}
}

// checkTerminals verifica estado de terminales
func (h *HostHandler) checkTerminals() HealthCheck {
	terminals := h.terminals.List()
	activeCount := 0
	for _, t := range terminals {
		if t.Active {
			activeCount++
		}
	}

	if activeCount > 50 {
		return HealthCheck{
			Status:  HealthStatusDegraded,
			Message: "many active terminals",
		}
	}
	return HealthCheck{Status: HealthStatusHealthy}
}
