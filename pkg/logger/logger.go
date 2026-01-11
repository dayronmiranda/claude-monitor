package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"
)

// Logger wrapper para slog con funcionalidad adicional
type Logger struct {
	*slog.Logger
	level *slog.LevelVar
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Config configuración del logger
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output io.Writer
}

// DefaultConfig configuración por defecto
func DefaultConfig() Config {
	return Config{
		Level:  "info",
		Format: "text",
		Output: os.Stdout,
	}
}

// New crea un nuevo logger
func New(cfg Config) *Logger {
	level := new(slog.LevelVar)
	level.Set(parseLevel(cfg.Level))

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Formato más corto para el tiempo
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("15:04:05.000"))
			}
			return a
		},
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  level,
	}
}

// Get obtiene el logger global
func Get() *Logger {
	once.Do(func() {
		defaultLogger = New(DefaultConfig())
	})
	return defaultLogger
}

// Init inicializa el logger global
func Init(cfg Config) *Logger {
	defaultLogger = New(cfg)
	return defaultLogger
}

// SetLevel cambia el nivel de log dinámicamente
func (l *Logger) SetLevel(level string) {
	l.level.Set(parseLevel(level))
}

// With añade campos al logger
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
		level:  l.level,
	}
}

// WithContext crea un logger con contexto
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extraer request_id del contexto si existe
	if reqID := ctx.Value(ContextKeyRequestID); reqID != nil {
		return l.With("request_id", reqID)
	}
	return l
}

// Context keys
type contextKey string

const (
	ContextKeyRequestID contextKey = "request_id"
	ContextKeyLogger    contextKey = "logger"
)

// FromContext obtiene logger del contexto
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ContextKeyLogger).(*Logger); ok {
		return l
	}
	return Get()
}

// WithLogger añade logger al contexto
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ContextKeyLogger, l)
}

// WithRequestID añade request_id al contexto
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// Convenience methods para el logger global

// Debug log de nivel debug
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Info log de nivel info
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn log de nivel warn
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error log de nivel error
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// Helper methods

// Request log para peticiones HTTP
func (l *Logger) Request(method, path string, statusCode int, duration time.Duration) {
	l.Info("HTTP request",
		"method", method,
		"path", path,
		"status", statusCode,
		"duration_ms", duration.Milliseconds(),
	)
}

// Terminal log para operaciones de terminal
func (l *Logger) Terminal(action, terminalID string, args ...any) {
	allArgs := append([]any{"action", action, "terminal_id", terminalID}, args...)
	l.Info("Terminal operation", allArgs...)
}

// WebSocket log para operaciones WebSocket
func (l *Logger) WebSocket(action, terminalID string, args ...any) {
	allArgs := append([]any{"action", action, "terminal_id", terminalID}, args...)
	l.Info("WebSocket", allArgs...)
}

// Session log para operaciones de sesión
func (l *Logger) Session(action, projectPath string, args ...any) {
	allArgs := append([]any{"action", action, "project", projectPath}, args...)
	l.Info("Session operation", allArgs...)
}

// parseLevel convierte string a slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Stats obtiene estadísticas de runtime
func Stats() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"goroutines":  runtime.NumGoroutine(),
		"heap_alloc":  m.HeapAlloc,
		"heap_sys":    m.HeapSys,
		"num_gc":      m.NumGC,
	}
}
