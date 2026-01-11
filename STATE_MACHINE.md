# Claude Monitor - State Machine Diagram

## Overview

Este documento describe la máquina de estados del backend Claude Monitor Driver.

---

## 1. Terminal State Machine

```
                              ┌──────────────────────────────────────┐
                              │                                      │
    ┌─────────┐               │            ┌─────────────┐           │
    │  INIT   │───────────────┼──────────▶ │   RUNNING   │ ◀─────────┘
    └─────────┘               │            └─────────────┘      POST /resume
         │                    │                   │
         │ POST /terminals    │                   │
         │ (create)           │                   │
         ▼                    │                   │
    ┌─────────────┐           │                   │ cmd.Wait()
    │  Validate   │           │                   │ cleanup()
    │  WorkDir    │           │                   │ POST /kill
    └─────────────┘           │                   │
         │                    │                   ▼
         │ OK                 │            ┌─────────────┐
         ▼                    │            │   STOPPED   │─────────▶ DELETED
    ┌─────────────┐           │            └─────────────┘    DELETE /terminals/{id}
    │  Start PTY  │           │                   │
    │  Process    │           │                   │ CanResume=true
    └─────────────┘           │                   │ (type=claude)
         │                    │                   │
         │                    └───────────────────┘
         │
         ▼
    ┌─────────────┐
    │  Register   │
    │  & Persist  │
    └─────────────┘
```

### Terminal States

| State | Description | Active | CanResume |
|-------|-------------|--------|-----------|
| RUNNING | Terminal PTY activa | true | false |
| STOPPED | Terminal terminada | false | true (if type=claude) |
| DELETED | Registro eliminado | N/A | N/A |

### Terminal Transitions

| From | To | Event | Endpoint |
|------|-----|-------|----------|
| INIT | RUNNING | Create | `POST /api/terminals` |
| RUNNING | STOPPED | Kill | `POST /api/terminals/{id}/kill` |
| RUNNING | STOPPED | Process Exit | (automatic) |
| STOPPED | RUNNING | Resume | `POST /api/terminals/{id}/resume` |
| STOPPED | DELETED | Delete | `DELETE /api/terminals/{id}` |

### Terminal WebSocket Flow

```
    CLIENT                         SERVER
       │                              │
       │  WS /terminals/{id}/ws       │
       │─────────────────────────────▶│
       │                              │ AddClient(conn)
       │                              │
       │◀─────────────────────────────│
       │     { type: "output", data } │
       │                              │
       │  { type: "input", data }     │
       │─────────────────────────────▶│ Write to PTY
       │                              │
       │  { type: "resize", cols, rows}│
       │─────────────────────────────▶│ Resize PTY
       │                              │
       │◀─────────────────────────────│
       │     { type: "closed" }       │ (on cleanup)
       │                              │
```

---

## 2. Session State Machine

```
    ┌────────────────────────────────────────────────────────┐
    │                   FILESYSTEM                            │
    │     ~/.claude/projects/{projectPath}/{sessionID}.jsonl │
    └────────────────────────────────────────────────────────┘
                              │
                              │ Claude CLI writes
                              ▼
                       ┌─────────────┐
                       │   EXISTS    │
                       └─────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  ACTIVE   │   │   EMPTY   │   │ IMPORTED  │
       │ (msgs>0)  │   │ (msgs=0)  │   │           │
       └───────────┘   └───────────┘   └───────────┘
              │               │               │
              │               │               │ POST /import
              │               │               ▼
              │               │        ┌─────────────┐
              │               │        │  TERMINAL   │
              │               │        │   SAVED     │
              │               │        └─────────────┘
              │               │               │
              │               │               │ Can Resume
              │               │               ▼
              │               │        ┌─────────────┐
              │               │        │   RUNNING   │
              │               │        │  TERMINAL   │
              │               │        └─────────────┘
              │               │
              ▼               ▼
       ┌──────────────────────────┐
       │        DELETED           │
       │  DELETE /sessions/{id}   │
       │  POST /sessions/clean    │
       │  POST /sessions/delete   │
       └──────────────────────────┘
```

### Session Actions

| Action | Endpoint | Description |
|--------|----------|-------------|
| List | `GET /api/projects/{path}/sessions` | Listar sesiones |
| Get | `GET /api/projects/{path}/sessions/{id}` | Obtener detalle |
| Delete | `DELETE /api/projects/{path}/sessions/{id}` | Eliminar una |
| Delete Multiple | `POST /api/projects/{path}/sessions/delete` | Eliminar varias |
| Clean Empty | `POST /api/projects/{path}/sessions/clean` | Eliminar vacías |
| Import | `POST /api/projects/{path}/sessions/import` | Importar a terminales |

---

## 3. Project State Machine

```
    ┌────────────────────────────────────────┐
    │           FILESYSTEM                    │
    │   ~/.claude/projects/{projectPath}/    │
    └────────────────────────────────────────┘
                        │
                        │ Directory exists
                        ▼
                 ┌─────────────┐
                 │ DISCOVERED  │
                 └─────────────┘
                        │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
 ┌───────────┐   ┌───────────┐   ┌───────────┐
 │  ACTIVE   │   │   EMPTY   │   │  DELETED  │
 │(sessions>0)│  │(sessions=0)│  │           │
 └───────────┘   └───────────┘   └───────────┘
        │               │               ▲
        │               │               │
        └───────────────┴───────────────┘
                        │
              DELETE /projects/{path}
```

### Project Actions

