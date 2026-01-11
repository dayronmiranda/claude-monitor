package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_monitor_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "claude_monitor_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Terminal metrics
	activeTerminals = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "claude_monitor_active_terminals",
			Help: "Number of active terminals",
		},
	)

	terminalOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_monitor_terminal_operations_total",
			Help: "Total number of terminal operations",
		},
		[]string{"operation"}, // create, kill, resume, delete
	)

	// WebSocket metrics
	activeWebsockets = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "claude_monitor_active_websocket_connections",
			Help: "Number of active WebSocket connections",
		},
	)

	websocketMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_monitor_websocket_messages_total",
			Help: "Total number of WebSocket messages",
		},
		[]string{"direction"}, // in, out
	)

	// Session metrics
	sessionOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_monitor_session_operations_total",
			Help: "Total number of session operations",
		},
		[]string{"operation"}, // list, get, delete, clean
	)

	// Info metric
	buildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "claude_monitor_build_info",
			Help: "Build information",
		},
		[]string{"version"},
	)
)

// Init initializes metrics and registers them
func Init(version string) {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		activeTerminals,
		terminalOperationsTotal,
		activeWebsockets,
		websocketMessagesTotal,
		sessionOperationsTotal,
		buildInfo,
	)

	buildInfo.WithLabelValues(version).Set(1)
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTP metrics

// RecordHTTPRequest records an HTTP request
func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, normalizePath(path), strconv.Itoa(status)).Inc()
	httpRequestDuration.WithLabelValues(method, normalizePath(path)).Observe(duration.Seconds())
}

// Terminal metrics

// SetActiveTerminals sets the number of active terminals
func SetActiveTerminals(count int) {
	activeTerminals.Set(float64(count))
}

// IncActiveTerminals increments active terminals
func IncActiveTerminals() {
	activeTerminals.Inc()
}

// DecActiveTerminals decrements active terminals
func DecActiveTerminals() {
	activeTerminals.Dec()
}

// RecordTerminalOperation records a terminal operation
func RecordTerminalOperation(operation string) {
	terminalOperationsTotal.WithLabelValues(operation).Inc()
}

// WebSocket metrics

// IncWebsocketConnections increments WebSocket connections
func IncWebsocketConnections() {
	activeWebsockets.Inc()
}

// DecWebsocketConnections decrements WebSocket connections
func DecWebsocketConnections() {
	activeWebsockets.Dec()
}

// RecordWebsocketMessage records a WebSocket message
func RecordWebsocketMessage(direction string) {
	websocketMessagesTotal.WithLabelValues(direction).Inc()
}

// Session metrics

// RecordSessionOperation records a session operation
func RecordSessionOperation(operation string) {
	sessionOperationsTotal.WithLabelValues(operation).Inc()
}

// normalizePath normalizes URL paths for metrics to avoid high cardinality
func normalizePath(path string) string {
	// Reemplazar IDs dinámicos con placeholders
	// /api/terminals/uuid -> /api/terminals/:id
	// /api/projects/path/sessions/uuid -> /api/projects/:path/sessions/:id

	if len(path) > 15 && path[:15] == "/api/terminals/" {
		rest := path[15:]
		if len(rest) > 0 && rest != "ws" {
			if idx := indexByte(rest, '/'); idx > 0 {
				return "/api/terminals/:id/" + rest[idx+1:]
			}
			return "/api/terminals/:id"
		}
	}

	if len(path) > 14 && path[:14] == "/api/projects/" {
		return "/api/projects/:path"
	}

	if len(path) > 20 && path[:20] == "/api/analytics/proj" {
		return "/api/analytics/projects/:path"
	}

	return path
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// MetricsMiddleware middleware para registrar métricas HTTP
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrapper para capturar status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		// Registrar métricas (excepto el endpoint de métricas)
		if r.URL.Path != "/metrics" {
			RecordHTTPRequest(r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implementa http.Hijacker para soportar WebSocket
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
}

// Flush implementa http.Flusher
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
