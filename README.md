# Claude Monitor

Sistema web para monitorear, gestionar y visualizar el historial completo de conversaciones con Claude desde tu máquina local o remota.

## 🎯 Características

### Backend (Go)
- API REST para gestionar proyectos, sesiones y terminales
- Lectura de archivos JSONL generados por Claude Code
- Extracción completa de contenido (pensamiento, comandos, resultados)
- Análisis y estadísticas de sesiones
- Autenticación Basic Auth + API Token
- Soporte WebSocket para terminales PTY

### Frontend (React + TypeScript)
- Interfaz moderna con Tailwind CSS
- Gestión de múltiples drivers (hosts)
- Listado de proyectos y sesiones
- **Página dedicada para ver historial completo de chat**
- Edición de nombres de sesiones
- Eliminación y limpieza de sesiones
- Control de terminales PTY
- Analytics global y por proyecto

## 📋 Contenido del Historial

Cada sesión muestra:
- ✅ Mensajes de usuario
- ✅ Respuestas del asistente
- ✅ Pensamientos internos (💭)
- ✅ Archivos leídos (🔍 Read)
- ✅ Cambios realizados (✏️ Edit)
- ✅ Comandos ejecutados (🔧 Bash)
- ✅ Resultados de herramientas (✅/❌)
- ✅ Listas de TODOs (📋)

## 🚀 Inicio Rápido

### Requisitos
- Go 1.24+ (backend)
- Node.js 18+ (frontend)
- Git

### Instalación

#### Backend
```bash
cd claude-monitor
go build -o claude-monitor .
./claude-monitor
```

El servidor iniciará en `http://localhost:9090`

#### Frontend
```bash
cd claude-monitor-client
npm install
npm run dev
```

El cliente estará disponible en `http://localhost:9001`

## 📝 Configuración

### Variables de Entorno (Backend)

```bash
CLAUDE_MONITOR_PORT=9090              # Puerto del servidor
CLAUDE_MONITOR_HOST=0.0.0.0           # Host del servidor
CLAUDE_MONITOR_USERNAME=admin         # Usuario básico
CLAUDE_MONITOR_PASSWORD=admin         # Contraseña básica
CLAUDE_MONITOR_ALLOWED_PATHS=/var/www # Paths permitidos
```

### Acceso al Frontend

1. Abre http://localhost:9001
2. Ve a "Drivers" (barra lateral)
3. Haz clic en "Add Driver"
4. Configura:
   - **Name**: Local Claude Monitor
   - **URL**: http://localhost:9090
   - **Username**: admin
   - **Password**: admin
5. Haz clic en "Connect"

## 📁 Estructura del Proyecto

```
claude-monitor/
├── main.go                 # Punto de entrada
├── router.go              # Enrutamiento HTTP
├── middleware.go          # CORS, Auth, Logging
├── config.go              # Configuración
├── handlers/              # HTTP Handlers
│   ├── projects.go
│   ├── sessions.go
│   ├── terminals.go
│   └── analytics.go
└── services/              # Lógica de negocio
    ├── claude.go          # Gestión de sesiones
    ├── terminal.go        # PTY
    └── analytics.go       # Estadísticas

claude-monitor-client/
├── src/
│   ├── components/
│   │   ├── sessions/
│   │   │   ├── SessionsPage.tsx
│   │   │   └── SessionMessagesPage.tsx
│   │   ├── hosts/
│   │   ├── projects/
│   │   └── terminals/
│   ├── services/
│   │   └── api.ts         # Cliente API
│   ├── stores/
│   │   └── useStore.ts    # Estado global
│   └── types/
│       └── index.ts       # TypeScript interfaces
```

## 🔌 API Endpoints

### Proyectos
- `GET /api/projects` - Listar proyectos
- `GET /api/projects/{path}` - Obtener proyecto
- `DELETE /api/projects/{path}` - Eliminar proyecto

### Sesiones
- `GET /api/projects/{path}/sessions` - Listar sesiones
- `GET /api/projects/{path}/sessions/{id}` - Obtener sesión
- `GET /api/projects/{path}/sessions/{id}/messages` - **Obtener historial de mensajes**
- `DELETE /api/projects/{path}/sessions/{id}` - Eliminar sesión
- `PUT /api/projects/{path}/sessions/{id}/rename` - Renombrar sesión

### Analytics
- `GET /api/analytics/global` - Analytics globales
- `GET /api/analytics/projects/{path}` - Analytics por proyecto

## 📊 Ejemplo de Respuesta (Historial)

```json
{
  "success": true,
  "data": [
    {
      "type": "user",
      "content": "¿Puedes ayudarme con React?",
      "timestamp": "2026-01-11T10:00:00Z",
      "todos": []
    },
    {
      "type": "assistant",
      "content": "Claro, con gusto te ayudo...",
      "timestamp": "2026-01-11T10:00:05Z",
      "todos": ["Explicar hooks", "Mostrar ejemplo"]
    },
    {
      "type": "assistant",
      "content": "🔧 Read:\nReading: /path/to/file.tsx",
      "timestamp": "2026-01-11T10:00:10Z"
    }
  ],
  "meta": {
    "total": 3
  }
}
```

## 🔐 Seguridad

- Basic Authentication (configurable)
- API Token support
- CORS configurado
- Path traversal prevention
- Validación de entrada

## 📈 Commits Principales

```
✓ feat: Agregar visualización de historial de mensajes en sesiones
✓ refactor: Cambiar modal de historial a página completa
✓ fix: Extraer información completa de tool_use blocks
✓ fix: Filtrar sesiones vacías y con solo caveats/metadata
```

## 🛠️ Desarrollo

### Backend
```bash
cd claude-monitor
go build -o claude-monitor .
./claude-monitor
```

### Frontend
```bash
cd claude-monitor-client
npm run dev    # Desarrollo
npm run build  # Producción
```

## 📝 Licencia

MIT

## 👤 Autor

[dayronmiranda](https://github.com/dayronmiranda)

---

**Repositorios:**
- Backend: https://github.com/dayronmiranda/claude-monitor
- Frontend: https://github.com/dayronmiranda/claude-monitor-client
