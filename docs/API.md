# Claude Monitor API Documentation

## Base URL

```
http://localhost:8080/api
```

## Response Format

All responses follow this structure:

```json
{
  "success": true,
  "data": { ... },
  "meta": { "total": 10 }  // Optional, for lists
}
```

Error responses:

```json
{
  "success": false,
  "error": "Error message"
}
```

---

## Table of Contents

1. [Host](#host)
2. [Projects](#projects)
3. [Sessions](#sessions)
4. [Terminals](#terminals)
5. [Terminal Screen State](#terminal-screen-state)
6. [Claude State Detection](#claude-state-detection)
7. [Jobs](#jobs)
8. [Analytics](#analytics)
9. [Filesystem](#filesystem)

---

## Host

### Get Host Info

```http
GET /api/host
```

Returns server information.

**Response:**

```json
{
  "success": true,
  "data": {
    "hostname": "my-server",
    "version": "1.0.0",
    "claude_dir": "/home/user/.claude",
    "uptime": 3600,
    "terminals_active": 2,
    "terminals_saved": 5
  }
}
```

### Health Check

```http
GET /api/health
```

**Response:**

```json
{
  "status": "healthy"
}
```

### Readiness Check

```http
GET /api/ready
```

**Response:**

```json
{
  "status": "ready"
}
```

---

## Projects

### List Projects

```http
GET /api/projects
```

Returns all Claude projects.

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "encoded-path",
      "name": "my-project",
      "path": "/home/user/my-project",
      "sessions_count": 5,
      "last_activity": "2026-01-16T10:30:00Z"
    }
  ],
  "meta": { "total": 1 }
}
```

### Get Project

```http
GET /api/projects/{encoded_path}
```

**Parameters:**
- `encoded_path`: URL-encoded project path

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "encoded-path",
    "name": "my-project",
    "path": "/home/user/my-project",
    "sessions_count": 5,
    "last_activity": "2026-01-16T10:30:00Z"
  }
}
```

### Get Project Activity

```http
GET /api/projects/{encoded_path}/activity
```

Returns activity statistics for a project.

### Delete Project

```http
DELETE /api/projects/{encoded_path}
```

Deletes a project and all its sessions.

---

## Sessions

### List Sessions

```http
GET /api/projects/{encoded_path}/sessions
```

Returns all sessions for a project.

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "session-uuid",
      "name": "Feature implementation",
      "project_path": "/home/user/my-project",
      "created_at": "2026-01-16T08:00:00Z",
      "updated_at": "2026-01-16T10:30:00Z",
      "message_count": 42,
      "cwd": "/home/user/my-project/src"
    }
  ],
  "meta": { "total": 1 }
}
```

### Get Session

```http
GET /api/projects/{encoded_path}/sessions/{session_id}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "session-uuid",
    "name": "Feature implementation",
    "project_path": "/home/user/my-project",
    "created_at": "2026-01-16T08:00:00Z",
    "updated_at": "2026-01-16T10:30:00Z",
    "message_count": 42
  }
}
```

### Get Session Messages

```http
GET /api/projects/{encoded_path}/sessions/{session_id}/messages
```

**Query Parameters:**
- `limit` (optional): Number of messages to return
- `offset` (optional): Offset for pagination

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "type": "user",
      "content": "Hello Claude",
      "timestamp": "2026-01-16T08:00:00Z"
    },
    {
      "type": "assistant",
      "content": "Hello! How can I help?",
      "timestamp": "2026-01-16T08:00:01Z"
    }
  ],
  "meta": { "total": 42 }
}
```

### Get Real-Time Messages

```http
GET /api/projects/{encoded_path}/sessions/{session_id}/messages/realtime
```

Returns messages from active session file (for live sessions).

### Rename Session

```http
PUT /api/projects/{encoded_path}/sessions/{session_id}/rename
```

**Request Body:**

```json
{
  "name": "New session name"
}
```

### Delete Session

```http
DELETE /api/projects/{encoded_path}/sessions/{session_id}
```

### Delete Multiple Sessions

```http
POST /api/projects/{encoded_path}/sessions/delete
```

**Request Body:**

```json
{
  "session_ids": ["id1", "id2", "id3"]
}
```

### Clean Empty Sessions

```http
POST /api/projects/{encoded_path}/sessions/clean
```

Removes sessions with no messages.

### Import Sessions

```http
POST /api/projects/{encoded_path}/sessions/import
```

Imports sessions to terminal manager.

---

## Terminals

### List Terminals

```http
GET /api/terminals
```

Returns all terminals (active and saved).

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "terminal-uuid",
      "name": "my-project",
      "work_dir": "/home/user/my-project",
      "session_id": "session-uuid",
      "type": "claude",
      "status": "running",
      "model": "claude-sonnet-4-20250514",
      "active": true,
      "clients": 1,
      "can_resume": true,
      "started_at": "2026-01-16T08:00:00Z"
    }
  ],
  "meta": { "total": 1 }
}
```

### Create Terminal

```http
POST /api/terminals
```

**Request Body:**

```json
{
  "name": "my-terminal",
  "work_dir": "/home/user/my-project",
  "type": "claude",
  "model": "claude-sonnet-4-20250514",
  "resume": false,
  "continue": false,
  "allowed_tools": ["Read", "Write", "Bash"],
  "disallowed_tools": []
}
```

**Parameters:**
- `name` (optional): Terminal name (defaults to directory name)
- `work_dir` (required): Working directory path
- `type` (optional): "claude" or "terminal" (default: "claude")
- `model` (optional): Claude model to use
- `resume` (optional): Resume existing session
- `continue` (optional): Continue last session
- `allowed_tools` (optional): List of allowed tools
- `disallowed_tools` (optional): List of disallowed tools

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "terminal-uuid",
    "name": "my-terminal",
    "work_dir": "/home/user/my-project",
    "type": "claude",
    "status": "running",
    "active": true
  }
}
```

