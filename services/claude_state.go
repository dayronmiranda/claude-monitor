package services

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 1. Estados y Tipos
// ============================================================================

// ClaudeState representa el estado actual de una sesión de Claude Code
type ClaudeState string

const (
	StateUnknown          ClaudeState = "unknown"
	StateWaitingInput     ClaudeState = "waiting_input"
	StateGenerating       ClaudeState = "generating"
	StatePermissionPrompt ClaudeState = "permission_prompt"
	StateToolRunning      ClaudeState = "tool_running"
	StateBackgroundTask   ClaudeState = "background_task"
	StateError            ClaudeState = "error"
	StateExited           ClaudeState = "exited"
)

// ClaudeMode representa el modo de edición actual
type ClaudeMode string

const (
	ModeNormal  ClaudeMode = "normal"
	ModeVim     ClaudeMode = "vim"
	ModePlan    ClaudeMode = "plan"
	ModeCompact ClaudeMode = "compact"
)

// VimSubMode representa el submodo dentro de vim mode
type VimSubMode string

const (
	VimInsert  VimSubMode = "insert"
	VimNormal  VimSubMode = "normal"
	VimVisual  VimSubMode = "visual"
	VimCommand VimSubMode = "command"
)

// PermissionMode representa el modo de permisos
type PermissionMode string

const (
	PermDefault          PermissionMode = "default"
	PermPlan             PermissionMode = "plan"
	PermAcceptEdits      PermissionMode = "acceptEdits"
	PermDontAsk          PermissionMode = "dontAsk"
	PermBypassPermissions PermissionMode = "bypassPermissions"
)

// ============================================================================
// 2. Patrones de Detección
// ============================================================================

// OutputPattern define un patrón a detectar en el output
type OutputPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Type        string
	Priority    int
	Description string
}

