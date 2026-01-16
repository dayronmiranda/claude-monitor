# Plan de Mejoras Arquitectónicas - Claude Monitor

> **Objetivo:** Transformar Claude Monitor de "funciona en mi máquina" a "listo para producción empresarial"

---

## Resumen Ejecutivo

Este plan está organizado en **5 fases** con prioridades claras. Cada fase tiene dependencias mínimas con las demás, permitiendo trabajo paralelo donde sea posible.

| Fase | Nombre | Prioridad | Impacto |
|------|--------|-----------|---------|
| 1 | Testing & Calidad | CRÍTICA | Alto |
| 2 | Robustez del Código | ALTA | Alto |
| 3 | Seguridad Hardening | ALTA | Crítico |
| 4 | Observabilidad | MEDIA | Medio |
| 5 | Performance & Escalabilidad | MEDIA | Medio |

---

## FASE 1: Testing & Calidad (CRÍTICA)

### 1.1 Infraestructura de Testing

```
tests/
├── unit/                    # Tests unitarios
│   ├── services/
│   │   ├── claude_test.go
│   │   ├── terminal_test.go
│   │   ├── screen_test.go
│   │   ├── claude_state_test.go
│   │   ├── job_service_test.go  # (ya existe)
│   │   └── analytics_test.go
│   ├── handlers/
│   │   ├── host_test.go
│   │   ├── projects_test.go
│   │   ├── sessions_test.go
│   │   ├── terminals_test.go
│   │   ├── jobs_test.go
│   │   └── analytics_test.go
│   └── pkg/
│       ├── validator_test.go    # (ya existe)
│       ├── errors_test.go
│       └── logger_test.go
├── integration/             # Tests de integración
│   ├── api_test.go         # Tests E2E de API REST
│   ├── websocket_test.go   # Tests de WebSocket
│   └── pty_test.go         # Tests de PTY lifecycle
├── fixtures/               # Datos de prueba
│   ├── sessions/           # JSONL de ejemplo
│   └── terminals/          # Estados de terminal
└── testutil/               # Helpers de testing
    ├── mock_services.go
    ├── test_server.go
    └── assertions.go
```

### 1.2 Tests Unitarios Prioritarios

#### A) ClaudeStateDetection - Tests de Patrones Regex
```go
// services/claude_state_test.go
func TestClaudeStateDetection(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected ClaudeState
    }{
        {"spinner_1", "⠋ Processing...", StateGenerating},
        {"spinner_2", "⠙ Thinking...", StateGenerating},
        {"prompt_empty", "> ", StateWaitingInput},
        {"prompt_claude", "claude> ", StateWaitingInput},
        {"permission_yn", "Allow? [y/n]", StatePermissionPrompt},
        {"permission_allow", "Allow Claude to read file.txt?", StatePermissionPrompt},
        {"tool_running", "Running: npm install", StateToolRunning},
        {"tool_writing", "Writing: src/main.go", StateToolRunning},
        {"tool_reading", "Reading: package.json", StateToolRunning},
        {"error_state", "Error: file not found", StateError},
        {"vim_mode", "-- INSERT --", ModeVim},
        // Agregar 50+ casos más para cobertura completa
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            handler := NewClaudeAwareScreenHandler()
            handler.ProcessLine(tc.input)
            assert.Equal(t, tc.expected, handler.GetState())
        })
    }
}
```

#### B) ScreenHandler - Tests de Emulación VT100
```go
// services/screen_test.go
func TestScreenHandler_CursorMovement(t *testing.T) {
    // Test cursor up/down/left/right
    // Test cursor save/restore
    // Test scroll regions
}

func TestScreenHandler_ANSISequences(t *testing.T) {
    // Test colores (16, 256, RGB)
    // Test atributos (bold, dim, underline)
    // Test clear screen/line
}

func TestScreenHandler_AlternateBuffer(t *testing.T) {
    // Test entrada a vim/htop (alternate screen)
    // Test salida y restauración
}
```