| Action | Endpoint | Description |
|--------|----------|-------------|
| List | `GET /api/projects` | Listar proyectos |
| Get | `GET /api/projects/{path}` | Obtener detalle + stats |
| Delete | `DELETE /api/projects/{path}` | Eliminar proyecto completo |
| Activity | `GET /api/projects/{path}/activity` | Actividad diaria |

---

## 4. Analytics Cache State Machine

```
                    ┌─────────────┐
                    │    INIT     │
                    └─────────────┘
                           │
                           │ Server Start
                           ▼
                    ┌─────────────┐
       ┌───────────▶│ INVALIDATED │◀──────────────┐
       │            └─────────────┘               │
       │                   │                      │
       │                   │ GET request          │ POST /invalidate
       │                   │ (cache miss)         │ DELETE session
       │                   ▼                      │ DELETE project
       │            ┌─────────────┐               │
       │            │  COMPUTING  │               │
       │            └─────────────┘               │
       │                   │                      │
       │                   │ Done                 │
       │                   ▼                      │
       │            ┌─────────────┐               │
       │            │   CACHED    │───────────────┘
       │            └─────────────┘
       │                   │
       │                   │ TTL expired
       │                   │ (5 min default)
       │                   ▼
       │            ┌─────────────┐
       └────────────│   EXPIRED   │
                    └─────────────┘
```

### Analytics Cache Keys

```
Global Cache:
  key: "global"
  ttl: globalTTL
  data: GlobalAnalytics

Project Cache:
  key: projectPath
  ttl: projectTTL[projectPath]
  data: ProjectAnalytics
```

### Analytics Actions

| Action | Endpoint | Description |
|--------|----------|-------------|
| Get Global | `GET /api/analytics/global` | Stats globales |
| Get Project | `GET /api/analytics/projects/{path}` | Stats de proyecto |
| Invalidate | `POST /api/analytics/invalidate` | Limpiar cache |
| Cache Status | `GET /api/analytics/cache` | Estado del cache |

---

## 5. Complete System Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                           CLIENT (React)                             │
└─────────────────────────────────────────────────────────────────────┘
         │                    │                    │
         │ HTTP REST          │ WebSocket          │ HTTP REST
         ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         MIDDLEWARE                                   │
│                   (CORS + Auth + JSON)                              │
└─────────────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   HANDLERS    │  │   HANDLERS    │  │   HANDLERS    │
│   Projects    │  │   Terminals   │  │   Analytics   │
│   Sessions    │  │   (+ WS)      │  │               │
└───────────────┘  └───────────────┘  └───────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   SERVICES    │  │   SERVICES    │  │   SERVICES    │
│ ClaudeService │  │TerminalService│  │AnalyticsService│
└───────────────┘  └───────────────┘  └───────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│  FILESYSTEM   │  │  PTY + JSON   │  │  IN-MEMORY    │
│~/.claude/     │  │terminals.json │  │   CACHE       │
│projects/      │  │               │  │               │
└───────────────┘  └───────────────┘  └───────────────┘
```

---

## 6. Event Flow Summary

### Creating a Terminal from Session Resume

```
1. User clicks "Resume" on Session
   └─▶ Navigate to /terminals?workdir={realPath}&session={id}&resume=true

2. TerminalsPage shows form with pre-filled data
   └─▶ User clicks "Create Terminal"

3. POST /api/terminals
   {
     work_dir: "/var/www/www.jollytienda.com",
     session_id: "uuid",
     resume: true,
     type: "claude"
   }

4. TerminalService.Create()
   ├─▶ Validate work_dir exists
   ├─▶ Build claude args: ["--resume", "uuid"]
   ├─▶ Start PTY process
   ├─▶ Register terminal (RUNNING)
   └─▶ Persist to terminals.json

5. Response: TerminalInfo { id, status: "running", ... }

6. Navigate to /terminals/{id}
   └─▶ Connect WebSocket
       └─▶ Two-way communication
```

### Deleting Sessions and Cache Invalidation

```
1. User selects multiple sessions
   └─▶ Clicks "Delete Selected"

2. POST /api/projects/{path}/sessions/delete
   { session_ids: ["uuid1", "uuid2", ...] }

3. SessionsHandler.DeleteMultiple()
   ├─▶ For each session:
   │   ├─▶ ClaudeService.DeleteSession()
   │   │   └─▶ os.Remove(sessionFile)
   │   └─▶ TerminalService.RemoveFromSaved()
   │       └─▶ persistSaved()
   └─▶ AnalyticsService.Invalidate(projectPath)
       └─▶ Clear cache for project + global

4. Response: { deleted: N }

5. Next analytics request will recompute
```

---

## 7. State Persistence

### terminals.json
```json
[
  {
    "id": "uuid",
    "name": "Terminal Name",
    "work_dir": "/path/to/dir",
    "session_id": "claude-session-uuid",
    "type": "claude",
    "status": "stopped",
    "created_at": "2024-01-01T00:00:00Z",
    "last_access_at": "2024-01-01T00:00:00Z",
    "config": { ... }
  }
]
```

### Session Files (JSONL)
```jsonl
{"type":"user","cwd":"/real/path","message":{...},"timestamp":"..."}
{"type":"assistant","message":{...},"timestamp":"..."}
```

---

## 8. Error States

| Resource | Error Condition | Result |
|----------|-----------------|--------|
| Terminal | Invalid work_dir | 500 + "directorio invalido" |
| Terminal | Already active | 500 + "ya esta activa" |
| Terminal | Not found | 404 + "no encontrada" |
| Terminal | Kill inactive | 404 + "no activa" |
| Session | Not found | 404 |
| Project | Not found | 404 |
| Auth | Invalid credentials | 401 Unauthorized |