// Patrones compilados para detección
var claudePatterns = []OutputPattern{
	// Prompts de permiso (alta prioridad)
	{Name: "permission_allow", Pattern: regexp.MustCompile(`(?i)Allow\s+\w+.*to`), Type: "permission", Priority: 100, Description: "Solicitud de permiso"},
	{Name: "permission_yn", Pattern: regexp.MustCompile(`\[y/n\]`), Type: "permission", Priority: 100, Description: "Confirmación sí/no"},
	{Name: "permission_Yn", Pattern: regexp.MustCompile(`\[Y/n\]`), Type: "permission", Priority: 100, Description: "Confirmación (default yes)"},
	{Name: "permission_yN", Pattern: regexp.MustCompile(`\[y/N\]`), Type: "permission", Priority: 100, Description: "Confirmación (default no)"},

	// Estados de herramientas
	{Name: "tool_running", Pattern: regexp.MustCompile(`(?i)^Running:`), Type: "tool", Priority: 80, Description: "Herramienta ejecutándose"},
	{Name: "tool_writing", Pattern: regexp.MustCompile(`(?i)^Writing:`), Type: "tool", Priority: 80, Description: "Escribiendo archivo"},
	{Name: "tool_reading", Pattern: regexp.MustCompile(`(?i)^Reading:`), Type: "tool", Priority: 80, Description: "Leyendo archivo"},
	{Name: "tool_searching", Pattern: regexp.MustCompile(`(?i)^Searching:`), Type: "tool", Priority: 80, Description: "Buscando"},
	{Name: "tool_editing", Pattern: regexp.MustCompile(`(?i)^Editing:`), Type: "tool", Priority: 80, Description: "Editando archivo"},

	// Spinners de progreso (detectar caracteres de spinner)
	{Name: "spinner", Pattern: regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷]`), Type: "progress", Priority: 70, Description: "Spinner activo"},
	{Name: "progress_numeric", Pattern: regexp.MustCompile(`\[\d+/\d+\]`), Type: "progress", Priority: 70, Description: "Progreso numérico"},
	{Name: "progress_percent", Pattern: regexp.MustCompile(`\d+%`), Type: "progress", Priority: 60, Description: "Progreso porcentaje"},

	// Prompts de entrada
	{Name: "claude_prompt", Pattern: regexp.MustCompile(`^>\s*$`), Type: "prompt", Priority: 50, Description: "Prompt de Claude"},
	{Name: "input_prompt", Pattern: regexp.MustCompile(`claude>\s*$`), Type: "prompt", Priority: 50, Description: "Prompt con nombre"},

	// Modos especiales
	{Name: "vim_mode", Pattern: regexp.MustCompile(`(?i)vim mode`), Type: "mode", Priority: 90, Description: "Modo vim activo"},
	{Name: "plan_mode", Pattern: regexp.MustCompile(`(?i)plan mode`), Type: "mode", Priority: 90, Description: "Modo plan activo"},
	{Name: "vim_insert", Pattern: regexp.MustCompile(`-- INSERT --`), Type: "vim", Priority: 85, Description: "Vim modo inserción"},
	{Name: "vim_normal", Pattern: regexp.MustCompile(`-- NORMAL --`), Type: "vim", Priority: 85, Description: "Vim modo normal"},
	{Name: "vim_visual", Pattern: regexp.MustCompile(`-- VISUAL --`), Type: "vim", Priority: 85, Description: "Vim modo visual"},

	// Errores y éxitos
	{Name: "error", Pattern: regexp.MustCompile(`(?i)^Error:`), Type: "status", Priority: 95, Description: "Error"},
	{Name: "warning", Pattern: regexp.MustCompile(`(?i)^Warning:`), Type: "status", Priority: 85, Description: "Advertencia"},
	{Name: "success_check", Pattern: regexp.MustCompile(`✓`), Type: "status", Priority: 40, Description: "Éxito"},
	{Name: "failure_x", Pattern: regexp.MustCompile(`✗`), Type: "status", Priority: 40, Description: "Fallo"},

	// Slash commands
	{Name: "slash_command", Pattern: regexp.MustCompile(`^/\w+`), Type: "command", Priority: 60, Description: "Slash command"},

	// Tokens/costo
	{Name: "tokens_info", Pattern: regexp.MustCompile(`(?i)tokens?:`), Type: "info", Priority: 30, Description: "Info de tokens"},
	{Name: "cost_info", Pattern: regexp.MustCompile(`(?i)\$[\d.]+`), Type: "info", Priority: 30, Description: "Info de costo"},

	// Background tasks
	{Name: "background_task", Pattern: regexp.MustCompile(`(?i)background|task \d+`), Type: "background", Priority: 50, Description: "Tarea en background"},

	// Checkpoint/Rewind
	{Name: "checkpoint", Pattern: regexp.MustCompile(`(?i)checkpoint|rewind`), Type: "checkpoint", Priority: 70, Description: "Checkpoint/Rewind"},
}

// ============================================================================
// 3. Slash Commands conocidos
// ============================================================================

// SlashCommand representa un slash command de Claude
type SlashCommand struct {
	Name        string
	Category    string
	Description string
	HasArgs     bool
}

var knownSlashCommands = map[string]SlashCommand{
	// Gestión de sesión
	"clear":   {Name: "clear", Category: "session", Description: "Limpiar historial", HasArgs: false},
	"compact": {Name: "compact", Category: "session", Description: "Compactar conversación", HasArgs: true},
	"resume":  {Name: "resume", Category: "session", Description: "Reanudar sesión", HasArgs: true},
	"rewind":  {Name: "rewind", Category: "session", Description: "Volver a checkpoint", HasArgs: true},
	"exit":    {Name: "exit", Category: "session", Description: "Salir", HasArgs: false},

	// Información
	"cost":    {Name: "cost", Category: "info", Description: "Mostrar tokens usados", HasArgs: false},
	"context": {Name: "context", Category: "info", Description: "Visualizar contexto", HasArgs: false},
	"todos":   {Name: "todos", Category: "info", Description: "Listar TODOs", HasArgs: false},
	"stats":   {Name: "stats", Category: "info", Description: "Estadísticas de uso", HasArgs: false},
	"bashes":  {Name: "bashes", Category: "info", Description: "Tareas en background", HasArgs: false},
	"help":    {Name: "help", Category: "info", Description: "Ayuda", HasArgs: false},

	// Configuración
	"model":       {Name: "model", Category: "config", Description: "Cambiar modelo", HasArgs: true},
	"permissions": {Name: "permissions", Category: "config", Description: "Ver/cambiar permisos", HasArgs: false},
	"hooks":       {Name: "hooks", Category: "config", Description: "Gestionar hooks", HasArgs: false},
	"config":      {Name: "config", Category: "config", Description: "Abrir configuración", HasArgs: false},

	// Herramientas
	"plan":    {Name: "plan", Category: "tool", Description: "Modo planificación", HasArgs: false},
	"vim":     {Name: "vim", Category: "tool", Description: "Modo vim", HasArgs: false},
	"sandbox": {Name: "sandbox", Category: "tool", Description: "Activar sandbox", HasArgs: false},
	"review":  {Name: "review", Category: "tool", Description: "Code review", HasArgs: false},
	"init":    {Name: "init", Category: "tool", Description: "Inicializar proyecto", HasArgs: false},
	"memory":  {Name: "memory", Category: "tool", Description: "Editar CLAUDE.md", HasArgs: false},
	"rename":  {Name: "rename", Category: "tool", Description: "Renombrar sesión", HasArgs: true},
	"export":  {Name: "export", Category: "tool", Description: "Exportar conversación", HasArgs: true},

	// MCP
	"mcp": {Name: "mcp", Category: "mcp", Description: "Gestionar MCP servers", HasArgs: false},
}

// ============================================================================
// 4. Hook Events
// ============================================================================

// HookEventType representa tipos de eventos de hooks
type HookEventType string

const (
	HookPreToolUse        HookEventType = "PreToolUse"
	HookPostToolUse       HookEventType = "PostToolUse"
	HookPermissionRequest HookEventType = "PermissionRequest"
	HookUserPromptSubmit  HookEventType = "UserPromptSubmit"
	HookStop              HookEventType = "Stop"
	HookSubagentStop      HookEventType = "SubagentStop"
	HookSessionStart      HookEventType = "SessionStart"
	HookSessionEnd        HookEventType = "SessionEnd"
	HookNotification      HookEventType = "Notification"
	HookPreCompact        HookEventType = "PreCompact"
)

// HookEvent representa un evento detectado
type HookEvent struct {
	Type      HookEventType `json:"type"`
	Tool      string        `json:"tool,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Data      interface{}   `json:"data,omitempty"`
}

