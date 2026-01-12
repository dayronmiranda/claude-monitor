# Testing Checklist - Unificación de Trabajos (Jobs)

## Fase 1: Testing Unitario

### Backend Tests
- [ ] `TestJobStateTransitions` - Todas las transiciones válidas funcionan
- [ ] `TestJobLifecycle` - Flujo completo CREATED → ACTIVE → PAUSED → STOPPED → ARCHIVED
- [ ] `TestJobResume` - Reanudación preserva SessionID y contexto
- [ ] `TestJobAutoArchive` - Auto-archivado después de 7 días
- [ ] `TestInvalidTransitions` - Transiciones inválidas retornan error
- [ ] `TestGetValidTransitions` - Acciones disponibles por estado
- [ ] `TestJobListByState` - Filtrado por estado funciona
- [ ] `TestValidateJobState` - Validación de integridad
- [ ] `TestRepairJob` - Reparación de jobs inconsistentes

### Ejecutar tests:
```bash
cd /root/claude-monitor
go test -v ./services -run "TestJob*"
go test -bench "BenchmarkJobTransition" ./services
```

---

## Fase 2: Testing de Integración

### API Endpoints
- [ ] `POST /api/projects/{path}/jobs` - Crear job
- [ ] `GET /api/projects/{path}/jobs` - Listar jobs
- [ ] `GET /api/projects/{path}/jobs?state=active` - Filtrar por estado
- [ ] `GET /api/projects/{path}/jobs/{id}` - Obtener job específico
- [ ] `DELETE /api/projects/{path}/jobs/{id}` - Eliminar job
- [ ] `POST /api/projects/{path}/jobs/{id}/start` - Iniciar job
- [ ] `POST /api/projects/{path}/jobs/{id}/pause` - Pausar job
- [ ] `POST /api/projects/{path}/jobs/{id}/resume` - Reanudar job
- [ ] `POST /api/projects/{path}/jobs/{id}/stop` - Detener job
- [ ] `POST /api/projects/{path}/jobs/{id}/archive` - Archivar job
- [ ] `POST /api/projects/{path}/jobs/{id}/retry` - Reintentar job en error
- [ ] `POST /api/projects/{path}/jobs/{id}/discard` - Descartar job en error
- [ ] `GET /api/projects/{path}/jobs/{id}/messages` - Obtener mensajes
- [ ] `GET /api/projects/{path}/jobs/{id}/actions` - Obtener acciones disponibles

### Test Script:
```bash
#!/bin/bash
# test_api.sh

BASE_URL="http://localhost:9003"
PROJECT_PATH="test-project"

# Crear job
JOB=$(curl -X POST $BASE_URL/api/projects/$PROJECT_PATH/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Job",
    "work_dir": "/tmp",
    "type": "claude",
    "model": "claude-3.5-sonnet"
  }')

JOB_ID=$(echo $JOB | jq -r '.data.id')
echo "✓ Job creado: $JOB_ID"

# Listar jobs
curl -X GET "$BASE_URL/api/projects/$PROJECT_PATH/jobs" | jq .
echo "✓ Jobs listados"

# Iniciar job
curl -X POST "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID/start" | jq .
echo "✓ Job iniciado"

# Obtener acciones disponibles
curl -X GET "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID/actions" | jq .
echo "✓ Acciones obtenidas"

# Pausar job
curl -X POST "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID/pause" | jq .
echo "✓ Job pausado"

# Reanudar job
curl -X POST "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID/resume" | jq .
echo "✓ Job reanudado"

# Detener job
curl -X POST "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID/stop" | jq .
echo "✓ Job detenido"

# Archivar job
curl -X POST "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID/archive" | jq .
echo "✓ Job archivado"

# Eliminar job
curl -X DELETE "$BASE_URL/api/projects/$PROJECT_PATH/jobs/$JOB_ID" | jq .
echo "✓ Job eliminado"
```

---

## Fase 3: Testing Manual - Flujo Completo

### Escenario 1: Crear y Completar Job Exitoso

