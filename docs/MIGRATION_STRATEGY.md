# Estrategia de Migración - Sesiones y Terminales → Trabajos (Jobs)

## Visión General

Esta estrategia permite la migración gradual de dos conceptos separados (Sesiones y Terminales) a un modelo unificado (Trabajos) **sin romper compatibilidad** durante el período de transición.

## Fases de Migración

### Fase 1: Backend Dual (Semanas 1-2)

**Objetivo**: Mantener ambas APIs funcionando en paralelo

#### Endpoints Existentes (Mantener):
```
GET  /api/terminals                    - Lista terminales activos
POST /api/terminals                    - Crear terminal
GET  /api/terminals/{id}               - Obtener terminal
DELETE /api/terminals/{id}             - Eliminar terminal
POST /api/terminals/{id}/kill          - Matar terminal
POST /api/terminals/{id}/resume        - Reanudar terminal

GET  /api/sessions                     - Lista sesiones
GET  /api/projects/{path}/sessions     - Sesiones por proyecto
GET  /api/sessions/{id}                - Obtener sesión
POST /api/sessions/{id}/delete         - Eliminar sesión
```

#### Endpoints Nuevos (Introducir):
```
GET  /api/projects/{path}/jobs              - Lista jobs
POST /api/projects/{path}/jobs              - Crear job
GET  /api/projects/{path}/jobs/{id}         - Obtener job
DELETE /api/projects/{path}/jobs/{id}       - Eliminar job
POST /api/projects/{path}/jobs/{id}/start   - Iniciar job
POST /api/projects/{path}/jobs/{id}/pause   - Pausar job
POST /api/projects/{path}/jobs/{id}/resume  - Reanudar job
POST /api/projects/{path}/jobs/{id}/stop    - Detener job
POST /api/projects/{path}/jobs/{id}/archive - Archivar job
```

#### Sincronización Bidireccional:

```go
// Cuando se crea un Terminal vía API vieja
func (h *TerminalsHandler) CreateTerminal() {
    terminal := h.terminalSvc.Create(config)

    // También crear un Job equivalente
    jobConfig := mapTerminalToJobConfig(terminal)
    h.jobSvc.Create(jobConfig)
}

// Cuando se crea un Job vía API nueva
func (h *JobsHandler) CreateJob() {
    job := h.jobSvc.Create(config)

    // OPCION 1: No sincronizar (recomendar uso de nuevos endpoints)
    // OPCION 2: Crear Terminal equivalente para compatibilidad
    // terminalConfig := mapJobToTerminalConfig(job)
    // h.terminalSvc.Create(terminalConfig)
}

// Acceso a JobService desde endpoints viejos
func (h *TerminalsHandler) KillTerminal(id string) {
    // Implementar usando JobService
    job, _ := h.jobSvc.Get(id)
    if job.State == "active" {
        h.jobSvc.Stop(id)
    }
}
```

#### Data Migration Script:

```bash
#!/bin/bash
# run_migration.sh

echo "Iniciando migración de Terminales y Sesiones a Jobs..."

# Backup
cp -r /root/claude-monitor/data /root/claude-monitor/data.backup
echo "✓ Backup creado"

# Ejecutar migración
curl -X POST http://localhost:9003/api/admin/migrate-to-jobs \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Verificar
STATS=$(curl http://localhost:9003/api/admin/migration-status)
echo "✓ Migración completada:"
echo "  Terminales migracos: $(echo $STATS | jq .terminals_migrated)"
echo "  Sesiones migradas: $(echo $STATS | jq .sessions_migrated)"
echo "  Total Jobs creados: $(echo $STATS | jq .total_jobs_created)"
```

### Fase 2: Frontend Dual (Semana 3)

**Objetivo**: Ofrecer ambas vistas, con banner recomendando migración

#### Estructura de Rutas:

```typescript
// Rutas viejas (deprecadas)
<Route path="/terminals" element={<TerminalsPage />} />
<Route path="/sessions" element={<SessionsPage />} />

// Rutas nuevas (recomendadas)
<Route path="/jobs" element={<JobsPage />} />
<Route path="/jobs/:id" element={<JobDetailPage />} />

// Redirecciones suaves (después de 2 semanas)
// /terminals → /jobs?state=active
// /sessions → /jobs?state=stopped,archived
```

#### Banner de Migración:

```tsx
export function MigrationBanner() {
  return (
    <div className="bg-blue-50 border-l-4 border-blue-500 p-4 mb-4">
      <p className="text-sm font-medium text-blue-900">
        📢 Nueva vista de trabajos disponible
      </p>
      <p className="text-xs text-blue-700 mt-1">
        Hemos unificado Terminales y Sesiones en una sola interfaz.
        <a href="/jobs" className="underline ml-1">Ver Trabajos</a>
      </p>
    </div>
  )
}
```

### Fase 3: Deprecación Suave (Semanas 4-8)

**Objetivo**: Avisar usuarios sin romper nada

#### Acciones:
1. Agregar `Deprecation` headers en endpoints viejos:
   ```
   Deprecation: true
   Sunset: Wed, 21 Feb 2024 23:59:59 GMT
   Link: </api/projects/{path}/jobs>; rel="successor-version"
   ```

2. Logs de deprecación:
   ```
   [DEPRECATED] GET /api/terminals - use /api/projects/{path}/jobs instead
   ```

3. UI warning:
   ```tsx
   <Alert variant="warning">
     Esta vista está deprecada. Usa la nueva vista de Trabajos.
   </Alert>
   ```

### Fase 4: Eliminación (después de Semana 8)

**Objetivo**: Remover código legacy

#### Acciones:
1. Remover endpoints viejos
2. Remover componentes viejos (TerminalsPage, SessionsPage)
3. Cleanup de servicios (TerminalService, SessionService referencias cruzadas)
4. Remover handlers viejos

#### Timeline Final:
```
Semana 1-2:   Backend dual (Jobs + Terminales/Sesiones)
Semana 3:     Frontend dual (Jobs + Terminales/Sesiones)
Semana 4-8:   Deprecación con warnings
Semana 9+:    Eliminación completa (breaking changes)
```

## Mapeo de Conceptos

### Terminal → Job

```
Terminal.ID                → Job.ID
Terminal.SessionID         → Job.SessionID
Terminal.Name              → Job.Name
Terminal.WorkDir           → Job.WorkDir
Terminal.Type              → Job.Type
Terminal.Status            → Job.State (mapping)
Terminal.Model             → Job.Model
Terminal.CreatedAt         → Job.CreatedAt
Terminal.StartedAt         → Job.StartedAt
Terminal.Cmd               → Job.Cmd
Terminal.Pty               → Job.Pty
Terminal.Active            → Job.State (active/stopped)
Terminal.Clients           → Job.Clients
Terminal.LastAccessAt      → Job.StoppedAt
```

### Session → Job

```
Session.ID                 → Job.SessionID (mismo que ID)
Session.Name               → Job.Name
Session.ProjectPath        → Job.ProjectPath
Session.RealPath           → Job.RealPath
Session.CreatedAt          → Job.CreatedAt
Session.ModifiedAt         → Job.StoppedAt
Session.MessageCount       → Job.MessageCount
Session.UserMessages       → Job.UserMessages
Session.AssistantMessages  → Job.AssistantMessages
Session.IsArchived         → Job.IsArchived
Session.Type               → Job.Type = "claude"
```

### Status Mapping

```
Terminal Status → Job State
"running"       → "active"
"stopped"       → "stopped"
"error"         → "error"
"initializing"  → "starting"

Session Status → Job State
"active"        → "stopped"
"inactive"      → "stopped"
"archived"      → "archived"
"error"         → "error"
```

## Compatibility Layer

### Reading (Leer datos viejos desde API nueva):