### Get Terminal

```http
GET /api/terminals/{id}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "terminal-uuid",
    "name": "my-terminal",
    "work_dir": "/home/user/my-project",
    "type": "claude",
    "status": "running",
    "active": true,
    "clients": 2
  }
}
```

### Delete Terminal

```http
DELETE /api/terminals/{id}
```

Deletes a saved terminal (must not be active).

### Kill Terminal

```http
POST /api/terminals/{id}/kill
```

Terminates an active terminal (sends SIGTERM).

### Resume Terminal

```http
POST /api/terminals/{id}/resume
```

Resumes a stopped Claude terminal.

### Resize Terminal

```http
POST /api/terminals/{id}/resize
```

**Request Body:**

```json
{
  "rows": 24,
  "cols": 80
}
```

### WebSocket Connection

```http
WS /api/terminals/{id}/ws
```

Bidirectional WebSocket for terminal I/O.

**Client → Server Messages:**

```json
// Send input
{
  "type": "input",
  "data": "ls -la\n"
}

// Resize terminal
{
  "type": "resize",
  "rows": 30,
  "cols": 120
}
```

**Server → Client Messages:**

```json
// Initial snapshot (on connect)
{
  "type": "snapshot",
  "snapshot": {
    "content": "...",
    "display": ["line1", "line2"],
    "cursor_x": 0,
    "cursor_y": 5,
    "width": 80,
    "height": 24
  }
}

// Terminal output
{
  "type": "output",
  "data": "command output here"
}

// Terminal closed
{
  "type": "closed",
  "message": "Terminal terminada"
}

// Server shutdown
{
  "type": "shutdown",
  "message": "Servidor terminando"
}
```

---

## Terminal Screen State

### Get Screen Snapshot

```http
GET /api/terminals/{id}/snapshot
```

Returns the current screen state (useful for reconnection).

**Response:**

```json
{
  "success": true,
  "data": {
    "content": "user@host:~$ ls\nfile1.txt  file2.txt\nuser@host:~$ ",
    "display": [
      "user@host:~$ ls",
      "file1.txt  file2.txt",
      "user@host:~$ "
    ],
    "cursor_x": 14,
    "cursor_y": 2,
    "width": 80,
    "height": 24,
    "in_alternate_screen": false,
    "history": [
      "Previous scrollback line 1",
      "Previous scrollback line 2"
    ]
  }
}
```