```
[ ] 1. Navegar a /jobs
[ ] 2. Hacer clic en "Nuevo Trabajo"
[ ] 3. Ingresar nombre: "Test Job 1"
[ ] 4. Seleccionar directorio: /tmp
[ ] 5. Tipo: Claude
[ ] 6. Modelo: Claude 3.5 Sonnet
[ ] 7. Hacer clic en "Crear Trabajo"
    → Verificar: Job aparece en lista con estado CREATED
[ ] 8. Hacer clic en botón "Iniciar"
    → Verificar: Estado cambia a STARTING/ACTIVE
[ ] 9. Escribir un comando de prueba
    → Verificar: Respuesta aparece en tiempo real
[ ] 10. Hacer clic en "Pausar"
    → Verificar: Estado cambia a PAUSED, botón cambia a "Reanudar"
[ ] 11. Esperar 1 minuto, hacer clic en "Reanudar"
    → Verificar: Estado vuelve a ACTIVE, conversación continúa
[ ] 12. Hacer clic en "Detener"
    → Verificar: Estado cambia a STOPPED, botones cambian
[ ] 13. Hacer clic en "Ver Conversación"
    → Verificar: Aparece historial de mensajes
[ ] 14. Volver a la lista, hacer clic en "Archivar"
    → Verificar: Estado cambia a ARCHIVED
[ ] 15. Hacer clic en el tab "Archivados"
    → Verificar: Job aparece en sección archivados
[ ] 16. Hacer clic en botón "Reabrir"
    → Verificar: Estado vuelve a STOPPED
[ ] 17. Hacer clic en "Eliminar"
    → Verificar: Confirmación, Job desaparece de lista
```

### Escenario 2: Pausar y Reanudar

```
[ ] 1. Crear nuevo job
[ ] 2. Iniciar job (estado → ACTIVE)
[ ] 3. Hacer clic en "Pausar"
    → Verificar: Estado → PAUSED
    → Verificar: pause_count incrementa en 1
    → Verificar: paused_at se establece
[ ] 4. Esperar 5 minutos
[ ] 5. Hacer clic en "Reanudar"
    → Verificar: Estado → ACTIVE
    → Verificar: resume_count incrementa en 1
    → Verificar: Conversación continúa desde donde se pausó
[ ] 6. Pausa nuevamente
    → Verificar: pause_count = 2
[ ] 7. Reanudar nuevamente
    → Verificar: resume_count = 2
```

### Escenario 3: Detencion y Reanudación Multi-Sesión

```
[ ] 1. Crear job "Long Running"
[ ] 2. Iniciar (ACTIVE)
[ ] 3. Enviar algunos comandos
    → Verificar: message_count aumenta
[ ] 4. Hacer clic en "Detener"
    → Verificar: Estado → STOPPED, stopped_at se establece
[ ] 5. Cerrar navegador o volver a home
[ ] 6. Regresar a /jobs
    → Verificar: Job aparece en "Detenidos"
[ ] 7. Hacer clic en "Reanudar"
    → Verificar: Estado → STARTING
    → Verificar: SessionID es el MISMO (no nuevo ID)
[ ] 8. Esperar a ACTIVE
    → Verificar: Mensajes anteriores aparecen en historial
    → Verificar: Puede continuar conversación
[ ] 9. Detener nuevamente
[ ] 10. Después de 7 días (simular en BD)
    → Verificar: Auto-archived
```

### Escenario 4: Manejo de Errores

```
[ ] 1. Crear job con directorio inválido: "/nonexistent/path"
    → Verificar: Muestra error "Directorio no válido"
[ ] 2. Crear job válido
[ ] 3. Iniciar
[ ] 4. Simular fallo (killing process)
    → Verificar: Estado cambia a ERROR
    → Verificar: job.error contiene código y mensaje
[ ] 5. Hacer clic en "Reintentar"
    → Verificar: retry_count incrementa
    → Verificar: Estado vuelve a STARTING
[ ] 6. Si falla 3 veces
    → Verificar: Auto-descarta
[ ] 7. Manualmente hacer clic en "Descartar" en estado ERROR
    → Verificar: Estado → DELETED
```

### Escenario 5: Filtrado y Búsqueda

```
[ ] 1. Crear 10 jobs en diferentes estados:
    - 3 ACTIVE
    - 2 PAUSED
    - 3 STOPPED
    - 1 ARCHIVED
    - 1 ERROR
[ ] 2. Hacer clic en tab "Activos"
    → Verificar: Muestra solo 3 jobs ACTIVE
[ ] 3. Hacer clic en tab "Pausados"
    → Verificar: Muestra solo 2 jobs PAUSED
[ ] 4. Hacer clic en tab "Detenidos"
    → Verificar: Muestra solo 3 jobs STOPPED
[ ] 5. Hacer clic en tab "Archivados"
    → Verificar: Muestra solo 1 job ARCHIVED
[ ] 6. Hacer clic en tab "Errores"
    → Verificar: Muestra solo 1 job ERROR
[ ] 7. Hacer clic en tab "Todos"
    → Verificar: Muestra todos los 10 jobs
```

### Escenario 6: Compatibilidad Hacia Atrás