```go
// GET /api/projects/{path}/jobs/{id} puede retornar formato "legacy"
// con ?format=terminal o ?format=session
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
    format := r.URL.Query().Get("format") // "job", "terminal", "session"

    job, _ := h.jobService.Get(jobID)

    switch format {
    case "terminal":
        // Convertir Job → Terminal format
        terminalData := h.jobService.GetJobAsTerminal(jobID)
        json.NewEncoder(w).Encode(terminalData)
    case "session":
        // Convertir Job → Session format
        sessionData := h.jobService.GetJobAsSession(jobID)
        json.NewEncoder(w).Encode(sessionData)
    default:
        // Retornar Job nativo
        json.NewEncoder(w).Encode(job)
    }
}
```

### Writing (Crear datos a través de API vieja):

```go
// POST /api/terminals crea un Job internamente
func (h *TerminalsHandler) CreateTerminal() {
    terminalConfig := parseRequest()

    // Convertir Terminal config → Job config
    jobConfig := services.JobConfig{
        ID:          terminalConfig.ID,
        Name:        terminalConfig.Name,
        WorkDir:     terminalConfig.WorkDir,
        Type:        terminalConfig.Type,
        ProjectPath: terminalConfig.ProjectPath,
        Model:       terminalConfig.Model,
    }

    // Crear Job (no Terminal)
    job, _ := h.jobService.Create(jobConfig)

    // Retornar en formato Terminal para compatibilidad
    response := h.jobService.GetJobAsTerminal(job.ID)
    json.NewEncoder(w).Encode(response)
}
```

## Validation Checklist

### Backend Compatibility:
- [ ] Endpoint /api/terminals sigue funcionando
- [ ] Endpoint /api/sessions sigue funcionando
- [ ] Nuevo endpoint /api/projects/{path}/jobs funciona
- [ ] Crear Terminal vía API vieja crea Job internamente
- [ ] Datos sincronizados entre ambas vistas
- [ ] Cambios en Job se reflejan en Terminal/Session
- [ ] No hay pérdida de datos

### Frontend Compatibility:
- [ ] Página /terminals sigue visible (con banner)
- [ ] Página /sessions sigue visible (con banner)
- [ ] Nueva página /jobs funciona
- [ ] Datos consistentes en ambas vistas
- [ ] Transiciones de estado correctas
- [ ] Performance no degradada

### Data Integrity:
- [ ] Backup automático antes de migración
- [ ] Validación de datos migrados
- [ ] Rollback script disponible
- [ ] Logs de migración detallados
- [ ] Zero data loss

## Rollback Strategy

Si hay problemas durante la migración:

```bash
#!/bin/bash
# rollback.sh

echo "Iniciando rollback..."

# 1. Stop services
systemctl stop claude-monitor

# 2. Restore backup
rm -rf /root/claude-monitor/data
cp -r /root/claude-monitor/data.backup /root/claude-monitor/data

# 3. Clear Job data
rm -rf /root/claude-monitor/jobs/*

# 4. Restart
systemctl start claude-monitor

echo "✓ Rollback completado"
```

## Comunicación al Usuario

### Email/Notification:

```
Asunto: Nueva Vista Unificada de Trabajos

Hola Usuario,

Hemos lanzado una nueva forma de ver tus sesiones de Claude:
en lugar de Terminales y Sesiones separados, ahora están
unificados como "Trabajos".

Visita: https://app/jobs

Durante las próximas semanas, ambas vistas estarán disponibles.
El 21 de Febrero, la vista antigua será descontinuada.

Preguntas? Contactanos...
```

### In-App Messaging:

```
Banner: "📢 Nueva vista de Trabajos disponible - Unifica tus sesiones"
Tooltip: "Los Trabajos combinan Terminales y Sesiones en una interfaz"
Help: "Migración gradual - ambas vistas disponibles hasta Feb 21"
```

---

## Resumen Temporal

| Período    | Acción | Estado |
|-----------|--------|--------|
| Semana 1-2| Backend dual | Ambos endpoints funcionan |
| Semana 3  | Frontend dual | Ambas UIs disponibles |
| Semana 4-8| Deprecación  | Warnings y avisos |
| Semana 9+ | Eliminación  | Solo API nueva |

**Resultado Final**: Usuarios migrados sin interrupción, datos preservados, UX mejorada.