**Fields:**
- `content`: Plain text representation of screen
- `display`: Array of screen lines (trimmed)
- `cursor_x`, `cursor_y`: Current cursor position
- `width`, `height`: Terminal dimensions
- `in_alternate_screen`: True if in vim/htop/less mode
- `history`: Scrollback buffer (lines that scrolled off screen)

---

## Claude State Detection

These endpoints are only available for terminals of type "claude".

### Get Claude State

```http
GET /api/terminals/{id}/claude-state
```

Returns detected Claude CLI state.

**Response:**

```json
{
  "success": true,
  "data": {
    "state": "waiting_input",
    "mode": "normal",
    "vim_sub_mode": "",
    "permission_mode": "default",

    "is_generating": false,
    "pending_permission": false,
    "pending_tool": "",
    "last_slash_command": "clear",
    "last_tool_used": "Edit",

    "tokens_estimated": 15000,
    "cost_estimated": 0.05,

    "background_tasks": [],

    "last_checkpoint_id": "cp-abc123",
    "checkpoint_count": 5,
    "can_rewind": true,

    "active_patterns": ["claude_prompt"],

    "recent_events": [
      {
        "type": "PostToolUse",
        "tool": "Edit",
        "timestamp": "2026-01-16T10:30:00Z"
      }
    ],

    "last_activity": "2026-01-16T10:30:00Z",
    "state_changed_at": "2026-01-16T10:29:55Z"
  }
}
```

**State Values:**
| State | Description |
|-------|-------------|
| `unknown` | State cannot be determined |
| `waiting_input` | Ready for user input (shows ">") |
| `generating` | Claude is generating a response |
| `permission_prompt` | Waiting for permission approval |
| `tool_running` | A tool is currently executing |
| `background_task` | Background task is running |
| `error` | An error occurred |
| `exited` | Claude has exited |

**Mode Values:**
| Mode | Description |
|------|-------------|
| `normal` | Standard input mode |
| `vim` | Vim mode active (/vim) |
| `plan` | Plan mode active (/plan) |
| `compact` | Compact mode active |

**Vim Sub-Modes:**
| SubMode | Description |
|---------|-------------|
| `insert` | -- INSERT -- |
| `normal` | -- NORMAL -- |
| `visual` | -- VISUAL -- |
| `command` | : command line |

**Permission Modes:**
| Mode | Description |
|------|-------------|
| `default` | Normal permission prompts |
| `plan` | Plan mode permissions |
| `acceptEdits` | Auto-accept file edits |
| `dontAsk` | Don't ask for permissions |
| `bypassPermissions` | Bypass all permissions |

### Get Checkpoints

```http
GET /api/terminals/{id}/checkpoints
```

Returns checkpoint history for rewind functionality.

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "cp-abc123",
      "timestamp": "2026-01-16T10:30:00Z",
      "tool_used": "Edit",
      "files_affected": [
        "src/main.go",
        "src/utils.go"
      ]
    },
    {
      "id": "cp-def456",
      "timestamp": "2026-01-16T10:25:00Z",
      "tool_used": "Write",
      "files_affected": [
        "README.md"
      ]
    }
  ],
  "meta": { "total": 2 }
}
```

### Get Events

```http
GET /api/terminals/{id}/events
```

Returns event history (hooks).

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "type": "PostToolUse",
      "tool": "Edit",
      "timestamp": "2026-01-16T10:30:00Z",
      "data": null
    },
    {
      "type": "PreToolUse",
      "tool": "Bash",
      "timestamp": "2026-01-16T10:29:00Z",
      "data": null
    },
    {
      "type": "PermissionRequest",
      "tool": "Write",
      "timestamp": "2026-01-16T10:28:00Z",
      "data": null
    }
  ],
  "meta": { "total": 3 }
}
```

**Event Types:**
| Type | Description |
|------|-------------|
| `PreToolUse` | Before a tool executes |
| `PostToolUse` | After a tool completes |
| `PermissionRequest` | Permission prompt shown |
| `UserPromptSubmit` | User submitted input |
| `Stop` | Agent stopped |
| `SubagentStop` | Subagent stopped |
| `SessionStart` | Session started |
| `SessionEnd` | Session ended |
| `Notification` | Notification sent |
| `PreCompact` | Before context compaction |

---

## Jobs

