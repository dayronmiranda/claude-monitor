# Guía Completa - Sistema Unificado de Trabajos (Jobs)

## Introducción

El sistema de **Trabajos (Jobs)** es una unificación conceptual de dos características anteriores:
- **Sesiones**: Historial de conversaciones guardadas (archivos JSONL)
- **Terminales**: Procesos PTY activos ejecutando Claude

Ahora existe un único concepto: **Trabajo**, que representa la ejecución de Claude en un directorio con un ciclo de vida definido.

---

## Modelo de Datos

### Máquina de Estados

```
┌─────────┐
│ CREATED │ ← Usuario crea trabajo, selecciona directorio
└────┬────┘
     │ START
     ↓
┌──────────┐
│ STARTING │ ← Iniciando proceso PTY (transitorio)
└────┬─────┘
     │ READY / FAILED
     ↓
┌───────┐
│ACTIVE │ ← Proceso corriendo, conversación en tiempo real
└──┬─┬──┘
   │ │ PAUSE / STOP / ERROR
   │ ↓
   │ ┌─────────┐
   │ │ PAUSED  │ ← Proceso pausado, puede reanudarse
   │ └─┬──┬────┘
   │  │  │ STOP / RESUME
   │  ↓  ↓
   └─→┌──────────┐
      │ STOPPED  │ ← Trabajo detenido, puede reanudarse (hasta 7 días)
      └──┬──┬───┘
         │  │ RESUME / ARCHIVE / DELETE
         │  ↓
         │  ┌──────────┐
         └─→│ ARCHIVED │ ← Almacenado permanentemente, solo lectura
            └──┬────┬─┘
               │    │ REOPEN / DELETE
               │    ↓
         ERROR DELETED (final)
```

### Estados y Significado

| Estado | Significado | Acciones Disponibles |
|--------|-------------|-------------------|
| **CREATED** | Trabajo creado pero no iniciado | Start, Delete |
| **STARTING** | Iniciando proceso (< 5 seg) | (Automático) |
| **ACTIVE** | En ejecución, conversación activa | Pause, Stop |
| **PAUSED** | Pausado voluntariamente | Resume, Stop |
| **STOPPED** | Detenido, puede reanudarse | Resume, Archive, Delete |
| **ARCHIVED** | Archivado permanentemente | Reopen, Delete |
| **ERROR** | Fallo en ejecución | Retry (hasta 3x), Discard |
| **DELETED** | Eliminado | (Final) |

---

## API REST

### Endpoints

#### Listar Trabajos
```
GET /api/projects/{projectPath}/jobs
GET /api/projects/{projectPath}/jobs?state=active
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "session_id": "uuid",
      "name": "Database Migration",
      "work_dir": "/var/www/project",
      "type": "claude",
      "model": "claude-3.5-sonnet",
      "state": "active",
      "created_at": "2026-01-12T10:00:00Z",
      "started_at": "2026-01-12T10:01:00Z",
      "message_count": 42,
      "user_messages": 21,
      "assistant_messages": 21,
      "pause_count": 1,
      "resume_count": 1,
      "clients": 2,
      "is_archived": false
    }
  ]
}
```

#### Crear Trabajo
```
POST /api/projects/{projectPath}/jobs
Content-Type: application/json

{
  "name": "Mi Proyecto",
  "description": "Descripción opcional",
  "work_dir": "/var/www/project",
  "type": "claude",
  "model": "claude-3.5-sonnet"
}
```

#### Obtener Detalles
```
GET /api/projects/{projectPath}/jobs/{jobId}
```

#### Transiciones de Estado
```
POST /api/projects/{projectPath}/jobs/{jobId}/start
POST /api/projects/{projectPath}/jobs/{jobId}/pause
POST /api/projects/{projectPath}/jobs/{jobId}/resume
POST /api/projects/{projectPath}/jobs/{jobId}/stop
POST /api/projects/{projectPath}/jobs/{jobId}/archive
POST /api/projects/{projectPath}/jobs/{jobId}/delete
```

#### Manejo de Errores
```
POST /api/projects/{projectPath}/jobs/{jobId}/retry
POST /api/projects/{projectPath}/jobs/{jobId}/discard
```

