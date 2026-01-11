# Claude Monitor Driver

Driver/Agent para gestionar sesiones de Claude Code remotamente. Expone una API REST para gestionar proyectos, sesiones y terminales PTY.

## Instalación

```bash
go build -o claude-monitor .
```

## Configuración

### Variables de Entorno (Requeridas para Producción)

```bash
# Autenticación (al menos una es requerida)
export CLAUDE_MONITOR_PASSWORD=tu_password_seguro
export CLAUDE_MONITOR_API_TOKEN=tu_token_seguro
export CLAUDE_MONITOR_USERNAME=admin  # opcional, default: admin
```

### Archivo de Configuración (config.json)

```json
{
  "port": 9090,
  "host": "0.0.0.0",
  "host_name": "mi-servidor",
  "allowed_origins": ["http://localhost:9001"],
  "allowed_path_prefixes": ["/root", "/home", "/var/www"],
  "claude_dir": "/root/.claude/projects",
  "working_dir": "/root",
  "cache_duration_minutes": 5
}
```

### Flags de Línea de Comandos

```bash
./claude-monitor \
  --port 9090 \
  --host 0.0.0.0 \
  --config /path/to/config.json \
  --shutdown-timeout 30 \
  --log-level info \
  --log-format text
```

## Ejecución

```bash
# Desarrollo
CLAUDE_MONITOR_PASSWORD=admin ./claude-monitor

# Producción
CLAUDE_MONITOR_PASSWORD=secure_password \
CLAUDE_MONITOR_API_TOKEN=secure_token \
./claude-monitor --log-format json
```

## API Endpoints

### Host
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/host` | Información del host |
| GET | `/api/health` | Health check detallado |
| GET | `/api/ready` | Readiness check (k8s) |

### Projects
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/projects` | Listar proyectos |
| GET | `/api/projects/{path}` | Obtener proyecto |
| DELETE | `/api/projects/{path}` | Eliminar proyecto |
| GET | `/api/projects/{path}/activity` | Actividad del proyecto |

### Sessions
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/projects/{path}/sessions` | Listar sesiones |
| GET | `/api/projects/{path}/sessions/{id}` | Obtener sesión |
| DELETE | `/api/projects/{path}/sessions/{id}` | Eliminar sesión |
| POST | `/api/projects/{path}/sessions/delete` | Eliminar múltiples |
| POST | `/api/projects/{path}/sessions/clean` | Limpiar vacías |
| POST | `/api/projects/{path}/sessions/import` | Importar sesión |

### Terminals
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/terminals` | Listar terminales |
| POST | `/api/terminals` | Crear terminal |
| GET | `/api/terminals/{id}` | Obtener terminal |
| DELETE | `/api/terminals/{id}` | Eliminar terminal |
| POST | `/api/terminals/{id}/kill` | Terminar terminal |
| POST | `/api/terminals/{id}/resume` | Reanudar terminal |
| POST | `/api/terminals/{id}/resize` | Redimensionar |
| WS | `/api/terminals/{id}/ws` | WebSocket |

### Analytics
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/analytics/global` | Estadísticas globales |
| GET | `/api/analytics/projects/{path}` | Estadísticas de proyecto |
| POST | `/api/analytics/invalidate` | Invalidar caché |
| GET | `/api/analytics/cache` | Estado del caché |

### Filesystem
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/filesystem/dir?path=/path` | Listar directorio |

### Métricas
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/metrics` | Métricas Prometheus |

## Autenticación

### Basic Auth
```bash
curl -u admin:password http://localhost:9090/api/host
```

### API Token
```bash
curl -H "X-API-Token: your_token" http://localhost:9090/api/host
```

### WebSocket (query params)
```javascript
new WebSocket('ws://localhost:9090/api/terminals/id/ws?token=your_token')
// o
new WebSocket('ws://localhost:9090/api/terminals/id/ws?user=admin&pass=password')
```

## Métricas Prometheus

Métricas disponibles en `/metrics`:

- `claude_monitor_http_requests_total` - Total de requests HTTP
- `claude_monitor_http_request_duration_seconds` - Duración de requests
- `claude_monitor_active_terminals` - Terminales activas
- `claude_monitor_terminal_operations_total` - Operaciones de terminal
- `claude_monitor_active_websocket_connections` - Conexiones WebSocket
- `claude_monitor_websocket_messages_total` - Mensajes WebSocket
- `claude_monitor_session_operations_total` - Operaciones de sesión
- `claude_monitor_build_info` - Información de build

## Health Checks

### /api/health
Retorna estado detallado con checks de:
- filesystem: Acceso a claude_dir
- goroutines: Cantidad de goroutines
- memory: Uso de memoria heap
- terminals: Estado de terminales

### /api/ready
Check simple para readiness probes de Kubernetes.

## Seguridad

- Credenciales solo via variables de entorno
- CORS configurable con `allowed_origins`
- Validación de paths con `allowed_path_prefixes`
- Detección de path traversal
- Rate limiting en WebSocket (ping/pong)

## Desarrollo

```bash
# Tests
go test ./...

# Build
go build -o claude-monitor .

# Run con logs debug
./claude-monitor --log-level debug
```