```
[ ] 1. Navegar a /terminals (endpoint viejo)
    → Verificar: Página funciona (con banner de deprecación)
    → Verificar: Muestra jobs en formato Terminal
[ ] 2. Navegar a /sessions (endpoint viejo)
    → Verificar: Página funciona (con banner de deprecación)
    → Verificar: Muestra jobs en formato Session
[ ] 3. Crear job vía API vieja (/api/terminals)
    → Verificar: Se crea con éxito
    → Verificar: Aparece en /jobs también
[ ] 4. Modificar job vía /jobs
    → Verificar: Cambios visibles en /terminals también
[ ] 5. Verificar que /api/jobs y /api/terminals retornan consistentemente
```

---

## Fase 4: Performance Testing

### Benchmarks
```bash
# Crear 1000 jobs y medir tiempo
go test -bench "BenchmarkJobTransition" -benchtime=10s ./services

# Medir consumo de memoria
go test -bench "BenchmarkJobTransition" -memprofile=mem.prof ./services
go tool pprof mem.prof

# Medir tiempo de listado
time curl http://localhost:9003/api/projects/test/jobs | jq length
```

### Criterios de Aceptación:
- [ ] Crear job: < 100ms
- [ ] Listar 1000 jobs: < 500ms
- [ ] Transición de estado: < 50ms
- [ ] Memoria por job: < 1MB
- [ ] Sin fugas de memoria después de 1000 ciclos

---

## Fase 5: Testing de Seguridad

- [ ] No se puede acceder a job de otro usuario
- [ ] No se puede cambiar estado de job sin autorización
- [ ] Path traversal: `/api/projects/../../admin` rechazado
- [ ] SQL injection: Parámetros sanitizados
- [ ] XSS: Contenido escapado en UI
- [ ] CSRF: Tokens validados
- [ ] No hay exposición de datos sensibles en logs
- [ ] No hay exposición de paths del sistema

---

## Fase 6: Testing de Base de Datos

- [ ] Persistencia: Job guardado a disco persiste después de restart
- [ ] Sincronización: In-memory y on-disk están sincronizados
- [ ] Recuperación: LoadJobsFromDisk restaura todos los jobs
- [ ] Limpieza: CleanupDeletedJobs funciona correctamente
- [ ] Validación: ValidateJobState detecta inconsistencias
- [ ] Reparación: RepairJob corrige automáticamente

---

## Fase 7: Testing de UX

- [ ] Botones contextuales apropiados por estado
- [ ] Mensajes de error claros y útiles
- [ ] Confirmaciones para acciones destructivas
- [ ] Indicadores de carga (spinner)
- [ ] Estado en vivo actualiza sin recargar
- [ ] Responsive en móvil
- [ ] Accesibilidad (ARIA, keyboard navigation)
- [ ] Indicadores visuales claros (colores, iconos)

---

## Resumen de Testing

| Tipo | Tests | Passed | Failed |
|------|-------|--------|--------|
| Unitario | 9 | ☐ | ☐ |
| Integración | 14 | ☐ | ☐ |
| Manual | 100+ | ☐ | ☐ |
| Performance | 4 | ☐ | ☐ |
| Seguridad | 8 | ☐ | ☐ |
| **Total** | **135+** | **☐** | **☐** |

---

## Criterios de Aprobación

✅ Para pasar Phase 5 (Testing):
- [ ] 100% de tests unitarios pasen
- [ ] 100% de endpoints de integración funcionen
- [ ] 100% de escenarios manuales completados
- [ ] Performance dentro de criterios aceptables
- [ ] Cero security issues
- [ ] Compatibilidad hacia atrás verificada
- [ ] Documentación actualizada
- [ ] Logs limpios sin warnings

---

## Notas para Testers

1. **Backup**: Siempre hacer backup de datos antes de testing
2. **Limpieza**: Remover datos de test después de cada sesión
3. **Reproducibilidad**: Documentar pasos exactos para reproducir issues
4. **Browserstack**: Probar en Chrome, Firefox, Safari, Edge
5. **Regressions**: Verificar que features viejas siguen funcionando
6. **Logs**: Revisar logs del backend para warnings/errors
7. **Rollback**: Verificar script de rollback funciona

---

## Timeline Estimado

| Fase | Duración | Responsable |
|------|----------|-------------|
| Unitario | 1 día | Backend Dev |
| Integración | 1 día | Backend Dev |
| Manual | 2 días | QA |
| Performance | 0.5 día | DevOps |
| Seguridad | 1 día | Security |
| **Total** | **5.5 días** | - |