#### Obtener Mensajes
```
GET /api/projects/{projectPath}/jobs/{jobId}/messages

Response:
{
  "id": "uuid",
  "message_count": 42,
  "user_messages": 21,
  "assistant_messages": 21,
  "messages": [
    {
      "type": "user",
      "content": "¿Cómo crear una tabla en PostgreSQL?",
      "timestamp": "2026-01-12T10:05:00Z"
    },
    {
      "type": "assistant",
      "content": "Para crear una tabla en PostgreSQL...",
      "timestamp": "2026-01-12T10:05:30Z"
    }
  ]
}
```

#### Acciones Disponibles
```
GET /api/projects/{projectPath}/jobs/{jobId}/actions

Response:
{
  "id": "uuid",
  "state": "active",
  "actions": ["PAUSE", "STOP", "ERROR"]
}
```

---

## Interfaz de Usuario

### Acceso

**URL Principal:** `http://72.60.69.72:9001/jobs`

### Vistas

#### 1. Página Principal (JobsPage)
- **Tabs de filtrado:**
  - 🟢 Activos - Trabajos en ejecución
  - ⏸️ Pausados - Trabajos pausados
  - ⏹️ Detenidos - Trabajos detenidos (reanudables)
  - 📦 Archivados - Trabajos archivados
  - ❌ Errores - Trabajos con error
  - Todos - Todos los trabajos

- **Dashboard:**
  - Total de trabajos
  - Contadores por estado
  - Indicadores visuales

- **Funcionalidades:**
  - Crear nuevo trabajo
  - Filtrar por estado
  - Ver tarjetas de trabajo
  - Acciones contextuales

#### 2. Detalle de Trabajo (JobDetailPage)
- **Información:**
  - Nombre y descripción
  - Directorio de trabajo
  - Tipo y modelo
  - Timestamps (creación, inicio, pausa, parada)
  - Estado actual

- **Métricas:**
  - Total de mensajes
  - Mensajes del usuario
  - Respuestas de Claude
  - Clientes conectados (si activo)

- **Acciones:**
  - Botones contextuales según estado
  - Ver conversación
  - Control de ciclo de vida

#### 3. Conversación (JobMessagesPage)
- **Visualización:**
  - Mensajes en orden cronológico
  - Avatar para usuario/Claude
  - Timestamps relativos
  - Formato markdown

- **Funcionalidades:**
  - Scroll para historial
  - Búsqueda (próximamente)
  - Exportar conversación (próximamente)

---

## Flujos de Uso

### Flujo 1: Crear y Ejecutar

```
1. Ir a /jobs
2. Haz clic en "Nuevo Trabajo"
3. Rellena el formulario:
   - Nombre: "Database Migration"
   - Directorio: /var/www/my-project
   - Tipo: Claude
   - Modelo: Claude 3.5 Sonnet
4. Haz clic en "Crear Trabajo"
   → Estado: CREATED
5. Haz clic en "Iniciar"
   → Estado: STARTING → ACTIVE
6. Comienza a escribir comandos/preguntas
   → Claude responde en tiempo real
```

### Flujo 2: Pausar y Reanudar

```
1. Trabajo en estado ACTIVE
2. Haz clic en "Pausar"
   → Estado: PAUSED
3. Proceso suspendido, PTY permanece abierto
4. (Esperar o trabajar en otro trabajo)
5. Haz clic en "Reanudar"
   → Estado: ACTIVE
6. Continuación automática de la conversación
   → Same session_id, contexto preservado
```

### Flujo 3: Detener y Guardar

```
1. Trabajo en estado ACTIVE
2. Haz clic en "Detener"
   → Estado: STOPPED
3. PTY cierra, JSONL se finaliza
4. Trabajo puede reanudarse hasta 7 días
5. Después: Haz clic en "Archivar"
   → Estado: ARCHIVED (permanente)
```

### Flujo 4: Manejo de Errores

```
1. Trabajo en estado ACTIVE
2. Error en ejecución
   → Estado: ERROR
3. Se muestra mensaje de error
4. Haz clic en "Reintentar"
   → Intento 1/3
   → Vuelve a STARTING
5. Si falla 3 veces → Auto-descartado
6. O: Haz clic en "Descartar"
   → Estado: DELETED
```

---

## Almacenamiento y Persistencia

### Estructura de Directorios

