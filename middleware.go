package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"claude-monitor/pkg/logger"
)

// responseWriter wrapper para capturar status code
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

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

// generateRequestID genera un ID único para la petición
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// LoggingMiddleware añade logging estructurado a cada petición
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generar request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Añadir request_id al contexto
		ctx := logger.WithRequestID(r.Context(), requestID)

		// Logger con request_id
		log := logger.Get().With(
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
		)

		// Añadir logger al contexto
		ctx = logger.WithLogger(ctx, log)

		// Añadir request_id al response header
		w.Header().Set("X-Request-ID", requestID)

		// Capturar tiempo
		start := time.Now()

		// Wrapper para capturar status code
		wrapped := newResponseWriter(w)

		// Ejecutar handler
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Log de la petición (excepto health checks)
		if r.URL.Path != "/api/health" {
			duration := time.Since(start)
			log.Request(r.Method, r.URL.Path, wrapped.statusCode, duration)
		}
	})
}

// CORSMiddleware añade headers CORS con validación de origen
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Si hay orígenes configurados, validar
		if len(config.AllowedOrigins) > 0 {
			if isOriginAllowed(origin, config.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				// Origen no permitido - log y continuar sin CORS
				log := logger.FromContext(r.Context())
				log.Warn("CORS: origen no permitido", "origin", origin)
			}
		} else {
			// Sin restricciones - permitir todos (modo desarrollo)
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Token, X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Preflight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed verifica si un origen está en la lista permitida
func isOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true // Peticiones sin Origin (mismo origen)
	}
	for _, o := range allowed {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

// AuthMiddleware valida autenticación via Basic Auth o API Token
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Endpoints públicos (sin autenticación)
		if r.URL.Path == "/metrics" || r.URL.Path == "/api/health" || r.URL.Path == "/api/ready" {
			next.ServeHTTP(w, r)
			return
		}

		log := logger.FromContext(r.Context())

		// Check API Token first
		token := r.Header.Get("X-API-Token")
		if token != "" && config.APIToken != "" && token == config.APIToken {
			next.ServeHTTP(w, r)
			return
		}

		// Check query param token (for WebSocket connections)
		queryToken := r.URL.Query().Get("token")
		if queryToken != "" && config.APIToken != "" && queryToken == config.APIToken {
			next.ServeHTTP(w, r)
			return
		}

		// Check Basic Auth
		user, pass, ok := r.BasicAuth()
		if ok && user == config.Username && pass == config.Password {
			next.ServeHTTP(w, r)
			return
		}

		// Check query params for basic auth (WebSocket fallback)
		queryUser := r.URL.Query().Get("user")
		queryPass := r.URL.Query().Get("pass")
		if queryUser == config.Username && queryPass == config.Password {
			next.ServeHTTP(w, r)
			return
		}

		// Unauthorized
		log.Warn("Authentication failed",
			"ip", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
		w.Header().Set("WWW-Authenticate", `Basic realm="Claude Monitor API"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// JSONMiddleware añade Content-Type JSON a las respuestas
func JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Solo para rutas /api que no sean WebSocket
		if strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasSuffix(r.URL.Path, "/ws") {
			w.Header().Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

// ChainMiddleware encadena múltiples middlewares
func ChainMiddleware(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