#### C) Handlers - Tests con httptest
```go
// handlers/sessions_test.go
func TestSessionsHandler_List(t *testing.T) {
    // Setup mock ClaudeService
    mockService := &MockClaudeService{
        Sessions: []ClaudeSession{...},
    }
    handler := NewSessionsHandler(mockService)

    // Create test request
    req := httptest.NewRequest("GET", "/api/projects/test/sessions", nil)
    w := httptest.NewRecorder()

    // Execute
    handler.List(w, req)

    // Assert
    assert.Equal(t, 200, w.Code)
    var response APIResponse
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.True(t, response.Success)
}
```

### 1.3 Tests de Integración

#### A) API E2E Tests
```go
// tests/integration/api_test.go
func TestAPI_FullWorkflow(t *testing.T) {
    // 1. Start test server
    server := testutil.NewTestServer(t)
    defer server.Close()

    // 2. Create terminal
    resp := server.POST("/api/terminals", CreateTerminalRequest{
        WorkDir: t.TempDir(),
        Command: "echo 'hello'",
    })
    assert.Equal(t, 201, resp.StatusCode)

    // 3. Get terminal
    terminalID := resp.Data["id"].(string)
    resp = server.GET("/api/terminals/" + terminalID)
    assert.Equal(t, 200, resp.StatusCode)

    // 4. Wait for completion
    // 5. Check snapshot
    // 6. Delete terminal
}
```

#### B) WebSocket Tests
```go
// tests/integration/websocket_test.go
func TestWebSocket_TerminalInteraction(t *testing.T) {
    server := testutil.NewTestServer(t)
    defer server.Close()

    // Create terminal with bash
    terminalID := server.CreateTerminal(t, "bash")

    // Connect WebSocket
    ws := server.ConnectWS(t, "/api/terminals/"+terminalID+"/ws")
    defer ws.Close()

    // Should receive initial snapshot
    msg := ws.ReadMessage(t)
    assert.Equal(t, "snapshot", msg.Type)

    // Send command
    ws.WriteMessage(t, "echo hello\n")

    // Should receive output
    msg = ws.ReadMessage(t)
    assert.Contains(t, msg.Data, "hello")
}
```

### 1.4 Mocks e Interfaces

```go
// services/interfaces.go
type ClaudeServiceInterface interface {
    ListProjects() ([]*ClaudeProject, error)
    GetProject(path string) (*ClaudeProject, error)
    ListSessions(projectPath string) ([]*ClaudeSession, error)
    GetSessionMessages(projectPath, sessionID string) ([]*SessionMessage, error)
}

type TerminalServiceInterface interface {
    Create(cfg TerminalConfig) (*Terminal, error)
    Get(id string) (*Terminal, error)
    List() []*TerminalInfo
    Delete(id string) error
}

// Esto permite crear mocks fácilmente para testing
```

### 1.5 Cobertura Objetivo