```
/root/claude-monitor/
├── jobs/                    # Directorio de persistencia
│   ├── {jobId1}.json       # Job en estado STOPPED/ARCHIVED
│   ├── {jobId2}.json       # Job en estado STOPPED/ARCHIVED
│   └── ...
├── monitor                  # Binario backend
└── config.json              # Configuración
```

### Formato de Archivo

Cada trabajo persistido es un JSON:

```json
{
  "id": "uuid",
  "session_id": "uuid",
  "name": "Database Migration",
  "work_dir": "/var/www/project",
  "state": "stopped",
  "created_at": "2026-01-12T10:00:00Z",
  "started_at": "2026-01-12T10:01:00Z",
  "stopped_at": "2026-01-12T10:30:00Z",
  "message_count": 42,
  "user_messages": 21,
  "assistant_messages": 21,
  "pause_count": 1,
  "resume_count": 1,
  "is_archived": false,
  "auto_archived": false
}
```

### Auto-Archiving

- Trabajos en estado STOPPED > 7 días → Auto-archivados
- Se ejecuta automáticamente
- Marca: `auto_archived: true`
- Puede reabrirse manualmente

---

## Características Avanzadas

### 1. Preservación de Contexto

Cuando reanudas un trabajo:
- **Mismo ID**: session_id se preserva
- **Mismo contexto**: Mensajes anteriores se cargan
- **Continuación**: Puedes retomar exactamente donde paraste

### 2. Métricas

Cada trabajo registra:
- Total de mensajes intercambiados
- Desglosen: usuario vs. asistente
- Contador de pausas/reanudaciones
- Clientes WebSocket conectados (si activo)

### 3. Control de Clientes

- Cuenta clientes conectados al PTY
- Número dinámico en UI
- Útil para monitoreo multi-usuario

### 4. Manejo de Errores

- Retry automático (máx. 3 intentos)
- Contador de fallos
- Mensaje de error legible
- Opción manual de descartar

---

## Migración desde Sesiones y Terminales

### Compatibilidad

Durante la migración gradual:
- Endpoints `/api/terminals` siguen funcionando
- Endpoints `/api/sessions` siguen funcionando
- Pero internamente usan Jobs
- Headers `Deprecation: true` en viejos endpoints

### Conversión

| Antes | Ahora |
|-------|-------|
| Terminal Activo | Job ACTIVE |
| Terminal Detenido | Job STOPPED |
| Sesión Guardada | Job STOPPED/ARCHIVED |
| Sesión Activa | Job ACTIVE |

### Timeline

- **Semanas 1-2**: Backend dual
- **Semana 3**: Frontend dual
- **Semanas 4-8**: Deprecación
- **Semana 9+**: Eliminación de viejos endpoints

---

## Troubleshooting

### "Error al crear trabajo"
→ Verifica que el directorio existe y tienes permisos

### "No se puede pausar trabajo"
→ Solo trabaj en estado ACTIVE pueden pausarse

### "No se puede reanudar trabajo antiguo"
→ Trabajos > 7 días detenidos no pueden reanudarse (archívalo primero)

### "Error con código 401"
→ Problema de autenticación con backend

### "Trabajo desaparece al refrescar"
→ Recarga la página para sincronizar estado

---

## Mejoras Futuras

- [ ] Búsqueda en conversaciones
- [ ] Exportar conversación a TXT/PDF
- [ ] Tags/Categorías para trabajos
- [ ] Filtrado avanzado
- [ ] Estadísticas por trabajo
- [ ] Integración con Git
- [ ] Notificaciones de cambio de estado
- [ ] Duplicar trabajo existente

---

## Referencias Técnicas

### Backend
- **Lenguaje**: Go
- **Servicio**: `services/JobService`
- **Handlers**: `handlers/JobsHandler`
- **Persistencia**: JSON files + in-memory maps
- **Concurrencia**: RWMutex para thread-safety

### Frontend
- **Framework**: React + TypeScript
- **Cliente API**: `services/JobsClient`
- **Componentes**: `components/jobs/*`
- **Rutas**:
  - `/jobs` - Lista
  - `/jobs/:jobId` - Detalle
  - `/jobs/:jobId/messages` - Conversación

### API
- **Base URL**: `http://localhost:9003/api`
- **Autenticación**: Basic Auth
- **Formato**: JSON
- **Endpoints**: 14 operaciones CRUD + transiciones

---

**Versión**: 1.0.0
**Última actualización**: 2026-01-12
**Estado**: Producción
