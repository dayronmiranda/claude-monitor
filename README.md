# Claude Monitor

Sistema web completo para monitorear, gestionar y visualizar sesiones de Claude Code CLI en tiempo real.

## Tabla de Contenidos

- [Características](#-características)
- [Arquitectura](#-arquitectura)
- [Inicio Rápido](#-inicio-rápido)
- [Configuración](#-configuración)
- [API REST](#-api-rest)
- [Terminal Virtual](#-terminal-virtual)
- [Detección de Estado Claude](#-detección-de-estado-claude)
- [Sistema de Jobs](#-sistema-de-jobs)
- [Documentación](#-documentación)
- [Desarrollo](#-desarrollo)

---

## Características

### Backend (Go)

- **API REST completa** para proyectos, sesiones, terminales y analytics
- **Emulación de terminal VT100/ANSI** con [Azure/go-ansiterm](https://github.com/Azure/go-ansiterm)
- **Detección de estados de Claude Code CLI** en tiempo real
- **PTY Management** con [creack/pty](https://github.com/creack/pty)
- **WebSocket bidireccional** para terminales interactivas
- **Sistema de Jobs unificado** (sesiones + terminales)
- Lectura y parsing de archivos JSONL de Claude Code
- Autenticación Basic Auth + API Token
- Analytics y estadísticas de uso

### Frontend (React + TypeScript)

- Interfaz moderna con Tailwind CSS
- Gestión de múltiples drivers (hosts remotos)
- Visualización de historial completo de conversaciones
- Terminal web interactiva con xterm.js
- Monitoreo de estado en tiempo real
- Dashboard de analytics

### Emulación de Terminal

- **Screen state tracking** - Estado completo de la pantalla virtual
- **Alternate screen mode** - Soporte para vim, htop, less, etc.
- **Scrollback history** - Historial de scroll
- **Snapshot & reconnection** - Restaurar estado al reconectar
- **Cursor tracking** - Posición y atributos del cursor
- **Color support** - 256 colores + RGB

### Detección de Claude State

- **Estados detectados**: waiting_input, generating, permission_prompt, tool_running, error
- **Modos**: normal, vim, plan, compact
- **Patrones**: 25+ regex para detectar estados específicos
- **Checkpoints**: Tracking para soporte de /rewind
- **Events**: Historial de eventos (PreToolUse, PostToolUse, etc.)

---

## Arquitectura

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Frontend (React + TypeScript)                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │
│  │  Dashboard  │  │  Sessions   │  │  Terminal   │  │ Analytics │  │
│  └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP/WebSocket
┌──────────────────────────────┼──────────────────────────────────────┐
│                      Backend (Go)                                    │
│  ┌───────────────────────────┴───────────────────────────────────┐  │
│  │                         Router                                 │  │
│  │  /api/projects  /api/sessions  /api/terminals  /api/jobs      │  │
│  └───────────────────────────┬───────────────────────────────────┘  │
│                              │                                       │
│  ┌───────────────┐  ┌────────┴────────┐  ┌───────────────────────┐  │
│  │ ClaudeService │  │ TerminalService │  │    JobService         │  │
│  │               │  │                 │  │                       │  │
│  │ - Sessions    │  │ - PTY mgmt      │  │ - Unified view        │  │
│  │ - Messages    │  │ - WebSocket     │  │ - State transitions   │  │
│  │ - JSONL parse │  │ - Screen state  │  │ - Migration           │  │
│  └───────────────┘  │ - Claude detect │  └───────────────────────┘  │
│                     └─────────────────┘                              │
│                              │                                       │
│  ┌───────────────────────────┴───────────────────────────────────┐  │
│  │                    go-ansiterm Layer                           │  │
│  │  ┌─────────────────┐  ┌──────────────────────────────────┐    │  │
│  │  │  ScreenHandler  │  │  ClaudeAwareScreenHandler        │    │  │
│  │  │  - VT100 emu    │  │  - Pattern detection             │    │  │
│  │  │  - Buffer mgmt  │  │  - State machine                 │    │  │
│  │  │  - Scroll       │  │  - Checkpoints                   │    │  │
│  │  └─────────────────┘  └──────────────────────────────────┘    │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Inicio Rápido

### Requisitos

- Go 1.21+
- Node.js 18+
- Git

### Backend

```bash
# Clonar repositorio
git clone https://github.com/dayronmiranda/claude-monitor.git
cd claude-monitor

# Compilar
go build -o claude-monitor .

# Ejecutar
./claude-monitor
```

El servidor inicia en `http://localhost:9090`

### Frontend

```bash
# Clonar repositorio del cliente
git clone https://github.com/dayronmiranda/claude-monitor-client.git
cd claude-monitor-client

# Instalar dependencias
npm install

# Ejecutar en desarrollo
npm run dev
```

El cliente estará en `http://localhost:9001`

### Configurar Conexión

1. Abre http://localhost:9001
2. Ve a "Drivers" en la barra lateral
3. Click en "Add Driver"
4. Configura:
   - **Name**: Local Monitor
   - **URL**: http://localhost:9090
   - **Username**: admin
   - **Password**: admin
5. Click en "Connect"

---

## Configuración

### Variables de Entorno

| Variable | Default | Descripción |
|----------|---------|-------------|
| `CLAUDE_MONITOR_PORT` | `9090` | Puerto del servidor |
| `CLAUDE_MONITOR_HOST` | `0.0.0.0` | Host del servidor |
| `CLAUDE_MONITOR_USERNAME` | `admin` | Usuario Basic Auth |
| `CLAUDE_MONITOR_PASSWORD` | `admin` | Password Basic Auth |
| `CLAUDE_MONITOR_ALLOWED_PATHS` | `/` | Paths permitidos (separados por coma) |
| `CLAUDE_DIR` | `~/.claude` | Directorio de Claude Code |

### Ejemplo con Docker

```bash
docker run -d \
  -p 9090:9090 \
  -v ~/.claude:/root/.claude:ro \
  -e CLAUDE_MONITOR_PASSWORD=secreto \
  claude-monitor
```

---

## API REST

### Endpoints Principales

#### Host
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/host` | Info del host |
| GET | `/api/health` | Health check |
| GET | `/api/ready` | Readiness check |

#### Session Roots (Directorios con sesiones de Claude)
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/session-roots` | Listar session roots |
| GET | `/api/session-roots/{path}` | Obtener session root |
| DELETE | `/api/session-roots/{path}` | Eliminar session root |
| GET | `/api/session-roots/{path}/activity` | Actividad del session root |

#### Sesiones
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/session-roots/{path}/sessions` | Listar sesiones |
| GET | `/api/session-roots/{path}/sessions/{id}` | Obtener sesión |
| GET | `/api/session-roots/{path}/sessions/{id}/messages` | Historial de mensajes |
| GET | `/api/session-roots/{path}/sessions/{id}/messages/realtime` | Mensajes en tiempo real |
| DELETE | `/api/session-roots/{path}/sessions/{id}` | Eliminar sesión |
| PUT | `/api/session-roots/{path}/sessions/{id}/rename` | Renombrar sesión |
| POST | `/api/session-roots/{path}/sessions/delete` | Eliminar múltiples |
| POST | `/api/session-roots/{path}/sessions/clean` | Limpiar vacías |
| POST | `/api/session-roots/{path}/sessions/import` | Importar sesión |

#### Terminales
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/terminals` | Listar terminales |
| POST | `/api/terminals` | Crear terminal |
| GET | `/api/terminals/{id}` | Obtener terminal |
| DELETE | `/api/terminals/{id}` | Eliminar terminal |
| POST | `/api/terminals/{id}/kill` | Terminar proceso |
| POST | `/api/terminals/{id}/resume` | Reanudar terminal |
| POST | `/api/terminals/{id}/resize` | Redimensionar |
| GET | `/api/terminals/{id}/ws` | WebSocket |
| GET | `/api/terminals/{id}/snapshot` | Estado de pantalla |
| GET | `/api/terminals/{id}/claude-state` | Estado de Claude |
| GET | `/api/terminals/{id}/checkpoints` | Checkpoints |
| GET | `/api/terminals/{id}/events` | Historial de eventos |

#### Jobs (Vista Unificada)
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/jobs` | Listar jobs |
| GET | `/api/jobs/{id}` | Obtener job |
| GET | `/api/jobs/{id}/messages` | Mensajes del job |
| POST | `/api/jobs/{id}/connect` | Conectar terminal |
| POST | `/api/jobs/{id}/disconnect` | Desconectar terminal |
| DELETE | `/api/jobs/{id}` | Eliminar job |

#### Analytics
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/analytics/global` | Analytics globales |
| GET | `/api/analytics/session-roots/{path}` | Analytics por session root |
| POST | `/api/analytics/invalidate` | Invalidar cache |
| GET | `/api/analytics/cache` | Estado del cache |

#### Filesystem
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/filesystem/dir` | Listar directorio |

---

## Terminal Virtual

### ScreenState (go-ansiterm)

El sistema emula una terminal VT100/ANSI completa para tracking de estado:

```go
// Crear terminal con screen state
terminal := &Terminal{
    Screen: NewScreenState(80, 24),
}

// Alimentar con output del PTY
terminal.Screen.Feed(data)

// Obtener estado
display := terminal.Screen.GetDisplay()     // []string con líneas
cursor := terminal.Screen.GetCursor()       // (x, y)
inAlt := terminal.Screen.IsInAlternateScreen() // vim/htop mode
```

### Snapshot API

```bash
# Obtener snapshot de pantalla
curl http://localhost:9090/api/terminals/term-123/snapshot
```

```json
{
  "success": true,
  "data": {
    "content": "...",
    "display": ["línea 1", "línea 2", "..."],
    "cursor_x": 0,
    "cursor_y": 5,
    "width": 80,
    "height": 24,
    "in_alternate_screen": false,
    "history": ["líneas anteriores..."]
  }
}
```

### WebSocket Reconnection

Al conectar por WebSocket, se envía automáticamente el snapshot:

```javascript
const ws = new WebSocket('ws://localhost:9090/api/terminals/term-123/ws');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  if (msg.type === 'snapshot') {
    // Restaurar estado de pantalla
    renderSnapshot(msg.snapshot);
  } else if (msg.type === 'output') {
    // Output incremental
    terminal.write(msg.data);
  }
};
```

---

## Detección de Estado Claude

### Estados Detectados

| Estado | Descripción | Patrón de Detección |
|--------|-------------|---------------------|
| `unknown` | Estado inicial | - |
| `waiting_input` | Esperando input | Prompt `>` |
| `generating` | Generando respuesta | Spinner `⠋⠙⠹...` |
| `permission_prompt` | Solicitando permiso | `Allow X to...` `[y/n]` |
| `tool_running` | Ejecutando herramienta | `Running:` `Writing:` |
| `error` | Error detectado | `Error:` |
| `exited` | Sesión terminada | - |

### API de Estado

```bash
# Obtener estado de Claude
curl http://localhost:9090/api/terminals/term-123/claude-state
```

```json
{
  "success": true,
  "data": {
    "state": "generating",
    "mode": "normal",
    "is_generating": true,
    "pending_permission": false,
    "pending_tool": "",
    "last_tool_used": "Read",
    "checkpoint_count": 3,
    "can_rewind": true,
    "active_patterns": ["spinner"],
    "last_activity": "2025-01-16T10:30:00Z"
  }
}
```

### Checkpoints y Events

```bash
# Obtener checkpoints (para /rewind)
curl http://localhost:9090/api/terminals/term-123/checkpoints

# Obtener historial de eventos
curl http://localhost:9090/api/terminals/term-123/events
```

---

## Sistema de Jobs

Jobs unifica sesiones históricas y terminales activas en una sola vista:

```
┌─────────────────────────────────────────────────────────────────┐
│                          Job                                     │
├─────────────────────────────────────────────────────────────────┤
│  ID: job-abc123                                                  │
│  Type: claude                                                    │
│  Status: active                                                  │
│                                                                  │
│  ┌──────────────────┐    ┌──────────────────────────────────┐  │
│  │     Session      │    │          Terminal                │  │
│  │                  │◄───┤                                  │  │
│  │  - JSONL file    │    │  - PTY process                   │  │
│  │  - Messages      │    │  - Screen state                  │  │
│  │  - History       │    │  - Claude detection              │  │
│  └──────────────────┘    └──────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Estados de Job

- `idle` - Sin actividad
- `active` - Terminal conectada y corriendo
- `paused` - Terminal pausada
- `completed` - Sesión finalizada
- `error` - Error en ejecución

---

## Documentación

Toda la documentación se encuentra en el directorio [`docs/`](docs/).

| Documento | Descripción |
|-----------|-------------|
| [docs/README.md](docs/README.md) | Índice de documentación |
| [docs/openapi.yaml](docs/openapi.yaml) | Especificación OpenAPI 3.0 |
| [docs/API.md](docs/API.md) | Guía de uso de la API REST |
| [docs/CLAUDE_STATE.md](docs/CLAUDE_STATE.md) | Sistema de detección de estados y eventos WebSocket |
| [docs/STATE_MACHINE.md](docs/STATE_MACHINE.md) | Máquina de estados de Jobs |
| [docs/JOBS_GUIDE.md](docs/JOBS_GUIDE.md) | Guía del sistema de Jobs |

### Ver documentación interactiva

```bash
# Con Swagger UI
docker run -p 8081:8080 -e SWAGGER_JSON=/api/openapi.yaml \
  -v $(pwd)/docs:/api swaggerapi/swagger-ui

# Con Redoc
npx @redocly/cli preview-docs docs/openapi.yaml
```

---

## Estructura del Proyecto

```
claude-monitor/
├── main.go                    # Punto de entrada
├── router.go                  # Enrutamiento HTTP
├── middleware.go              # CORS, Auth, Logging
├── config.go                  # Configuración
├── go.mod                     # Dependencias Go
│
├── handlers/                  # HTTP Handlers
│   ├── common.go              # Respuestas comunes
│   ├── host.go                # Info del host
│   ├── projects.go            # Gestión de proyectos
│   ├── sessions.go            # Gestión de sesiones
│   ├── terminals.go           # Gestión de terminales
│   ├── jobs.go                # Sistema de jobs
│   └── analytics.go           # Estadísticas
│
├── services/                  # Lógica de negocio
│   ├── claude.go              # Parsing de sesiones
│   ├── terminal.go            # PTY management
│   ├── screen.go              # Emulación VT100 (go-ansiterm)
│   ├── claude_state.go        # Detección de estados Claude
│   ├── job.go                 # Modelo de jobs
│   ├── job_service.go         # Servicio de jobs
│   ├── job_transitions.go     # Transiciones de estado
│   ├── job_migration.go       # Migración de datos
│   └── analytics.go           # Cálculo de estadísticas
│
├── pkg/                       # Paquetes reutilizables
│   ├── errors/                # Manejo de errores
│   ├── logger/                # Logging estructurado
│   └── validator/             # Validación de requests
│
└── docs/                      # Documentación
    ├── README.md              # Índice de documentación
    ├── openapi.yaml           # Especificación OpenAPI 3.0
    ├── API.md                 # API Reference
    ├── CLAUDE_STATE.md        # Claude State Detection
    ├── STATE_MACHINE.md       # Máquina de estados de Jobs
    ├── JOBS_GUIDE.md          # Guía del sistema de Jobs
    ├── IMPLEMENTATION_SUMMARY.md # Resumen de implementación
    ├── TESTING_CHECKLIST.md   # Checklist de testing
    ├── MIGRATION_STRATEGY.md  # Estrategia de migración
    ├── IMPROVEMENT_PLAN.md    # Plan de mejoras
    └── PROJECT_COMPLETE.md    # Estado del proyecto
```

---

## Desarrollo

### Compilar Backend

```bash
go build -o claude-monitor .
```

### Tests

```bash
go test ./...
```

### Frontend (Desarrollo)

```bash
cd claude-monitor-client
npm run dev
```

### Frontend (Producción)

```bash
npm run build
```

---

## Dependencias Principales

### Backend (Go)

| Paquete | Uso |
|---------|-----|
| `github.com/Azure/go-ansiterm` | Parser ANSI/VT100 |
| `github.com/creack/pty` | Pseudoterminal management |
| `github.com/gorilla/websocket` | WebSocket server |

### Frontend

| Paquete | Uso |
|---------|-----|
| `react` | UI Framework |
| `xterm.js` | Terminal emulator |
| `tailwindcss` | Styling |
| `zustand` | State management |

---

## Seguridad

- Basic Authentication (configurable)
- API Token support
- CORS configurado
- Path traversal prevention
- Validación de entrada
- Paths permitidos configurables

---

## Licencia

MIT

---

## Autor

[dayronmiranda](https://github.com/dayronmiranda)

---

**Repositorios:**
- Backend: https://github.com/dayronmiranda/claude-monitor
- Frontend: https://github.com/dayronmiranda/claude-monitor-client