| Paquete | Cobertura Actual | Objetivo |
|---------|------------------|----------|
| services/claude_state | 0% | 90% |
| services/screen | 0% | 85% |
| services/terminal | 0% | 80% |
| services/job | ~30% | 85% |
| handlers/* | 0% | 75% |
| pkg/validator | ~50% | 90% |
| pkg/errors | 0% | 90% |

**Meta global: 80% de cobertura**

---

## FASE 2: Robustez del Código (ALTA)

### 2.1 Eliminar time.Sleep() - Sincronización Adecuada

#### Problema Actual:
```go
// main.go - gracefulShutdown
time.Sleep(500 * time.Millisecond) // "esperar y rezar"
```

#### Solución: WaitGroup + Channels
```go
// services/terminal.go
type TerminalService struct {
    // ... existing fields
    shutdownWg sync.WaitGroup
    shutdownCh chan struct{}
}

func (s *TerminalService) ShutdownAll(ctx context.Context) error {
    close(s.shutdownCh) // Signal all goroutines

    done := make(chan struct{})
    go func() {
        s.shutdownWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil // All goroutines finished
    case <-ctx.Done():
        return ctx.Err() // Timeout
    }
}

// En cada goroutine de terminal:
func (s *TerminalService) startOutputReader(t *Terminal) {
    s.shutdownWg.Add(1)
    go func() {
        defer s.shutdownWg.Done()

        for {
            select {
            case <-s.shutdownCh:
                return // Graceful shutdown
            default:
                // Read from PTY with timeout
                // ...
            }
        }
    }()
}
```

### 2.2 Mejorar Router - Usar Chi (Lightweight)

#### Problema Actual:
```go
// 255 líneas de if-else con strings.HasPrefix/HasSuffix
if strings.HasSuffix(path, "/sessions/delete") {
    // ...
}
```

#### Solución: Migrar a Chi
```go
// router.go
import "github.com/go-chi/chi/v5"

func (router *Router) SetupRoutes() http.Handler {
    r := chi.NewRouter()

    // Middlewares
    r.Use(middleware.RequestID)
    r.Use(router.metricsMiddleware)
    r.Use(router.loggingMiddleware)
    r.Use(router.corsMiddleware)
    r.Use(router.authMiddleware)

    // API Routes
    r.Route("/api", func(r chi.Router) {
        // Host
        r.Get("/host", router.hostHandler.GetInfo)
        r.Get("/health", router.hostHandler.Health)
        r.Get("/ready", router.hostHandler.Ready)

        // Projects
        r.Route("/projects", func(r chi.Router) {
            r.Get("/", router.projectsHandler.List)
            r.Get("/{path}", router.projectsHandler.Get)
            r.Delete("/{path}", router.projectsHandler.Delete)
            r.Get("/{path}/activity", router.projectsHandler.GetActivity)

            // Sessions (nested)
            r.Route("/{path}/sessions", func(r chi.Router) {
                r.Get("/", router.sessionsHandler.List)
                r.Get("/{id}", router.sessionsHandler.Get)
                r.Get("/{id}/messages", router.sessionsHandler.GetMessages)
                r.Delete("/{id}", router.sessionsHandler.Delete)
                r.Put("/{id}/rename", router.sessionsHandler.Rename)
                r.Post("/delete", router.sessionsHandler.BatchDelete)
                r.Post("/clean", router.sessionsHandler.Clean)
            })
        })

        // Terminals
        r.Route("/terminals", func(r chi.Router) {
            r.Get("/", router.terminalsHandler.List)
            r.Post("/", router.terminalsHandler.Create)
            r.Get("/{id}", router.terminalsHandler.Get)
            r.Delete("/{id}", router.terminalsHandler.Delete)
            r.Get("/{id}/ws", router.terminalsHandler.WebSocket)
            r.Get("/{id}/snapshot", router.terminalsHandler.Snapshot)
            r.Get("/{id}/claude-state", router.terminalsHandler.ClaudeState)
            r.Post("/{id}/resize", router.terminalsHandler.Resize)
        })

        // Jobs
        r.Route("/jobs", func(r chi.Router) {
            r.Get("/", router.jobsHandler.List)
            r.Get("/{id}", router.jobsHandler.Get)
            r.Delete("/{id}", router.jobsHandler.Delete)
        })

        // Analytics
        r.Route("/analytics", func(r chi.Router) {
            r.Get("/global", router.analyticsHandler.Global)
            r.Get("/projects/{path}", router.analyticsHandler.Project)
            r.Post("/invalidate", router.analyticsHandler.Invalidate)
        })
    })

    // Metrics (public)
    r.Handle("/metrics", promhttp.Handler())

    return r
}
```

**Beneficios:**
- Path parameters nativos (`{id}`, `{path}`)
- Middleware chain más limpio
- Subroutes anidadas
- ~50% menos código
- Battle-tested en producción

### 2.3 WebSocket Client Cleanup

#### Problema: Memory leaks potenciales
```go
// Actual: ¿se limpian siempre los clientes?
Clients map[*websocket.Conn]bool
```

#### Solución: Client Manager con Lifecycle
```go
// services/websocket_manager.go
type WebSocketManager struct {
    clients    map[string]map[*websocket.Conn]*ClientInfo
    register   chan *ClientRegistration
    unregister chan *ClientRegistration
    mu         sync.RWMutex
}

type ClientInfo struct {
    Conn         *websocket.Conn
    TerminalID   string
    ConnectedAt  time.Time
    LastActivity time.Time
}

type ClientRegistration struct {
    TerminalID string
    Conn       *websocket.Conn
    Done       chan struct{}
}

func NewWebSocketManager() *WebSocketManager {
    m := &WebSocketManager{
        clients:    make(map[string]map[*websocket.Conn]*ClientInfo),
        register:   make(chan *ClientRegistration, 100),
        unregister: make(chan *ClientRegistration, 100),
    }
    go m.run()
    return m
}

func (m *WebSocketManager) run() {
    // Cleanup ticker for stale connections
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case reg := <-m.register:
            m.addClient(reg)
        case reg := <-m.unregister:
            m.removeClient(reg)
        case <-ticker.C:
            m.cleanupStaleConnections()
        }
    }
}

func (m *WebSocketManager) cleanupStaleConnections() {
    m.mu.Lock()
    defer m.mu.Unlock()

    threshold := time.Now().Add(-5 * time.Minute)
    for termID, clients := range m.clients {
        for conn, info := range clients {
            if info.LastActivity.Before(threshold) {
                // Ping to check if alive
                if err := conn.WriteControl(
                    websocket.PingMessage,
                    nil,
                    time.Now().Add(time.Second),
                ); err != nil {
                    conn.Close()
                    delete(clients, conn)
                    log.Info("Cleaned up stale WebSocket",
                        "terminal", termID,
                        "connected_at", info.ConnectedAt)
                }
            }
        }
    }
}
```

### 2.4 Context Propagation

#### Problema: Contextos no propagados consistentemente
```go
// Algunos servicios ignoran el contexto
func (s *ClaudeService) ListProjects() ([]*ClaudeProject, error)
```

#### Solución: Context-First APIs
```go
// Todas las operaciones aceptan context
func (s *ClaudeService) ListProjects(ctx context.Context) ([]*ClaudeProject, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Pasar context a operaciones de filesystem
    return s.listProjectsWithContext(ctx)
}

// Handlers extraen context del request
func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    projects, err := h.claudeService.ListProjects(ctx)
    if err != nil {
        if errors.Is(err, context.Canceled) {
            // Client disconnected
            return
        }
        // Handle error
    }
    // ...
}
```

### 2.5 Error Handling Mejorado

#### Problema: log.Fatalf en servicios
```go
// Algunos lugares usan log.Fatalf que mata el proceso
log.Fatalf("Error crítico: %v", err)
```

#### Solución: Propagar errores con contexto
```go
// pkg/errors/errors.go - Agregar wrapping
func Wrap(err error, message string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", message, err)
}

func WrapWithCode(err error, code ErrorCode, message string) *APIError {
    return &APIError{
        Code:    code,
        Message: message,
        Details: map[string]interface{}{
            "cause": err.Error(),
        },
    }
}

// Uso en servicios
func (s *TerminalService) Create(cfg TerminalConfig) (*Terminal, error) {
    cmd := exec.Command(cfg.Command, cfg.Args...)

    ptyFile, err := pty.Start(cmd)
    if err != nil {
        return nil, errors.Wrap(err, "failed to start PTY")
    }

    // ... nunca log.Fatalf
}
```

---

## FASE 3: Seguridad Hardening (ALTA)

### 3.1 Configuración Segura por Defecto

```go
// config.go - Defaults seguros
type Config struct {
    // ... existing fields

    // NUEVOS campos de seguridad
    AllowedPathPrefixes  []string `json:"allowed_path_prefixes"`
    MaxTerminals         int      `json:"max_terminals"`
    MaxWebSocketClients  int      `json:"max_websocket_clients"`
    SessionTimeout       Duration `json:"session_timeout"`
    RateLimitPerMinute   int      `json:"rate_limit_per_minute"`
}

func DefaultConfig() *Config {
    homeDir, _ := os.UserHomeDir()

    return &Config{
        Port:                9090,
        Host:                "127.0.0.1",  // Solo localhost por defecto!
        AllowedPathPrefixes: []string{homeDir}, // Solo home dir
        MaxTerminals:        10,
        MaxWebSocketClients: 50,
        SessionTimeout:      Duration(30 * time.Minute),
        RateLimitPerMinute:  100,
    }
}
```

### 3.2 Rate Limiting Middleware

```go
// middleware/ratelimit.go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     rate.Limit(rps),
        burst:    burst,
    }
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    if limiter, exists := rl.limiters[ip]; exists {
        return limiter
    }

    limiter := rate.NewLimiter(rl.rate, rl.burst)
    rl.limiters[ip] = limiter
    return limiter
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := getClientIP(r)
        limiter := rl.getLimiter(ip)

        if !limiter.Allow() {
            errors.WriteError(w, errors.New(
                errors.ErrCodeTooManyRequests,
                "Rate limit exceeded",
            ))
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### 3.3 Input Sanitization

```go
// pkg/sanitize/sanitize.go
package sanitize

import (
    "path/filepath"
    "strings"
)

// Path sanitiza y valida paths del filesystem
func Path(input string) (string, error) {
    // Limpiar path
    cleaned := filepath.Clean(input)

    // Convertir a absoluto
    abs, err := filepath.Abs(cleaned)
    if err != nil {
        return "", fmt.Errorf("invalid path: %w", err)
    }

    // Verificar que no contiene ..
    if strings.Contains(abs, "..") {
        return "", fmt.Errorf("path traversal detected")
    }

    // Verificar symlinks
    real, err := filepath.EvalSymlinks(abs)
    if err == nil {
        abs = real
    }

    return abs, nil
}

// Command sanitiza comandos antes de ejecución
func Command(cmd string, allowedCommands []string) error {
    base := filepath.Base(strings.Fields(cmd)[0])

    for _, allowed := range allowedCommands {
        if base == allowed {
            return nil
        }
    }

    return fmt.Errorf("command not in allowlist: %s", base)
}

// TerminalInput sanitiza input hacia el PTY
func TerminalInput(input []byte) []byte {
    // Filtrar secuencias peligrosas si es necesario
    // Por ahora, permitir todo (el PTY maneja esto)
    return input
}
```

### 3.4 Audit Logging

```go
// pkg/audit/audit.go
package audit

type AuditEvent struct {
    Timestamp   time.Time         `json:"timestamp"`
    RequestID   string            `json:"request_id"`
    UserIP      string            `json:"user_ip"`
    UserAgent   string            `json:"user_agent"`
    Action      string            `json:"action"`
    Resource    string            `json:"resource"`
    ResourceID  string            `json:"resource_id,omitempty"`
    Result      string            `json:"result"` // success, failure, denied
    Details     map[string]any    `json:"details,omitempty"`
}

type AuditLogger struct {
    writer io.Writer
    mu     sync.Mutex
}

func NewAuditLogger(path string) (*AuditLogger, error) {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return nil, err
    }
    return &AuditLogger{writer: f}, nil
}

func (a *AuditLogger) Log(event AuditEvent) {
    a.mu.Lock()
    defer a.mu.Unlock()

    event.Timestamp = time.Now().UTC()
    json.NewEncoder(a.writer).Encode(event)
}

// Middleware de audit
func (a *AuditLogger) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Capture response
        rec := &responseRecorder{ResponseWriter: w, statusCode: 200}

        next.ServeHTTP(rec, r)

        // Log event
        a.Log(AuditEvent{
            RequestID:  r.Header.Get("X-Request-ID"),
            UserIP:     getClientIP(r),
            UserAgent:  r.UserAgent(),
            Action:     r.Method,
            Resource:   r.URL.Path,
            Result:     statusToResult(rec.statusCode),
        })
    })
}
```

### 3.5 Secrets Management

```go
// config/secrets.go
type SecretsProvider interface {
    GetSecret(key string) (string, error)
}

// Environment variables (default)
type EnvSecretsProvider struct{}

func (p *EnvSecretsProvider) GetSecret(key string) (string, error) {
    value := os.Getenv(key)
    if value == "" {
        return "", fmt.Errorf("secret not found: %s", key)
    }
    return value, nil
}

// AWS Secrets Manager (producción)
type AWSSecretsProvider struct {
    client *secretsmanager.Client
    prefix string
}

func (p *AWSSecretsProvider) GetSecret(key string) (string, error) {
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(p.prefix + "/" + key),
    }
    result, err := p.client.GetSecretValue(context.Background(), input)
    if err != nil {
        return "", err
    }
    return *result.SecretString, nil
}

// Uso en config.go
func LoadSecrets(provider SecretsProvider) (*Secrets, error) {
    password, err := provider.GetSecret("CLAUDE_MONITOR_PASSWORD")
    if err != nil {
        return nil, err
    }

    apiToken, _ := provider.GetSecret("CLAUDE_MONITOR_API_TOKEN") // Optional

    return &Secrets{
        Password: password,
        APIToken: apiToken,
    }, nil
}
```

---

## FASE 4: Observabilidad (MEDIA)

### 4.1 Métricas Estructuradas

```go
// pkg/metrics/metrics.go - Expandir métricas
var (
    // HTTP
    HTTPRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "claude_monitor_http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "claude_monitor_http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    // Terminals
    TerminalsActive = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "claude_monitor_terminals_active",
            Help: "Number of active terminals",
        },
    )

    TerminalCreatedTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "claude_monitor_terminals_created_total",
            Help: "Total terminals created",
        },
    )

    // WebSocket
    WebSocketConnectionsActive = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "claude_monitor_websocket_connections_active",
            Help: "Active WebSocket connections",
        },
    )

    WebSocketMessagesSent = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "claude_monitor_websocket_messages_sent_total",
            Help: "Total WebSocket messages sent",
        },
    )

    // Claude State
    ClaudeStateChanges = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "claude_monitor_claude_state_changes_total",
            Help: "Claude state transitions",
        },
        []string{"from_state", "to_state"},
    )

    // PTY
    PTYReadBytes = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "claude_monitor_pty_read_bytes_total",
            Help: "Total bytes read from PTYs",
        },
    )

    PTYWriteBytes = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "claude_monitor_pty_write_bytes_total",
            Help: "Total bytes written to PTYs",
        },
    )
)
```

### 4.2 Distributed Tracing (OpenTelemetry)

```go
// pkg/tracing/tracing.go
package tracing

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(serviceName string, endpoint string) (func(), error) {
    exporter, err := otlptrace.New(
        context.Background(),
        otlptrace.WithEndpoint(endpoint),
        otlptrace.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
        )),
    )

    otel.SetTracerProvider(tp)

    return func() {
        tp.Shutdown(context.Background())
    }, nil
}

// Middleware de tracing
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx, span := otel.Tracer("http").Start(r.Context(), r.URL.Path)
        defer span.End()

        span.SetAttributes(
            attribute.String("http.method", r.Method),
            attribute.String("http.url", r.URL.String()),
        )

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 4.3 Health Checks Mejorados

```go
// handlers/health.go
type HealthStatus struct {
    Status     string                 `json:"status"` // healthy, degraded, unhealthy
    Version    string                 `json:"version"`
    Uptime     string                 `json:"uptime"`
    Checks     map[string]CheckResult `json:"checks"`
    LastCheck  time.Time              `json:"last_check"`
}

type CheckResult struct {
    Status   string `json:"status"`
    Message  string `json:"message,omitempty"`
    Duration string `json:"duration"`
}

func (h *HostHandler) Health(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    checks := make(map[string]CheckResult)

    // Check filesystem access
    checks["filesystem"] = h.checkFilesystem(ctx)

    // Check terminal service
    checks["terminals"] = h.checkTerminals(ctx)

    // Check memory usage
    checks["memory"] = h.checkMemory()

    // Aggregate status
    status := "healthy"
    for _, check := range checks {
        if check.Status == "unhealthy" {
            status = "unhealthy"
            break
        }
        if check.Status == "degraded" {
            status = "degraded"
        }
    }

    health := HealthStatus{
        Status:    status,
        Version:   version.Version,
        Uptime:    time.Since(h.startTime).String(),
        Checks:    checks,
        LastCheck: time.Now(),
    }

    if status == "unhealthy" {
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(health)
}

func (h *HostHandler) checkFilesystem(ctx context.Context) CheckResult {
    start := time.Now()

    // Try to read claude directory
    _, err := os.ReadDir(h.config.ClaudeDir)

    result := CheckResult{
        Duration: time.Since(start).String(),
    }

    if err != nil {
        result.Status = "unhealthy"
        result.Message = err.Error()
    } else {
        result.Status = "healthy"
    }

    return result
}
```

### 4.4 Logging Estructurado Mejorado

```go
// pkg/logger/logger.go - Campos estándar
type LogFields struct {
    RequestID   string
    UserIP      string
    TerminalID  string
    SessionID   string
    ProjectPath string
    Action      string
    Duration    time.Duration
    Error       error
}

func (l *Logger) WithFields(fields LogFields) *slog.Logger {
    attrs := []any{}

    if fields.RequestID != "" {
        attrs = append(attrs, "request_id", fields.RequestID)
    }
    if fields.TerminalID != "" {
        attrs = append(attrs, "terminal_id", fields.TerminalID)
    }
    if fields.SessionID != "" {
        attrs = append(attrs, "session_id", fields.SessionID)
    }
    if fields.Duration > 0 {
        attrs = append(attrs, "duration_ms", fields.Duration.Milliseconds())
    }
    if fields.Error != nil {
        attrs = append(attrs, "error", fields.Error.Error())
    }

    return l.With(attrs...)
}

// Uso
log.WithFields(LogFields{
    RequestID:  reqID,
    TerminalID: termID,
    Action:     "create_terminal",
    Duration:   time.Since(start),
}).Info("Terminal created successfully")
```

---

## FASE 5: Performance & Escalabilidad (MEDIA)

### 5.1 Connection Pooling para I/O

```go
// services/file_pool.go
type FilePool struct {
    maxOpen   int
    openFiles map[string]*pooledFile
    mu        sync.Mutex
    lru       *list.List
}

type pooledFile struct {
    file     *os.File
    path     string
    lastUsed time.Time
    element  *list.Element
}

func NewFilePool(maxOpen int) *FilePool {
    pool := &FilePool{
        maxOpen:   maxOpen,
        openFiles: make(map[string]*pooledFile),
        lru:       list.New(),
    }
    go pool.cleanup()
    return pool
}

func (p *FilePool) Open(path string) (*os.File, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Check if already open
    if pf, exists := p.openFiles[path]; exists {
        pf.lastUsed = time.Now()
        p.lru.MoveToFront(pf.element)
        return pf.file, nil
    }

    // Evict if at capacity
    if len(p.openFiles) >= p.maxOpen {
        p.evictOldest()
    }

    // Open new file
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }

    pf := &pooledFile{
        file:     f,
        path:     path,
        lastUsed: time.Now(),
    }
    pf.element = p.lru.PushFront(pf)
    p.openFiles[path] = pf

    return f, nil
}
```

### 5.2 Regex Compilation Cache

```go
// services/claude_state.go - Optimizar regex
type PatternCache struct {
    patterns map[string]*regexp.Regexp
    mu       sync.RWMutex
}

var patternCache = &PatternCache{
    patterns: make(map[string]*regexp.Regexp),
}

func init() {
    // Pre-compile all patterns at startup
    patterns := []string{
        `^> $`,
        `claude> `,
        `\[y/n\]`,
        `^Running:`,
        `^Writing:`,
        `^Reading:`,
        `[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]`,
        `^Error:`,
        `-- INSERT --`,
        // ... todos los patterns
    }

    for _, p := range patterns {
        patternCache.patterns[p] = regexp.MustCompile(p)
    }
}

func (c *PatternCache) Match(pattern, text string) bool {
    c.mu.RLock()
    re, exists := c.patterns[pattern]
    c.mu.RUnlock()

    if !exists {
        // Fallback: compile on demand
        c.mu.Lock()
        re = regexp.MustCompile(pattern)
        c.patterns[pattern] = re
        c.mu.Unlock()
    }

    return re.MatchString(text)
}
```

### 5.3 Buffer Pool para PTY I/O

```go
// services/buffer_pool.go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}

func (s *TerminalService) readPTY(t *Terminal) {
    for {
        buf := bufferPool.Get().([]byte)

        n, err := t.Pty.Read(buf)
        if err != nil {
            bufferPool.Put(buf)
            if err == io.EOF {
                return
            }
            continue
        }

        // Process data
        data := make([]byte, n)
        copy(data, buf[:n])
        bufferPool.Put(buf)

        // Feed to screen handler
        t.Screen.Feed(data)

        // Broadcast to clients
        t.broadcast(data)
    }
}
```

### 5.4 Caching Layer

```go
// pkg/cache/cache.go
type Cache[T any] struct {
    data     map[string]*cacheEntry[T]
    mu       sync.RWMutex
    ttl      time.Duration
    maxSize  int
}

type cacheEntry[T any] struct {
    value     T
    expiresAt time.Time
}

func NewCache[T any](ttl time.Duration, maxSize int) *Cache[T] {
    c := &Cache[T]{
        data:    make(map[string]*cacheEntry[T]),
        ttl:     ttl,
        maxSize: maxSize,
    }
    go c.cleanup()
    return c
}

func (c *Cache[T]) Get(key string) (T, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    entry, exists := c.data[key]
    if !exists || time.Now().After(entry.expiresAt) {
        var zero T
        return zero, false
    }

    return entry.value, true
}

func (c *Cache[T]) Set(key string, value T) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Evict if at capacity (simple LRU would be better)
    if len(c.data) >= c.maxSize {
        c.evictOne()
    }

    c.data[key] = &cacheEntry[T]{
        value:     value,
        expiresAt: time.Now().Add(c.ttl),
    }
}
```

---

## Cronograma de Implementación

```
                    SEMANA
FASE               1   2   3   4   5   6
───────────────────────────────────────────
1. Testing         ████████████
   - Unit tests    ████
   - Integration       ████
   - Mocks                 ████

2. Robustez            ████████
   - Sync/Sleep        ████
   - Router                ████
   - Error handling        ████

3. Seguridad               ████████
   - Rate limit            ████
   - Audit                     ████
   - Secrets                   ████

4. Observabilidad              ████████
   - Metrics                   ████
   - Tracing                       ████
   - Health                        ████

5. Performance                     ████████
   - Pooling                       ████
   - Caching                           ████
───────────────────────────────────────────
```

---

## Métricas de Éxito

| Métrica | Actual | Objetivo |
|---------|--------|----------|
| Test Coverage | ~5% | 80% |
| Lint Issues | ? | 0 |
| Security Issues | ? | 0 Critical/High |
| Startup Time | ? | < 2s |
| Memory Usage | ? | < 100MB idle |
| P99 Latency | ? | < 100ms |
| Uptime | ? | 99.9% |

---

## Resumen

Este plan transforma Claude Monitor de un proyecto funcional a uno listo para producción empresarial, con:

1. **Testing robusto** - De 5% a 80% de cobertura
2. **Código más limpio** - Sin sleeps, router adecuado, errores propagados
3. **Seguridad hardened** - Rate limiting, audit logs, secrets management
4. **Observabilidad completa** - Métricas, tracing, health checks
5. **Performance optimizado** - Pooling, caching, buffers

*"El código que funciona es solo el comienzo. El código que se puede mantener es el objetivo."*