// ============================================================================
// 5. Checkpoint tracking
// ============================================================================

// Checkpoint representa un checkpoint detectado
type Checkpoint struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	ToolUsed  string    `json:"tool_used,omitempty"`
	FilesAffected []string `json:"files_affected,omitempty"`
}

// ============================================================================
// 6. ClaudeAwareScreenHandler - Handler extendido
// ============================================================================

// ClaudeStateInfo contiene toda la información de estado de Claude
type ClaudeStateInfo struct {
	// Estado principal
	State           ClaudeState    `json:"state"`
	Mode            ClaudeMode     `json:"mode"`
	VimSubMode      VimSubMode     `json:"vim_sub_mode,omitempty"`
	PermissionMode  PermissionMode `json:"permission_mode"`

	// Información detectada
	IsGenerating       bool   `json:"is_generating"`
	PendingPermission  bool   `json:"pending_permission"`
	PendingTool        string `json:"pending_tool,omitempty"`
	LastSlashCommand   string `json:"last_slash_command,omitempty"`
	LastToolUsed       string `json:"last_tool_used,omitempty"`

	// Métricas detectadas
	TokensEstimated    int     `json:"tokens_estimated,omitempty"`
	CostEstimated      float64 `json:"cost_estimated,omitempty"`

	// Background tasks
	BackgroundTasks    []string `json:"background_tasks,omitempty"`

	// Checkpoints
	LastCheckpointID   string      `json:"last_checkpoint_id,omitempty"`
	CheckpointCount    int         `json:"checkpoint_count"`
	CanRewind          bool        `json:"can_rewind"`

	// Patrones detectados
	ActivePatterns     []string `json:"active_patterns,omitempty"`

	// Eventos recientes
	RecentEvents       []HookEvent `json:"recent_events,omitempty"`

	// Timestamps
	LastActivity       time.Time `json:"last_activity"`
	StateChangedAt     time.Time `json:"state_changed_at"`
}

// ClaudeAwareScreenHandler extiende ScreenState con detección de Claude
type ClaudeAwareScreenHandler struct {
	*ScreenState
	mu sync.RWMutex

	// Estado actual
	stateInfo ClaudeStateInfo

	// Historial de checkpoints
	checkpoints []Checkpoint

	// Historial de eventos (últimos N)
	eventHistory    []HookEvent
	maxEventHistory int

	// Callbacks para eventos
	OnStateChange      func(old, new ClaudeState)
	OnPermissionPrompt func(tool string)
	OnSlashCommand     func(cmd string, args string)
	OnCheckpoint       func(cp Checkpoint)
	OnToolUse          func(tool string, phase string) // phase: "pre" or "post"
}

