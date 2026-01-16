# Claude Monitor - Documentación

Documentación completa del sistema Claude Monitor.

## Índice

### Referencia de API

| Documento | Descripción |
|-----------|-------------|
| [openapi.yaml](./openapi.yaml) | Especificación OpenAPI 3.0 completa |
| [API.md](./API.md) | Guía de uso de la API REST |

### Arquitectura y Diseño

| Documento | Descripción |
|-----------|-------------|
| [CLAUDE_STATE.md](./CLAUDE_STATE.md) | Sistema de detección de estados de Claude |
| [STATE_MACHINE.md](./STATE_MACHINE.md) | Máquina de estados de Jobs |
| [IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md) | Resumen de implementación |

### Guías de Uso

| Documento | Descripción |
|-----------|-------------|
| [JOBS_GUIDE.md](./JOBS_GUIDE.md) | Guía del sistema de Jobs |
| [TESTING_CHECKLIST.md](./TESTING_CHECKLIST.md) | Checklist de testing |

### Desarrollo y Migración

| Documento | Descripción |
|-----------|-------------|
| [MIGRATION_STRATEGY.md](./MIGRATION_STRATEGY.md) | Estrategia de migración |
| [IMPROVEMENT_PLAN.md](./IMPROVEMENT_PLAN.md) | Plan de mejoras arquitectónicas |
| [PROJECT_COMPLETE.md](./PROJECT_COMPLETE.md) | Estado del proyecto |

---

## Estructura de Documentos

```
docs/
├── README.md                 # Este índice
├── openapi.yaml              # Especificación OpenAPI 3.0 (Swagger)
├── API.md                    # Guía de uso de la API
├── CLAUDE_STATE.md           # Detección de estados y eventos WebSocket
├── STATE_MACHINE.md          # Máquina de estados de Jobs
├── JOBS_GUIDE.md             # Sistema de tareas programadas
├── IMPLEMENTATION_SUMMARY.md # Resumen técnico de implementación
├── TESTING_CHECKLIST.md      # Checklist para testing
├── MIGRATION_STRATEGY.md     # Estrategia de migración
├── IMPROVEMENT_PLAN.md       # Plan de mejoras
└── PROJECT_COMPLETE.md       # Estado y completitud del proyecto
```

---

## Documentos Clave por Tema

### Para Desarrolladores de Frontend

1. **[openapi.yaml](./openapi.yaml)** - Especificación completa para generar clientes
2. **[CLAUDE_STATE.md](./CLAUDE_STATE.md)** - Eventos WebSocket y estados de Claude
3. **[API.md](./API.md)** - Ejemplos de uso de endpoints

### Para Desarrolladores de Backend

1. **[IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md)** - Arquitectura interna
2. **[STATE_MACHINE.md](./STATE_MACHINE.md)** - Máquina de estados de Jobs
3. **[CLAUDE_STATE.md](./CLAUDE_STATE.md)** - Detección de patrones

### Para Operaciones/DevOps

1. **[API.md](./API.md)** - Endpoints de health y métricas
2. **[TESTING_CHECKLIST.md](./TESTING_CHECKLIST.md)** - Verificación del sistema

---

## Uso de la Especificación OpenAPI

### Ver documentación interactiva

```bash
# Con Swagger UI
docker run -p 8081:8080 -e SWAGGER_JSON=/api/openapi.yaml \
  -v $(pwd)/docs:/api swaggerapi/swagger-ui

# Con Redoc
npx @redocly/cli preview-docs docs/openapi.yaml
```

### Generar clientes

```bash
# TypeScript
npx openapi-typescript docs/openapi.yaml -o src/api-types.ts

# Go
go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen \
  -generate types,client -o pkg/client/client.go docs/openapi.yaml

# Python
openapi-generator generate -i docs/openapi.yaml -g python -o ./python-client
```

### Validar especificación

```bash
npx @redocly/cli lint docs/openapi.yaml
```

---

## Convenciones

- **Formato**: Markdown (GitHub Flavored)
- **Diagramas**: ASCII art para máxima compatibilidad
- **Ejemplos de código**: Con syntax highlighting
- **Idioma**: Español (comentarios de código en inglés)
