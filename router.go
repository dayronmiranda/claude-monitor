package main

import (
	"net/http"
	"strings"

	"claude-monitor/handlers"
	"claude-monitor/services"
)

// Router maneja el enrutamiento de la API
type Router struct {
	host      *handlers.HostHandler
	projects  *handlers.ProjectsHandler
	sessions  *handlers.SessionsHandler
	terminals *handlers.TerminalsHandler
	jobs      *handlers.JobsHandler
	analytics *handlers.AnalyticsHandler
}

// NewRouter crea un nuevo router con todos los handlers
func NewRouter(
	claude *services.ClaudeService,
	terminals *services.TerminalService,
	jobs *services.JobService,
	analytics *services.AnalyticsService,
	hostName, version, claudeDir string,
	allowedPathPrefixes []string,
) *Router {
	return &Router{
		host:      handlers.NewHostHandler(hostName, version, claudeDir, terminals, claude),
		projects:  handlers.NewProjectsHandler(claude, analytics),
		sessions:  handlers.NewSessionsHandler(claude, terminals, analytics),
		terminals: handlers.NewTerminalsHandler(terminals, allowedPathPrefixes),
		jobs:      handlers.NewJobsHandler(jobs),
		analytics: handlers.NewAnalyticsHandler(analytics),
	}
}

// SetupRoutes configura todas las rutas
func (r *Router) SetupRoutes(mux *http.ServeMux) {
	// Host
	mux.HandleFunc("/api/host", r.routeHost)
	mux.HandleFunc("/api/health", r.host.Health)
	mux.HandleFunc("/api/ready", r.host.Ready)

	// Projects
	mux.HandleFunc("/api/projects", r.routeProjects)
	mux.HandleFunc("/api/projects/", r.routeProjectsWithPath)

	// Jobs (unified Sessions + Terminals)
	handlers.RegisterJobsRoutes(mux, r.jobs)

	// Terminals
	mux.HandleFunc("/api/terminals", r.routeTerminals)
	mux.HandleFunc("/api/terminals/", r.routeTerminalsWithID)

	// Analytics
	mux.HandleFunc("/api/analytics/global", r.analytics.GetGlobal)
	mux.HandleFunc("/api/analytics/projects/", r.analytics.GetProject)
	mux.HandleFunc("/api/analytics/invalidate", r.analytics.Invalidate)
	mux.HandleFunc("/api/analytics/cache", r.analytics.GetCacheStatus)

	// Filesystem
	mux.HandleFunc("/api/filesystem/dir", r.terminals.ListDir)
}

// routeHost enruta peticiones de host
func (r *Router) routeHost(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.host.Get(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeProjects enruta peticiones de proyectos (sin path)
func (r *Router) routeProjects(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.projects.List(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeProjectsWithPath enruta peticiones de proyectos con path
func (r *Router) routeProjectsWithPath(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/api/projects/")

	// Detectar si es una ruta de sesiones
	if strings.Contains(path, "/sessions") {
		r.routeSessions(w, req)
		return
	}

	// Detectar si es una ruta de actividad
	if strings.HasSuffix(path, "/activity") {
		r.projects.GetActivity(w, req)
		return
	}

	// Es una petición de proyecto específico
	switch req.Method {
	case http.MethodGet:
		r.projects.Get(w, req)
	case http.MethodDelete:
		r.projects.Delete(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeSessions enruta peticiones de sesiones
func (r *Router) routeSessions(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// POST /api/projects/{path}/sessions/delete
	if strings.HasSuffix(path, "/sessions/delete") && req.Method == http.MethodPost {
		r.sessions.DeleteMultiple(w, req)
		return
	}

	// POST /api/projects/{path}/sessions/clean
	if strings.HasSuffix(path, "/sessions/clean") && req.Method == http.MethodPost {
		r.sessions.CleanEmpty(w, req)
		return
	}

	// POST /api/projects/{path}/sessions/import
	if strings.HasSuffix(path, "/sessions/import") && req.Method == http.MethodPost {
		r.sessions.Import(w, req)
		return
	}

	// PUT /api/projects/{path}/sessions/{id}/rename
	if strings.HasSuffix(path, "/rename") && req.Method == http.MethodPut {
		r.sessions.Rename(w, req)
		return
	}

	// GET /api/projects/{path}/sessions/{id}/messages/realtime
	if strings.HasSuffix(path, "/messages/realtime") && req.Method == http.MethodGet {
		r.sessions.GetRealTimeMessages(w, req)
		return
	}

	// GET /api/projects/{path}/sessions/{id}/messages
	if strings.HasSuffix(path, "/messages") && req.Method == http.MethodGet {
		r.sessions.GetMessages(w, req)
		return
	}

	// Extraer si tiene ID de sesión
	parts := strings.Split(path, "/sessions/")
	hasSessionID := len(parts) == 2 && parts[1] != "" && !strings.Contains(parts[1], "/")

	if hasSessionID {
		// /api/projects/{path}/sessions/{id}
		switch req.Method {
		case http.MethodGet:
			r.sessions.Get(w, req)
		case http.MethodDelete:
			r.sessions.Delete(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /api/projects/{path}/sessions
	switch req.Method {
	case http.MethodGet:
		r.sessions.List(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeTerminals enruta peticiones de terminales (sin ID)
func (r *Router) routeTerminals(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.terminals.List(w, req)
	case http.MethodPost:
		r.terminals.Create(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeTerminalsWithID enruta peticiones de terminales con ID
func (r *Router) routeTerminalsWithID(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// WebSocket
	if strings.HasSuffix(path, "/ws") {
		r.terminals.WebSocket(w, req)
		return
	}

	// POST /api/terminals/{id}/kill
	if strings.HasSuffix(path, "/kill") && req.Method == http.MethodPost {
		r.terminals.Kill(w, req)
		return
	}

	// POST /api/terminals/{id}/resume
	if strings.HasSuffix(path, "/resume") && req.Method == http.MethodPost {
		r.terminals.Resume(w, req)
		return
	}

	// POST /api/terminals/{id}/resize
	if strings.HasSuffix(path, "/resize") && req.Method == http.MethodPost {
		r.terminals.Resize(w, req)
		return
	}

	// GET /api/terminals/{id}/snapshot
	if strings.HasSuffix(path, "/snapshot") && req.Method == http.MethodGet {
		r.terminals.Snapshot(w, req)
		return
	}

	// /api/terminals/{id}
	switch req.Method {
	case http.MethodGet:
		r.terminals.Get(w, req)
	case http.MethodDelete:
		r.terminals.Delete(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
