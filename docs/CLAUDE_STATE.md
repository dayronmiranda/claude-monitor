# Claude State Detection System

Sistema de detección y seguimiento del estado de Claude Code CLI en tiempo real.

## Tabla de Contenidos

1. [Arquitectura General](#arquitectura-general)
2. [Flujo de Datos](#flujo-de-datos)
3. [Emulación de Terminal](#emulación-de-terminal)
4. [Detección de Estados](#detección-de-estados)
5. [Patrones de Detección](#patrones-de-detección)
6. [API de Acceso](#api-de-acceso)
7. [Casos de Uso](#casos-de-uso)
8. [Ejemplos de Código](#ejemplos-de-código)

---

## Arquitectura General

El sistema de detección de Claude State está compuesto por tres capas principales:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Frontend (React)                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │  WebSocket   │  │   REST API   │  │   State Display          │  │
│  │   Client     │  │   Polling    │  │   Components             │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────────┘
                             │ HTTP/WS
┌────────────────────────────┼────────────────────────────────────────┐
│                        Backend (Go)                                 │
│                            │                                        │
│  ┌─────────────────────────▼─────────────────────────────────────┐ │
│  │                  TerminalService                               │ │
│  │  ┌──────────────────────────────────────────────────────────┐ │ │
│  │  │                    Terminal                               │ │ │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐ │ │ │
│  │  │  │    PTY      │  │ ScreenState │  │ ClaudeAware      │ │ │ │
│  │  │  │ (creack/pty)│  │ (go-ansiterm│  │ ScreenHandler    │ │ │ │
│  │  │  └──────┬──────┘  └──────┬──────┘  └────────┬─────────┘ │ │ │
│  │  │         │                │                   │           │ │ │
│  │  │         │    ┌───────────┴───────────┐      │           │ │ │
│  │  │         └────►  Raw bytes (ANSI)     ├──────┘           │ │ │
│  │  │              └───────────────────────┘                  │ │ │
│  │  └──────────────────────────────────────────────────────────┘ │ │
│  └───────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────┘
                             │
┌────────────────────────────┼────────────────────────────────────────┐
│                     Claude Code CLI                                 │
│                            │                                        │
│  ┌─────────────────────────▼─────────────────────────────────────┐ │
│  │  PTY (Pseudoterminal)                                         │ │
│  │  - Ejecuta claude CLI                                         │ │
│  │  - Envía secuencias ANSI                                      │ │
│  │  - Recibe input del usuario                                   │ │
│  └───────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────┘
```

### Componentes Clave

| Componente | Archivo | Descripción |
|------------|---------|-------------|
| `ScreenHandler` | `services/screen.go` | Implementación de go-ansiterm AnsiEventHandler |
| `ScreenState` | `services/screen.go` | Wrapper que combina handler + parser |
| `ClaudeAwareScreenHandler` | `services/claude_state.go` | Extensión con detección de Claude |
| `TerminalService` | `services/terminal.go` | Gestión de terminales PTY |

---

## Flujo de Datos

```
┌─────────────┐    ┌─────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Claude CLI  │───►│    PTY      │───►│   readLoop()     │───►│   Broadcast     │
│             │    │             │    │                  │    │   to Clients    │
└─────────────┘    └─────────────┘    └────────┬─────────┘    └─────────────────┘
                                               │
                                               ▼
                   ┌───────────────────────────────────────────────────────┐
                   │                    Feed(data []byte)                   │
                   │                                                        │
                   │  ┌──────────────────┐    ┌──────────────────────────┐ │
                   │  │   ScreenState    │    │ ClaudeAwareScreenHandler │ │
                   │  │                  │    │                          │ │
                   │  │  ┌────────────┐  │    │  ┌────────────────────┐  │ │
                   │  │  │ AnsiParser │  │    │  │ analyzeContent()   │  │ │
                   │  │  │            │  │    │  │                    │  │ │
                   │  │  │ Parse ANSI │  │    │  │ - detectPatterns() │  │ │
                   │  │  │ sequences  │  │    │  │ - updateState()    │  │ │
                   │  │  └─────┬──────┘  │    │  │ - detectCommands() │  │ │
                   │  │        │         │    │  └────────────────────┘  │ │
                   │  │        ▼         │    │                          │ │
                   │  │  ┌────────────┐  │    │  Outputs:                │ │
                   │  │  │ScreenHdlr │  │    │  - ClaudeState           │ │
                   │  │  │            │  │    │  - ClaudeMode            │ │
                   │  │  │ - buffer   │  │    │  - Checkpoints           │ │
                   │  │  │ - cursor   │  │    │  - Events                │ │
                   │  │  │ - colors   │  │    │                          │ │
                   │  │  └────────────┘  │    └──────────────────────────┘ │
                   │  └──────────────────┘                                 │
                   └───────────────────────────────────────────────────────┘
```

### Proceso Detallado

1. **PTY Read**: El proceso `readLoop()` lee bytes del PTY continuamente
2. **Feed to Screens**: Los bytes se alimentan a ambos handlers:
   - `ScreenState`: Procesa secuencias ANSI, actualiza buffer de pantalla
   - `ClaudeAwareScreenHandler`: Analiza patrones específicos de Claude
3. **Broadcast**: Los bytes raw se envían a clientes WebSocket
4. **State Detection**: El handler de Claude detecta estados y emite eventos

---

## Emulación de Terminal

### ScreenHandler (go-ansiterm)

El `ScreenHandler` implementa la interfaz `AnsiEventHandler` de Azure/go-ansiterm para procesar todas las secuencias de escape ANSI/VT100.

#### Estructura de Datos

```go
type Cell struct {
    Char rune   // Carácter Unicode
    FG   int    // Color foreground (0-7, 9=default)
    BG   int    // Color background (0-7, 9=default)
    Bold bool   // Atributo bold
    Dim  bool   // Atributo dim
}

type ScreenHandler struct {
    width, height int      // Dimensiones de pantalla
    buffer    [][]Cell     // Buffer principal
    altBuffer [][]Cell     // Buffer alternativo (vim, htop)
    inAltMode bool         // Indica si está en modo alternativo
    cursorX, cursorY int   // Posición del cursor
    currentFG, currentBG int // Colores actuales
    scrollTop, scrollBottom int // Región de scroll
    history [][]Cell       // Scrollback buffer
}
```

#### Métodos AnsiEventHandler Implementados

| Método | Descripción | Secuencia ANSI |
|--------|-------------|----------------|
| `Print(b byte)` | Imprime carácter | Carácter normal |
| `Execute(b byte)` | Control chars | BS, TAB, LF, CR |
| `CUU(n)` | Cursor arriba | `\e[nA` |
| `CUD(n)` | Cursor abajo | `\e[nB` |
| `CUF(n)` | Cursor derecha | `\e[nC` |
| `CUB(n)` | Cursor izquierda | `\e[nD` |
| `CUP(r,c)` | Posición cursor | `\e[r;cH` |
| `ED(mode)` | Borrar display | `\e[nJ` |
| `EL(mode)` | Borrar línea | `\e[nK` |
| `SGR(params)` | Set Graphics | `\e[n;...m` |
| `SU(n)` | Scroll up | `\e[nS` |
| `SD(n)` | Scroll down | `\e[nT` |
| `DECSTBM(t,b)` | Set scroll region | `\e[t;br` |
| `IL(n)` | Insert líneas | `\e[nL` |
| `DL(n)` | Delete líneas | `\e[nM` |
| `ICH(n)` | Insert caracteres | `\e[n@` |
| `DCH(n)` | Delete caracteres | `\e[nP` |

#### Alternate Screen Mode

El modo alternativo se usa por aplicaciones TUI (vim, htop, less):

```go
// Detectado automáticamente por secuencias:
// \e[?1049h - Activar alternate screen
// \e[?1049l - Desactivar alternate screen

func (h *ScreenHandler) SetAlternateMode(enable bool) {
    if enable && !h.inAltMode {
        h.inAltMode = true
        // Limpiar buffer alternativo
    } else if !enable && h.inAltMode {
        h.inAltMode = false
        // Restaurar buffer principal
    }
}
```

---

## Detección de Estados

### Estados de Claude

```go
type ClaudeState string

const (
    StateUnknown          = "unknown"           // Estado inicial/desconocido
    StateWaitingInput     = "waiting_input"     // Esperando input del usuario
    StateGenerating       = "generating"        // Generando respuesta
    StatePermissionPrompt = "permission_prompt" // Solicitando permiso
    StateToolRunning      = "tool_running"      // Ejecutando herramienta
    StateBackgroundTask   = "background_task"   // Tarea en background
    StateError            = "error"             // Error detectado
    StateExited           = "exited"            // Sesión terminada
)
```

### Máquina de Estados

```
                    ┌─────────────────────────────────────────┐
                    │                                         │
                    ▼                                         │
              ┌─────────┐                                     │
              │ unknown │◄─────────────────────────────┐      │
              └────┬────┘                              │      │
                   │ (inicio sesión)                   │      │
                   ▼                                   │      │
          ┌────────────────┐                           │      │
    ┌────►│ waiting_input  │◄──────────────────────┐   │      │
    │     └───────┬────────┘                       │   │      │
    │             │ (usuario envía prompt)         │   │      │
    │             ▼                                │   │      │
    │     ┌───────────────┐                        │   │      │
    │     │  generating   │─────────────────┐      │   │      │
    │     └───────┬───────┘                 │      │   │      │
    │             │                         │      │   │      │
    │    ┌────────┴────────────┐            │      │   │      │
    │    ▼                     ▼            │      │   │      │
    │ ┌────────────────┐  ┌─────────────┐   │      │   │      │
    │ │permission_prompt│  │tool_running │   │      │   │      │
    │ └───────┬────────┘  └──────┬──────┘   │      │   │      │
    │         │                  │          │      │   │      │
    │         │ (y/n)            │(completa)│      │   │      │
    │         ▼                  │          │      │   │      │
    │    ┌────────┐              │          │      │   │      │
    │    │ error  │◄─────────────┴──────────┘      │   │      │
    │    └───┬────┘                                │   │      │
    │        │                                     │   │      │
    │        └─────────────────────────────────────┘   │      │
    │                                                  │      │
    │     ┌────────────────┐                           │      │
    └─────│background_task │───────────────────────────┘      │
          └───────┬────────┘                                  │
                  │ (/exit, ctrl+c)                           │
                  ▼                                           │
            ┌─────────┐                                       │
            │ exited  │───────────────────────────────────────┘
            └─────────┘
```

### Modos de Edición

```go
type ClaudeMode string

const (
    ModeNormal  = "normal"   // Modo estándar
    ModeVim     = "vim"      // Modo vim (/vim)
    ModePlan    = "plan"     // Modo planificación (/plan)
    ModeCompact = "compact"  // Modo compacto (/compact)
)

type VimSubMode string

const (
    VimInsert  = "insert"   // -- INSERT --
    VimNormal  = "normal"   // -- NORMAL --
    VimVisual  = "visual"   // -- VISUAL --
    VimCommand = "command"  // :command
)
```

---

## Patrones de Detección

### Categorías de Patrones

El sistema utiliza 25+ patrones regex para detectar diferentes estados:

#### Patrones de Permiso (Prioridad: 100)

```go
{Name: "permission_allow", Pattern: `(?i)Allow\s+\w+.*to`, Type: "permission"}
{Name: "permission_yn",    Pattern: `\[y/n\]`,            Type: "permission"}
{Name: "permission_Yn",    Pattern: `\[Y/n\]`,            Type: "permission"}
{Name: "permission_yN",    Pattern: `\[y/N\]`,            Type: "permission"}
```

**Ejemplo de output detectado:**
```
Allow Edit to modify src/main.go? [y/n]
```

#### Patrones de Herramientas (Prioridad: 80)

```go
{Name: "tool_running",   Pattern: `(?i)^Running:`,   Type: "tool"}
{Name: "tool_writing",   Pattern: `(?i)^Writing:`,   Type: "tool"}
{Name: "tool_reading",   Pattern: `(?i)^Reading:`,   Type: "tool"}
{Name: "tool_searching", Pattern: `(?i)^Searching:`, Type: "tool"}
{Name: "tool_editing",   Pattern: `(?i)^Editing:`,   Type: "tool"}
```

**Ejemplo de output detectado:**
```
Running: go build ./...
Writing: src/handlers/user.go
```

#### Patrones de Progreso (Prioridad: 70)

```go
{Name: "spinner",          Pattern: `[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷]`, Type: "progress"}
{Name: "progress_numeric", Pattern: `\[\d+/\d+\]`,                     Type: "progress"}
{Name: "progress_percent", Pattern: `\d+%`,                            Type: "progress"}
```

**Ejemplo de output detectado:**
```
⠋ Generating response...
[3/10] Processing files...
```

#### Patrones de Modo Vim (Prioridad: 85-90)

```go
{Name: "vim_mode",   Pattern: `(?i)vim mode`,   Type: "mode"}
{Name: "vim_insert", Pattern: `-- INSERT --`,   Type: "vim"}
{Name: "vim_normal", Pattern: `-- NORMAL --`,   Type: "vim"}
{Name: "vim_visual", Pattern: `-- VISUAL --`,   Type: "vim"}
```

#### Patrones de Estado (Prioridad: 40-95)

```go
{Name: "error",         Pattern: `(?i)^Error:`,   Type: "status", Priority: 95}
{Name: "warning",       Pattern: `(?i)^Warning:`, Type: "status", Priority: 85}
{Name: "success_check", Pattern: `✓`,             Type: "status", Priority: 40}
{Name: "failure_x",     Pattern: `✗`,             Type: "status", Priority: 40}
```

### Algoritmo de Priorización

```go
func (h *ClaudeAwareScreenHandler) updateStateFromPatterns(patterns []string, content string) {
    // Flags de detección
    hasPermission := false
    hasSpinner    := false
    hasTool       := false
    hasPrompt     := false
    hasError      := false

    for _, p := range patterns {
        switch {
        case strings.HasPrefix(p, "permission"):
            hasPermission = true
        case p == "spinner":
            hasSpinner = true
        case strings.HasPrefix(p, "tool_"):
            hasTool = true
        case strings.HasSuffix(p, "_prompt"):
            hasPrompt = true
        case p == "error":
            hasError = true
        }
    }

    // Determinar estado por prioridad
    if hasError {
        h.stateInfo.State = StateError
    } else if hasPermission {
        h.stateInfo.State = StatePermissionPrompt
    } else if hasTool {
        h.stateInfo.State = StateToolRunning
    } else if hasSpinner {
        h.stateInfo.State = StateGenerating
    } else if hasPrompt {
        h.stateInfo.State = StateWaitingInput
    }
}
```

---

## API de Acceso

### Endpoints REST

#### GET /api/terminals/{id}/claude-state

Retorna el estado actual de Claude para una terminal.

**Response:**
```json
{
  "success": true,
  "data": {
    "state": "generating",
    "mode": "normal",
    "vim_sub_mode": "",
    "permission_mode": "default",
    "is_generating": true,
    "pending_permission": false,
    "pending_tool": "",
    "last_slash_command": "",
    "last_tool_used": "Read",
    "tokens_estimated": 0,
    "cost_estimated": 0,
    "background_tasks": [],
    "last_checkpoint_id": "cp_abc123",
    "checkpoint_count": 3,
    "can_rewind": true,
    "active_patterns": ["spinner", "progress_numeric"],
    "recent_events": [
      {
        "type": "PostToolUse",
        "tool": "Read",
        "timestamp": "2025-01-16T10:30:00Z"
      }
    ],
    "last_activity": "2025-01-16T10:30:05Z",
    "state_changed_at": "2025-01-16T10:29:50Z"
  }
}
```

#### GET /api/terminals/{id}/checkpoints

Retorna historial de checkpoints.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "cp_001",
      "timestamp": "2025-01-16T10:00:00Z",
      "tool_used": "Edit",
      "files_affected": ["src/main.go", "src/handler.go"]
    },
    {
      "id": "cp_002",
      "timestamp": "2025-01-16T10:15:00Z",
      "tool_used": "Write",
      "files_affected": ["src/new_file.go"]
    }
  ],
  "meta": {
    "total": 2
  }
}
```

#### GET /api/terminals/{id}/events

Retorna historial de eventos.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "type": "PreToolUse",
      "tool": "Bash",
      "timestamp": "2025-01-16T10:30:00Z",
      "data": {"command": "go build"}
    },
    {
      "type": "PostToolUse",
      "tool": "Bash",
      "timestamp": "2025-01-16T10:30:05Z",
      "data": {"exit_code": 0}
    }
  ],
  "meta": {
    "total": 2
  }
}
```

#### GET /api/terminals/{id}/snapshot

Retorna snapshot del estado de pantalla.

**Response:**
```json
{
  "success": true,
  "data": {
    "display": [
      "claude-monitor $ claude",
      "",
      "> What would you like me to help you with?",
      "",
      "⠋ Generating response...",
      "",
      ""
    ],
    "cursor": {
      "x": 0,
      "y": 5
    },
    "size": {
      "width": 80,
      "height": 24
    },
    "in_alternate_screen": false
  }
}
```

### WebSocket Messages

El WebSocket en `/api/terminals/{id}/ws` envía automáticamente un snapshot al conectarse:

```json
{
  "type": "snapshot",
  "snapshot": {
    "display": ["..."],
    "cursor": {"x": 0, "y": 0},
    "size": {"width": 80, "height": 24},
    "in_alternate_screen": false
  }
}
```

### Eventos Claude Proactivos (WebSocket Push)

El sistema envía eventos de Claude de forma proactiva via WebSocket. Esto elimina la necesidad de polling constante para detectar cambios de estado.

#### Tipos de Mensajes WebSocket

| Tipo | Descripción |
|------|-------------|
| `output` | Salida raw de la terminal (bytes PTY) |
| `claude:event` | Evento específico de Claude (estructurado) |
| `snapshot` | Estado inicial de pantalla al conectar |
| `closed` | Terminal terminada |
| `shutdown` | Servidor apagándose |

#### Estructura de Evento Claude

Todos los eventos Claude usan el tipo `claude:event` con la siguiente estructura:

```json
{
  "type": "claude:event",
  "event_type": "<subtipo>",
  "data": { ... },
  "timestamp": "2026-01-16T10:30:00Z"
}
```

#### Evento: Cambio de Estado (`state`)

Se emite cuando Claude cambia de estado (generating → waiting_input, etc.)

```json
{
  "type": "claude:event",
  "event_type": "state",
  "data": {
    "old_state": "waiting_input",
    "new_state": "generating"
  },
  "timestamp": "2026-01-16T10:30:00Z"
}
```

**Estados posibles:**
- `unknown` - Estado inicial/desconocido
- `waiting_input` - Esperando input del usuario
- `generating` - Generando respuesta (spinner activo)
- `permission_prompt` - Solicitando permiso
- `tool_running` - Ejecutando herramienta
- `background_task` - Tarea en background
- `error` - Error detectado
- `exited` - Sesión terminada

#### Evento: Solicitud de Permiso (`permission`)

Se emite cuando Claude solicita permiso para usar una herramienta.

```json
{
  "type": "claude:event",
  "event_type": "permission",
  "data": {
    "tool": "Edit"
  },
  "timestamp": "2026-01-16T10:30:00Z"
}
```

#### Evento: Slash Command (`command`)

Se emite cuando se detecta un slash command.

```json
{
  "type": "claude:event",
  "event_type": "command",
  "data": {
    "command": "vim",
    "args": ""
  },
  "timestamp": "2026-01-16T10:30:00Z"
}
```

#### Evento: Checkpoint (`checkpoint`)

Se emite cuando se crea un nuevo checkpoint.

```json
{
  "type": "claude:event",
  "event_type": "checkpoint",
  "data": {
    "id": "cp_abc123",
    "timestamp": "2026-01-16T10:30:00Z",
    "tool_used": "Edit",
    "files_affected": ["src/main.go"]
  },
  "timestamp": "2026-01-16T10:30:00Z"
}
```

#### Evento: Uso de Herramienta (`tool`)

Se emite antes (`pre`) y después (`post`) de usar una herramienta.

```json
{
  "type": "claude:event",
  "event_type": "tool",
  "data": {
    "tool": "Bash",
    "phase": "pre"
  },
  "timestamp": "2026-01-16T10:30:00Z"
}
```

### Ejemplo Completo de Cliente WebSocket

```javascript
const ws = new WebSocket(`ws://localhost:8080/api/terminals/${terminalId}/ws`);

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  switch (msg.type) {
    case 'output':
      // Salida raw de terminal - renderizar en xterm.js
      terminal.write(msg.data);
      break;

    case 'snapshot':
      // Estado inicial al conectar
      restoreScreen(msg.snapshot);
      break;

    case 'claude:event':
      // Eventos específicos de Claude
      handleClaudeEvent(msg);
      break;

    case 'closed':
      showNotification('Terminal cerrada');
      break;

    case 'shutdown':
      showNotification('Servidor apagándose');
      break;
  }
};

function handleClaudeEvent(msg) {
  const { event_type, data, timestamp } = msg;

  switch (event_type) {
    case 'state':
      // Actualizar indicador de estado en UI
      updateStateIndicator(data.new_state);

      // Mostrar notificación si cambió a permission_prompt
      if (data.new_state === 'permission_prompt') {
        showPermissionBanner();
      }

      // Ocultar spinner si terminó de generar
      if (data.old_state === 'generating' && data.new_state !== 'generating') {
        hideLoadingSpinner();
      }
      break;

    case 'permission':
      // Mostrar diálogo de permiso
      showPermissionDialog({
        tool: data.tool,
        onApprove: () => ws.send(JSON.stringify({ type: 'input', data: 'y\n' })),
        onDeny: () => ws.send(JSON.stringify({ type: 'input', data: 'n\n' }))
      });
      break;

    case 'command':
      // Actualizar modo en UI
      if (data.command === 'vim') {
        setEditorMode('vim');
      } else if (data.command === 'plan') {
        setEditorMode('plan');
      }
      // Logging para analytics
      trackSlashCommand(data.command, data.args);
      break;

    case 'checkpoint':
      // Añadir checkpoint a timeline
      addCheckpointToTimeline({
        id: data.id,
        tool: data.tool_used,
        files: data.files_affected,
        time: new Date(timestamp)
      });
      // Habilitar botón de rewind
      enableRewindButton();
      break;

    case 'tool':
      // Mostrar actividad de herramienta
      if (data.phase === 'pre') {
        showToolRunning(data.tool);
      } else {
        hideToolRunning(data.tool);
      }
      break;
  }
}
```

### Comparación: Polling vs Push Proactivo

| Aspecto | Polling (antes) | Push Proactivo (ahora) |
|---------|-----------------|------------------------|
| **Latencia** | 100-500ms (intervalo polling) | ~0ms (instantáneo) |
| **Carga servidor** | Alta (muchas requests) | Baja (solo eventos reales) |
| **Carga red** | Alta (respuestas repetidas) | Baja (solo cambios) |
| **Complejidad cliente** | Alta (manejar intervalos) | Baja (solo listeners) |
| **Detección permisos** | Puede tardar 500ms | Instantánea |

### Arquitectura de Eventos

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Terminal PTY                                │
│                              │                                      │
│                              ▼                                      │
│                      readLoop(terminal)                             │
│                              │                                      │
│              ┌───────────────┼───────────────┐                      │
│              ▼               ▼               ▼                      │
│       ScreenState    ClaudeAwareScreen   broadcast()                │
│       (emulación)    Handler (detección)  │                         │
│                              │             │                        │
│                              ▼             │                        │
│                      analyzeContent()      │                        │
│                              │             │                        │
│              ┌───────────────┼─────┐       │                        │
│              ▼               ▼     ▼       │                        │
│         detectPatterns() detectState()     │                        │
│              │               │     │       │                        │
│              └───────────────┼─────┘       │                        │
│                              │             │                        │
│                              ▼             │                        │
│                      Callbacks activados   │                        │
│                              │             │                        │
│              ┌───────────────┼─────────────┼──────────┐             │
│              ▼               ▼             ▼          ▼             │
│      OnStateChange  OnPermission   OnToolUse    broadcastClaudeEvent│
│              │               │             │          │             │
│              └───────────────┴─────────────┴──────────┘             │
│                              │                                      │
│                              ▼                                      │
│                    WebSocket Clients                                │
│                              │                                      │
│              ┌───────────────┼───────────────┐                      │
│              ▼               ▼               ▼                      │
│         "output"      "claude:event"    "snapshot"                  │
│        (raw bytes)    (estructurado)   (inicial)                    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Casos de Uso

### 1. Monitoreo de Estado en Dashboard

```javascript
// Polling del estado de Claude
async function pollClaudeState(terminalId) {
  const response = await fetch(`/api/terminals/${terminalId}/claude-state`);
  const { data } = await response.json();

  updateUI({
    state: data.state,
    isGenerating: data.is_generating,
    pendingPermission: data.pending_permission
  });
}

// Actualizar cada 500ms
setInterval(() => pollClaudeState('term-123'), 500);
```

### 2. Auto-respuesta a Permisos

```javascript
// Detectar y responder automáticamente a permisos
async function handlePermissions(terminalId) {
  const { data } = await fetch(`/api/terminals/${terminalId}/claude-state`).then(r => r.json());

  if (data.state === 'permission_prompt' && data.pending_tool === 'Read') {
    // Auto-aprobar lecturas de archivo
    ws.send(JSON.stringify({ type: 'input', data: 'y\n' }));
  }
}
```

### 3. Reconexión con Snapshot

```javascript
// Al reconectar WebSocket, recibir snapshot automáticamente
const ws = new WebSocket(`ws://localhost:8080/api/terminals/${terminalId}/ws`);

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  if (msg.type === 'snapshot') {
    // Restaurar estado de pantalla
    terminal.write('\x1b[2J'); // Clear
    msg.snapshot.display.forEach(line => {
      terminal.writeln(line);
    });
  } else if (msg.type === 'output') {
    terminal.write(msg.data);
  }
};
```

### 4. Callbacks de Eventos (Conectados a WebSocket)

Los callbacks ahora están automáticamente conectados para enviar eventos via WebSocket:

```go
// En TerminalService.setupClaudeCallbacks() - services/terminal.go
func (s *TerminalService) setupClaudeCallbacks(terminal *Terminal) {
    if terminal.ClaudeScreen == nil {
        return
    }

    // Cambio de estado → broadcast claude:event type=state
    terminal.ClaudeScreen.OnStateChange = func(old, new ClaudeState) {
        logger.Debug("Claude state change", "old", old, "new", new)
        s.broadcastClaudeEvent(terminal, "state", StateChangeData{
            OldState: string(old),
            NewState: string(new),
        })
    }

    // Permiso solicitado → broadcast claude:event type=permission
    terminal.ClaudeScreen.OnPermissionPrompt = func(tool string) {
        logger.Debug("Claude permission prompt", "tool", tool)
        s.broadcastClaudeEvent(terminal, "permission", PermissionData{
            Tool: tool,
        })
    }

    // Slash command → broadcast claude:event type=command
    terminal.ClaudeScreen.OnSlashCommand = func(cmd, args string) {
        logger.Debug("Claude slash command", "command", cmd)
        s.broadcastClaudeEvent(terminal, "command", SlashCommandData{
            Command: cmd,
            Args:    args,
        })
    }

    // Checkpoint → broadcast claude:event type=checkpoint
    terminal.ClaudeScreen.OnCheckpoint = func(cp Checkpoint) {
        logger.Debug("Claude checkpoint", "id", cp.ID)
        s.broadcastClaudeEvent(terminal, "checkpoint", cp)
    }

    // Uso de herramienta → broadcast claude:event type=tool
    terminal.ClaudeScreen.OnToolUse = func(tool, phase string) {
        logger.Debug("Claude tool use", "tool", tool, "phase", phase)
        s.broadcastClaudeEvent(terminal, "tool", ToolUseData{
            Tool:  tool,
            Phase: phase,
        })
    }
}
```

### 5. Reaccionar a Eventos en Tiempo Real (Frontend)

```javascript
// Ejemplo: Auto-aprobar permisos de lectura
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  if (msg.type === 'claude:event' && msg.event_type === 'permission') {
    if (msg.data.tool === 'Read') {
      // Auto-aprobar lecturas
      ws.send(JSON.stringify({ type: 'input', data: 'y\n' }));
      logAction('Auto-approved Read permission');
    } else {
      // Mostrar diálogo para otras herramientas
      showPermissionDialog(msg.data.tool);
    }
  }
};
```

### 6. Dashboard de Estado en Tiempo Real

```javascript
// Componente React que reacciona a eventos push
function ClaudeStatusDashboard({ terminalId }) {
  const [state, setState] = useState('unknown');
  const [isGenerating, setIsGenerating] = useState(false);
  const [pendingTool, setPendingTool] = useState(null);
  const [checkpoints, setCheckpoints] = useState([]);

  useEffect(() => {
    const ws = new WebSocket(`ws://server/api/terminals/${terminalId}/ws`);

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      if (msg.type === 'claude:event') {
        switch (msg.event_type) {
          case 'state':
            setState(msg.data.new_state);
            setIsGenerating(msg.data.new_state === 'generating');
            if (msg.data.new_state !== 'permission_prompt') {
              setPendingTool(null);
            }
            break;

          case 'permission':
            setPendingTool(msg.data.tool);
            break;

          case 'checkpoint':
            setCheckpoints(prev => [...prev, msg.data]);
            break;
        }
      }
    };

    return () => ws.close();
  }, [terminalId]);

  return (
    <div className="dashboard">
      <StateIndicator state={state} />
      {isGenerating && <Spinner />}
      {pendingTool && <PermissionBanner tool={pendingTool} />}
      <CheckpointTimeline checkpoints={checkpoints} />
    </div>
  );
}
```

---

## Ejemplos de Código

### Inicialización del Handler

```go
// Crear terminal con detección de Claude
func (s *TerminalService) Create(cfg TerminalConfig) (*Terminal, error) {
    terminal := &Terminal{
        ID:     cfg.ID,
        Screen: NewScreenState(80, 24), // Emulación base
    }

    // Agregar detección de Claude si es terminal tipo claude
    if cfg.Type == "claude" {
        terminal.ClaudeScreen = NewClaudeAwareScreenHandler(80, 24)

        // Configurar callbacks
        terminal.ClaudeScreen.OnStateChange = func(old, new ClaudeState) {
            s.broadcastStateChange(terminal.ID, old, new)
        }
    }

    return terminal, nil
}
```

### Procesamiento de Output

```go
// Loop de lectura del PTY
func (s *TerminalService) readLoop(t *Terminal) {
    buf := make([]byte, 4096)

    for {
        n, err := t.Pty.Read(buf)
        if err != nil {
            break
        }

        data := buf[:n]

        // Alimentar screen state (emulación ANSI)
        t.Screen.Feed(data)

        // Alimentar detección de Claude
        if t.ClaudeScreen != nil {
            t.ClaudeScreen.Feed(data)
        }

        // Broadcast a clientes
        s.broadcast(t, data)
    }
}
```

### Obtener Estado Actual

```go
// Handler HTTP para estado de Claude
func (h *TerminalsHandler) ClaudeState(w http.ResponseWriter, r *http.Request) {
    id := extractTerminalID(r.URL.Path)

    state, err := h.terminals.GetClaudeState(id)
    if err != nil {
        WriteNotFound(w, err.Error())
        return
    }

    json.NewEncoder(w).Encode(SuccessResponse(state))
}

// Método del servicio
func (s *TerminalService) GetClaudeState(id string) (*ClaudeStateInfo, error) {
    t, ok := s.terminals[id]
    if !ok {
        return nil, fmt.Errorf("terminal no encontrada")
    }

    if t.ClaudeScreen == nil {
        return nil, fmt.Errorf("terminal no es tipo claude")
    }

    state := t.ClaudeScreen.GetClaudeState()
    return &state, nil
}
```

---

## Estructura de Archivos

```
services/
├── screen.go           # Emulación VT100 con go-ansiterm
│   ├── Cell            # Celda de pantalla
│   ├── ScreenHandler   # Implementación AnsiEventHandler
│   └── ScreenState     # Wrapper con parser
│
├── claude_state.go     # Detección de estados Claude
│   ├── ClaudeState     # Enum de estados
│   ├── ClaudeMode      # Modos de edición
│   ├── OutputPattern   # Patrones de detección
│   ├── HookEvent       # Eventos detectados
│   ├── Checkpoint      # Puntos de control
│   └── ClaudeAwareScreenHandler  # Handler extendido
│
└── terminal.go         # Servicio de terminales
    ├── Terminal        # Estructura de terminal
    └── TerminalService # Gestión de terminales
```

---

## Dependencias

- **github.com/Azure/go-ansiterm**: Parser de secuencias ANSI
- **github.com/creack/pty**: Gestión de pseudoterminales
- **github.com/gorilla/websocket**: Comunicación WebSocket