// NewClaudeAwareScreenHandler crea un nuevo handler con detección de Claude
func NewClaudeAwareScreenHandler(width, height int) *ClaudeAwareScreenHandler {
	return &ClaudeAwareScreenHandler{
		ScreenState: NewScreenState(width, height),
		stateInfo: ClaudeStateInfo{
			State:          StateUnknown,
			Mode:           ModeNormal,
			PermissionMode: PermDefault,
			LastActivity:   time.Now(),
			StateChangedAt: time.Now(),
		},
		checkpoints:     make([]Checkpoint, 0),
		eventHistory:    make([]HookEvent, 0),
		maxEventHistory: 100,
	}
}

// Feed procesa bytes y actualiza estado de Claude
func (h *ClaudeAwareScreenHandler) Feed(data []byte) error {
	// Primero alimentar el screen base
	if err := h.ScreenState.Feed(data); err != nil {
		return err
	}

	// Luego analizar para detección de Claude
	h.analyzeContent(string(data))

	return nil
}

// analyzeContent analiza el contenido para detectar estados de Claude
func (h *ClaudeAwareScreenHandler) analyzeContent(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	oldState := h.stateInfo.State
	h.stateInfo.LastActivity = time.Now()

	// Detectar patrones activos
	activePatterns := h.detectPatterns(content)
	h.stateInfo.ActivePatterns = activePatterns

	// Actualizar estado basado en patrones
	h.updateStateFromPatterns(activePatterns, content)

	// Detectar slash commands
	h.detectSlashCommands(content)

	// Detectar información de tokens/costo
	h.detectMetrics(content)

	// Notificar cambio de estado
	if oldState != h.stateInfo.State {
		h.stateInfo.StateChangedAt = time.Now()
		if h.OnStateChange != nil {
			go h.OnStateChange(oldState, h.stateInfo.State)
		}
	}
}

// detectPatterns encuentra patrones activos en el contenido
func (h *ClaudeAwareScreenHandler) detectPatterns(content string) []string {
	var patterns []string

	for _, p := range claudePatterns {
		if p.Pattern.MatchString(content) {
			patterns = append(patterns, p.Name)
		}
	}

	return patterns
}

// updateStateFromPatterns actualiza el estado basado en patrones detectados
func (h *ClaudeAwareScreenHandler) updateStateFromPatterns(patterns []string, content string) {
	// Priorizar por tipo de patrón
	hasPermission := false
	hasSpinner := false
	hasTool := false
	hasPrompt := false
	hasError := false

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
		case p == "vim_insert":
			h.stateInfo.VimSubMode = VimInsert
			h.stateInfo.Mode = ModeVim
		case p == "vim_normal":
			h.stateInfo.VimSubMode = VimNormal
			h.stateInfo.Mode = ModeVim
		case p == "vim_visual":
			h.stateInfo.VimSubMode = VimVisual
			h.stateInfo.Mode = ModeVim
		case p == "plan_mode":
			h.stateInfo.Mode = ModePlan
		}
	}

	// Determinar estado principal (por prioridad)
	if hasError {
		h.stateInfo.State = StateError
	} else if hasPermission {
		h.stateInfo.State = StatePermissionPrompt
		h.stateInfo.PendingPermission = true
		h.extractPendingTool(content)
	} else if hasTool {
		h.stateInfo.State = StateToolRunning
		h.extractToolName(content)
	} else if hasSpinner {
		h.stateInfo.State = StateGenerating
		h.stateInfo.IsGenerating = true
	} else if hasPrompt {
		h.stateInfo.State = StateWaitingInput
		h.stateInfo.IsGenerating = false
		h.stateInfo.PendingPermission = false
	}
}

// extractPendingTool extrae el nombre de la herramienta pendiente de permiso
func (h *ClaudeAwareScreenHandler) extractPendingTool(content string) {
	// Patrón: "Allow X to" o "Allow X("
	re := regexp.MustCompile(`(?i)Allow\s+(\w+)`)
	if matches := re.FindStringSubmatch(content); len(matches) > 1 {
		h.stateInfo.PendingTool = matches[1]
		if h.OnPermissionPrompt != nil {
			go h.OnPermissionPrompt(matches[1])
		}
	}
}

