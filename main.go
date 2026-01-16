package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"claude-monitor/middleware"
	"claude-monitor/pkg/logger"
	"claude-monitor/pkg/metrics"
	"claude-monitor/services"
)

const Version = "2.1.0"

func main() {
	// Flags
	var (
		port            int
		host            string
		configPath      string
		shutdownTimeout int
		logLevel        string
		logFormat       string
	)

	flag.IntVar(&port, "port", 0, "Puerto del servidor (default: 9090)")
	flag.StringVar(&host, "host", "", "Host del servidor (default: 0.0.0.0)")
	flag.StringVar(&configPath, "config", "", "Ruta al archivo de configuración")
	flag.IntVar(&shutdownTimeout, "shutdown-timeout", 30, "Timeout de shutdown en segundos")
	flag.StringVar(&logLevel, "log-level", "info", "Nivel de log (debug, info, warn, error)")
	flag.StringVar(&logFormat, "log-format", "text", "Formato de log (text, json)")
	flag.Parse()

	// Inicializar logger
	log := logger.Init(logger.Config{
		Level:  logLevel,
		Format: logFormat,
	})

	// Inicializar métricas
	metrics.Init(Version)

	// Cargar configuración
	if configPath == "" {
		configPath = filepath.Join(getExecutableDir(), "config.json")
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Warn("Error cargando configuración, usando valores por defecto",
			"path", configPath,
			"error", err,
		)
		cfg = DefaultConfig()
	}

	// Override con flags
	if port != 0 {
		cfg.Port = port
	}
	if host != "" {
		cfg.Host = host
	}

	// Guardar config global
	config = cfg

	log.Info("Configuración cargada",
		"path", configPath,
		"port", cfg.Port,
		"host", cfg.Host,
	)

	// Inicializar servicios
	dataDir := getExecutableDir()

	claudeService := services.NewClaudeService(cfg.ClaudeDir)

	// Inicializar nombres de sesiones
	if err := services.InitSessionNames(dataDir); err != nil {
		logger.Warn("Error cargando nombres de sesiones", "error", err)
	}

	terminalService := services.NewTerminalService(dataDir, cfg.AllowedPathPrefixes...)
	analyticsService := services.NewAnalyticsService(
		claudeService,
		time.Duration(cfg.CacheDurationMinutes)*time.Minute,
	)

	// Inicializar JobService
	jobsDir := filepath.Join(dataDir, "jobs")
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		log.Warn("Error creando directorio de jobs", "error", err)
	}
	jobService := services.NewJobService()
	jobService.SetJobsDir(jobsDir)
	if err := jobService.LoadJobsFromDisk(); err != nil {
		log.Warn("Error cargando jobs del disco", "error", err)
	}

	// Crear router con Chi
	router := NewRouter(
		claudeService,
		terminalService,
		jobService,
		analyticsService,
		cfg.HostName,
		Version,
		cfg.ClaudeDir,
		cfg.AllowedPathPrefixes,
	)

	// Configurar rutas
	router.SetupRoutes()

	// Construir cadena de middlewares
	middlewares := []func(http.Handler) http.Handler{}

	// Rate limiting (si está habilitado)
	if cfg.RateLimitEnabled {
		rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
		middlewares = append(middlewares, rateLimiter.Middleware)
		log.Info("Rate limiting habilitado", "rps", cfg.RateLimitRPS, "burst", cfg.RateLimitBurst)
	}

	// Middlewares estándar (orden: rate_limit -> metrics -> logging -> cors -> auth -> json)
	middlewares = append(middlewares,
		metrics.MetricsMiddleware,
		LoggingMiddleware,
		CORSMiddleware,
		AuthMiddleware,
		JSONMiddleware,
	)

	// Aplicar middlewares (Chi ya tiene recoverer)
	handler := ChainMiddleware(router.Handler(), middlewares...)

	// Crear servidor HTTP con configuración
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para errores del servidor
	serverErr := make(chan error, 1)

	// Iniciar servidor en goroutine
	go func() {
		log.Info("Servidor iniciando",
			"version", Version,
			"address", addr,
			"claude_dir", cfg.ClaudeDir,
		)
		printEndpoints(log, cfg.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Configurar captura de señales
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Esperar señal o error
	select {
	case err := <-serverErr:
		log.Error("Error iniciando servidor", "error", err)
		os.Exit(1)
	case sig := <-sigChan:
		log.Info("Señal recibida, iniciando shutdown", "signal", sig.String())
	}

	// Iniciar graceful shutdown
	gracefulShutdown(log, server, terminalService, time.Duration(shutdownTimeout)*time.Second)
}

// gracefulShutdown realiza un shutdown ordenado sin usar time.Sleep
func gracefulShutdown(log *logger.Logger, server *http.Server, terminalService *services.TerminalService, timeout time.Duration) {
	// Crear context con timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Info("Iniciando graceful shutdown", "timeout", timeout.String())

	// Canal para coordinar pasos de shutdown
	done := make(chan struct{})

	go func() {
		// 1. Terminar todas las terminales activas (ahora usa channels internamente)
		log.Info("Terminando terminales activas")
		terminalService.ShutdownAllWithTimeout(timeout / 2) // Usar mitad del timeout para terminales

		// 2. Persistir estado final
		log.Info("Persistiendo estado final")
		terminalService.PersistState()

		close(done)
	}()

	// Esperar terminación de terminales o timeout
	select {
	case <-done:
		log.Debug("Terminales cerradas correctamente")
	case <-ctx.Done():
		log.Warn("Timeout esperando cierre de terminales")
	}

	// 3. Shutdown del servidor HTTP (deja de aceptar nuevas conexiones)
	log.Info("Cerrando servidor HTTP")
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Error en shutdown del servidor", "error", err)
	}

	log.Info("Shutdown completado")
}

// getExecutableDir retorna el directorio del ejecutable
func getExecutableDir() string {
	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(ex)
}

// printEndpoints imprime los endpoints disponibles
func printEndpoints(log *logger.Logger, port int) {
	log.Debug("Endpoints disponibles",
		"host", []string{"GET /api/host", "GET /api/health"},
		"projects", []string{"GET /api/projects", "GET /api/projects/{path}", "DELETE /api/projects/{path}"},
		"sessions", []string{"GET /api/projects/{path}/sessions", "DELETE /api/projects/{path}/sessions/{id}"},
		"terminals", []string{"GET /api/terminals", "POST /api/terminals", "WS /api/terminals/{id}/ws"},
		"analytics", []string{"GET /api/analytics/global", "GET /api/analytics/projects/{path}"},
	)
}