Jobs represent unified sessions (active terminals + historical sessions).

### List Jobs

```http
GET /api/jobs
```

**Query Parameters:**
- `project_path` (optional): Filter by project path

### Get Job

```http
GET /api/jobs/{id}
```

### Create Job

```http
POST /api/jobs
```

### Job Transitions

```http
POST /api/jobs/{id}/start
POST /api/jobs/{id}/pause
POST /api/jobs/{id}/stop
POST /api/jobs/{id}/resume
POST /api/jobs/{id}/archive
POST /api/jobs/{id}/retry
POST /api/jobs/{id}/discard
```

### Get Job Messages

```http
GET /api/jobs/{id}/messages
```

---

## Analytics

### Get Global Analytics

```http
GET /api/analytics/global
```

**Response:**

```json
{
  "success": true,
  "data": {
    "total_projects": 10,
    "total_sessions": 50,
    "total_messages": 5000,
    "active_terminals": 2
  }
}
```

### Get Project Analytics

```http
GET /api/analytics/projects/{encoded_path}
```

### Invalidate Cache

```http
POST /api/analytics/invalidate
```

### Get Cache Status

```http
GET /api/analytics/cache
```

---

## Filesystem

### List Directory

```http
GET /api/filesystem/dir?path=/home/user
```

**Query Parameters:**
- `path` (optional): Directory path (default: "/")

**Response:**

```json
{
  "success": true,
  "data": {
    "current_path": "/home/user",
    "entries": [
      {
        "name": "..",
        "path": "/home",
        "is_dir": true,
        "size": 0
      },
      {
        "name": "projects",
        "path": "/home/user/projects",
        "is_dir": true,
        "size": 0
      },
      {
        "name": "file.txt",
        "path": "/home/user/file.txt",
        "is_dir": false,
        "size": 1024
      }
    ]
  }
}
```

---

## Error Codes

| HTTP Code | Description |
|-----------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid parameters |
| 404 | Not Found - Resource doesn't exist |
| 405 | Method Not Allowed |
| 409 | Conflict - Resource already exists |
| 500 | Internal Server Error |

---

## WebSocket Ping/Pong

The WebSocket connection uses ping/pong frames to maintain the connection:

- **Ping Interval**: 30 seconds
- **Pong Timeout**: 60 seconds

If no pong is received within the timeout, the connection is closed.

---

## Detected Patterns

The Claude state detection system recognizes these patterns:

### Permission Patterns
- `Allow X to` - Permission request
- `[y/n]` - Yes/No confirmation
- `[Y/n]` - Default yes confirmation
- `[y/N]` - Default no confirmation

### Tool Patterns
- `Running:` - Tool executing
- `Writing:` - File being written
- `Reading:` - File being read
- `Editing:` - File being edited
- `Searching:` - Search in progress

### Progress Patterns
- Spinner characters: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
- Numeric progress: `[3/10]`
- Percentage: `45%`

### Mode Patterns
- `-- INSERT --` - Vim insert mode
- `-- NORMAL --` - Vim normal mode
- `-- VISUAL --` - Vim visual mode
- `vim mode` - Vim mode active
- `plan mode` - Plan mode active

### Status Patterns
- `Error:` - Error message
- `Warning:` - Warning message
- `✓` - Success indicator
- `✗` - Failure indicator

---

## Known Slash Commands

The system recognizes these Claude CLI slash commands:

### Session Management
- `/clear` - Clear history
- `/compact` - Compact conversation
- `/resume` - Resume session
- `/rewind` - Rewind to checkpoint
- `/exit` - Exit Claude

### Information
- `/cost` - Show token usage
- `/context` - Show context usage
- `/todos` - List TODOs
- `/stats` - Show statistics
- `/bashes` - List background tasks
- `/help` - Show help

### Configuration
- `/model` - Change model
- `/permissions` - View/change permissions
- `/hooks` - Manage hooks
- `/config` - Open config

### Tools
- `/plan` - Enter plan mode
- `/vim` - Enter vim mode
- `/sandbox` - Enable sandbox
- `/review` - Request code review
- `/init` - Initialize project
- `/memory` - Edit CLAUDE.md
- `/rename` - Rename session
- `/export` - Export conversation