// extractToolName extrae el nombre de la herramienta en ejecución
func (h *ClaudeAwareScreenHandler) extractToolName(content string) {
	// Patrones: "Running: X", "Writing: file", etc.
	patterns := []string{
		`(?i)Running:\s*(\w+)`,
		`(?i)Writing:\s*(.+)`,
		`(?i)Reading:\s*(.+)`,
		`(?i)Editing:\s*(.+)`,
		`(?i)Searching:\s*(.+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			h.stateInfo.LastToolUsed = strings.TrimSpace(matches[1])
			break
		}
	}
}

// detectSlashCommands detecta slash commands en el contenido
func (h *ClaudeAwareScreenHandler) detectSlashCommands(content string) {
	re := regexp.MustCompile(`^/(\w+)(?:\s+(.*))?$`)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			cmd := matches[1]
			args := ""
			if len(matches) > 2 {
				args = matches[2]
			}

			h.stateInfo.LastSlashCommand = cmd

			// Verificar si es un comando conocido
			if _, known := knownSlashCommands[cmd]; known {
				// Actualizar modo si es relevante
				switch cmd {
				case "vim":
					h.stateInfo.Mode = ModeVim
				case "plan":
					h.stateInfo.Mode = ModePlan
				case "compact":
					h.stateInfo.Mode = ModeCompact
				case "clear":
					h.stateInfo.Mode = ModeNormal
				case "rewind":
					h.stateInfo.CanRewind = true
				}

				if h.OnSlashCommand != nil {
					go h.OnSlashCommand(cmd, args)
				}
			}
		}
	}
}

// detectMetrics detecta información de tokens y costo
func (h *ClaudeAwareScreenHandler) detectMetrics(content string) {
	// Detectar tokens
	tokenRe := regexp.MustCompile(`(?i)(\d+)\s*tokens?`)
	if matches := tokenRe.FindStringSubmatch(content); len(matches) > 1 {
		// Parsear número de tokens (simplificado)
		var tokens int
		if _, err := parseIntFromString(matches[1]); err == nil {
			h.stateInfo.TokensEstimated = tokens
		}
	}

	// Detectar costo
	costRe := regexp.MustCompile(`\$(\d+\.?\d*)`)
	if matches := costRe.FindStringSubmatch(content); len(matches) > 1 {
		// Parsear costo (simplificado)
		// h.stateInfo.CostEstimated = parsedCost
	}
}

// parseIntFromString helper para parsear int
func parseIntFromString(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}

// AddCheckpoint agrega un checkpoint al historial
func (h *ClaudeAwareScreenHandler) AddCheckpoint(id string, tool string, files []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cp := Checkpoint{
		ID:            id,
		Timestamp:     time.Now(),
		ToolUsed:      tool,
		FilesAffected: files,
	}

	h.checkpoints = append(h.checkpoints, cp)
	h.stateInfo.LastCheckpointID = id
	h.stateInfo.CheckpointCount = len(h.checkpoints)
	h.stateInfo.CanRewind = true

	if h.OnCheckpoint != nil {
		go h.OnCheckpoint(cp)
	}
}

// AddEvent agrega un evento al historial
func (h *ClaudeAwareScreenHandler) AddEvent(eventType HookEventType, tool string, data interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	event := HookEvent{
		Type:      eventType,
		Tool:      tool,
		Timestamp: time.Now(),
		Data:      data,
	}

	h.eventHistory = append(h.eventHistory, event)

	// Mantener solo los últimos N eventos
	if len(h.eventHistory) > h.maxEventHistory {
		h.eventHistory = h.eventHistory[len(h.eventHistory)-h.maxEventHistory:]
	}

	// Actualizar eventos recientes en stateInfo
	recentCount := 10
	if len(h.eventHistory) < recentCount {
		recentCount = len(h.eventHistory)
	}
	h.stateInfo.RecentEvents = h.eventHistory[len(h.eventHistory)-recentCount:]

	// Notificar callback de tool use
	if h.OnToolUse != nil && (eventType == HookPreToolUse || eventType == HookPostToolUse) {
		phase := "pre"
		if eventType == HookPostToolUse {
			phase = "post"
		}
		go h.OnToolUse(tool, phase)
	}
}

// GetClaudeState retorna el estado actual de Claude
func (h *ClaudeAwareScreenHandler) GetClaudeState() ClaudeStateInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Crear copia para evitar race conditions
	info := h.stateInfo
	info.ActivePatterns = make([]string, len(h.stateInfo.ActivePatterns))
	copy(info.ActivePatterns, h.stateInfo.ActivePatterns)

	info.BackgroundTasks = make([]string, len(h.stateInfo.BackgroundTasks))
	copy(info.BackgroundTasks, h.stateInfo.BackgroundTasks)

	info.RecentEvents = make([]HookEvent, len(h.stateInfo.RecentEvents))
	copy(info.RecentEvents, h.stateInfo.RecentEvents)

	return info
}

// GetCheckpoints retorna el historial de checkpoints
func (h *ClaudeAwareScreenHandler) GetCheckpoints() []Checkpoint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cps := make([]Checkpoint, len(h.checkpoints))
	copy(cps, h.checkpoints)
	return cps
}

// GetEventHistory retorna el historial de eventos
func (h *ClaudeAwareScreenHandler) GetEventHistory() []HookEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	events := make([]HookEvent, len(h.eventHistory))
	copy(events, h.eventHistory)
	return events
}

// SetPermissionMode establece el modo de permisos
func (h *ClaudeAwareScreenHandler) SetPermissionMode(mode PermissionMode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stateInfo.PermissionMode = mode
}

// SetMode establece el modo de edición
func (h *ClaudeAwareScreenHandler) SetMode(mode ClaudeMode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stateInfo.Mode = mode
}

// AddBackgroundTask agrega una tarea en background
func (h *ClaudeAwareScreenHandler) AddBackgroundTask(taskID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stateInfo.BackgroundTasks = append(h.stateInfo.BackgroundTasks, taskID)
}

// RemoveBackgroundTask elimina una tarea en background
func (h *ClaudeAwareScreenHandler) RemoveBackgroundTask(taskID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tasks := make([]string, 0, len(h.stateInfo.BackgroundTasks))
	for _, t := range h.stateInfo.BackgroundTasks {
		if t != taskID {
			tasks = append(tasks, t)
		}
	}
	h.stateInfo.BackgroundTasks = tasks
}

// ============================================================================
// 7. Funciones de utilidad para detección de pantalla completa
// ============================================================================

// DetectStateFromScreen analiza toda la pantalla para determinar estado
func (h *ClaudeAwareScreenHandler) DetectStateFromScreen() ClaudeState {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Obtener contenido de pantalla
	display := h.ScreenState.GetDisplay()
	if len(display) == 0 {
		return StateUnknown
	}

	// Analizar última línea para prompt
	lastLine := strings.TrimSpace(display[len(display)-1])

	// Buscar en todas las líneas visibles
	fullContent := strings.Join(display, "\n")

	// Detectar permiso pendiente
	for _, p := range claudePatterns {
		if p.Type == "permission" && p.Pattern.MatchString(fullContent) {
			return StatePermissionPrompt
		}
	}

	// Detectar spinner (generando)
	for _, p := range claudePatterns {
		if p.Name == "spinner" && p.Pattern.MatchString(fullContent) {
			return StateGenerating
		}
	}

	// Detectar prompt de entrada
	if strings.HasSuffix(lastLine, ">") || lastLine == ">" {
		return StateWaitingInput
	}

	// Detectar herramienta corriendo
	for _, p := range claudePatterns {
		if p.Type == "tool" && p.Pattern.MatchString(fullContent) {
			return StateToolRunning
		}
	}

	return StateUnknown
}

// IsReadyForInput retorna true si Claude está listo para recibir input
func (h *ClaudeAwareScreenHandler) IsReadyForInput() bool {
	state := h.DetectStateFromScreen()
	return state == StateWaitingInput
}

// HasPendingPermission retorna true si hay un permiso pendiente
func (h *ClaudeAwareScreenHandler) HasPendingPermission() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stateInfo.PendingPermission
}

// IsGenerating retorna true si Claude está generando
func (h *ClaudeAwareScreenHandler) IsGenerating() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stateInfo.IsGenerating
}
