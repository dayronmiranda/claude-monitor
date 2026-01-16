package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"claude-monitor/handlers"
	"claude-monitor/pkg/metrics"
	"claude-monitor/services"
)

// Router maneja el enrutamiento de la API usando Chi
type Router struct {
	chi          chi.Router
	host         *handlers.HostHandler
	sessionRoots *handlers.SessionRootsHandler
	sessions     *handlers.SessionsHandler
	terminals    *handlers.TerminalsHandler
	jobs         *handlers.JobsHandler
	analytics    *handlers.AnalyticsHandler
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
		chi:          chi.NewRouter(),
		host:         handlers.NewHostHandler(hostName, version, claudeDir, terminals, claude),
		sessionRoots: handlers.NewSessionRootsHandler(claude, analytics),
		sessions:     handlers.NewSessionsHandler(claude, terminals, analytics),
		terminals:    handlers.NewTerminalsHandler(terminals, allowedPathPrefixes),
		jobs:         handlers.NewJobsHandler(jobs),
		analytics:    handlers.NewAnalyticsHandler(analytics),
	}
}

// Handler retorna el http.Handler configurado
func (r *Router) Handler() http.Handler {
	return r.chi
}

// SetupRoutes configura todas las rutas usando Chi
func (r *Router) SetupRoutes() {
	// Middleware global de Chi (recovery para evitar panics)
	r.chi.Use(middleware.Recoverer)

	// Rutas públicas (sin auth) - métricas y health
	r.chi.Group(func(router chi.Router) {
		router.Handle("/metrics", metrics.Handler())
		router.Get("/api/health", r.host.Health)
		router.Get("/api/ready", r.host.Ready)
	})

	// Rutas de API (con middlewares)
	r.chi.Route("/api", func(api chi.Router) {
		// Host info
		api.Get("/host", r.host.Get)

		// Session Roots (directorios donde se han ejecutado sesiones de Claude)
		api.Route("/session-roots", func(roots chi.Router) {
			roots.Get("/", r.sessionRoots.List)

			// Rutas con path del session-root
			roots.Route("/{rootPath}", func(root chi.Router) {
				root.Get("/", r.sessionRoots.Get)
				root.Delete("/", r.sessionRoots.Delete)
				root.Get("/activity", r.sessionRoots.GetActivity)

				// Sessions dentro del session-root
				root.Route("/sessions", func(sessions chi.Router) {
					sessions.Get("/", r.sessions.List)
					sessions.Post("/delete", r.sessions.DeleteMultiple)
					sessions.Post("/clean", r.sessions.CleanEmpty)
					sessions.Post("/import", r.sessions.Import)

					sessions.Route("/{sessionID}", func(session chi.Router) {
						session.Get("/", r.sessions.Get)
						session.Delete("/", r.sessions.Delete)
						session.Put("/rename", r.sessions.Rename)
						session.Get("/messages", r.sessions.GetMessages)
						session.Get("/messages/realtime", r.sessions.GetRealTimeMessages)
					})
				})

				// Jobs dentro del session-root
				root.Route("/jobs", func(jobsRouter chi.Router) {
					jobsRouter.Get("/", r.jobs.ListJobs)
					jobsRouter.Post("/", r.jobs.CreateJob)

					jobsRouter.Route("/{jobID}", func(job chi.Router) {
						job.Get("/", r.jobs.GetJob)
						job.Delete("/", r.jobs.DeleteJob)

						// Transiciones de estado
						job.Post("/start", r.jobs.StartJob)
						job.Post("/pause", r.jobs.PauseJob)
						job.Post("/resume", r.jobs.ResumeJob)
						job.Post("/stop", r.jobs.StopJob)
						job.Post("/archive", r.jobs.ArchiveJob)

						// Error handling
						job.Post("/retry", r.jobs.RetryJob)
						job.Post("/discard", r.jobs.DiscardJob)

						// Info
						job.Get("/messages", r.jobs.GetJobMessages)
						job.Get("/actions", r.jobs.GetJobActions)
					})
				})
			})
		})

		// Terminals
		api.Route("/terminals", func(terms chi.Router) {
			terms.Get("/", r.terminals.List)
			terms.Post("/", r.terminals.Create)

			terms.Route("/{terminalID}", func(term chi.Router) {
				term.Get("/", r.terminals.Get)
				term.Delete("/", r.terminals.Delete)

				// WebSocket (sin middleware JSON)
				term.Get("/ws", r.terminals.WebSocket)

				// Operaciones
				term.Post("/kill", r.terminals.Kill)
				term.Post("/resume", r.terminals.Resume)
				term.Post("/resize", r.terminals.Resize)

				// Info
				term.Get("/snapshot", r.terminals.Snapshot)
				term.Get("/claude-state", r.terminals.ClaudeState)
				term.Get("/checkpoints", r.terminals.ClaudeCheckpoints)
				term.Get("/events", r.terminals.ClaudeEvents)
			})
		})

		// Analytics
		api.Route("/analytics", func(anal chi.Router) {
			anal.Get("/global", r.analytics.GetGlobal)
			anal.Get("/session-roots/{rootPath}", r.analytics.GetSessionRoot)
			anal.Post("/invalidate", r.analytics.Invalidate)
			anal.Get("/cache", r.analytics.GetCacheStatus)
		})

		// Filesystem
		api.Get("/filesystem/dir", r.terminals.ListDir)
	})
}

// GetURLParam obtiene un parámetro de URL de Chi
// Wrapper para facilitar migración de handlers
func GetURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
