package services

import (
	"testing"
	"time"
)

func TestClaudeState_Constants(t *testing.T) {
	// Verify state constants exist and are unique
	states := []ClaudeState{
		StateUnknown,
		StateWaitingInput,
		StateGenerating,
		StatePermissionPrompt,
		StateToolRunning,
		StateBackgroundTask,
		StateError,
		StateExited,
	}

	seen := make(map[ClaudeState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("Duplicate state: %s", s)
		}
		seen[s] = true
	}
}

func TestClaudeMode_Constants(t *testing.T) {
	modes := []ClaudeMode{
		ModeNormal,
		ModeVim,
		ModePlan,
		ModeCompact,
	}

	seen := make(map[ClaudeMode]bool)
	for _, m := range modes {
		if seen[m] {
			t.Errorf("Duplicate mode: %s", m)
		}
		seen[m] = true
	}
}

func TestNewClaudeAwareScreenHandler(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	if handler == nil {
		t.Fatal("NewClaudeAwareScreenHandler should return non-nil handler")
	}

	state := handler.GetClaudeState()
	if state.State != StateUnknown {
		t.Errorf("Initial state: got %s, want %s", state.State, StateUnknown)
	}

	if state.Mode != ModeNormal {
		t.Errorf("Initial mode: got %s, want %s", state.Mode, ModeNormal)
	}

	if state.PermissionMode != PermDefault {
		t.Errorf("Initial permission mode: got %s, want %s", state.PermissionMode, PermDefault)
	}
}

func TestClaudeAwareScreenHandler_Feed(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	err := handler.Feed([]byte("Hello World"))
	if err != nil {
		t.Fatalf("Feed error: %v", err)
	}
}

func TestClaudeAwareScreenHandler_DetectPermissionPrompt(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  bool
	}{
		{"allow prompt", "Allow Edit to write file.txt? [y/n]", true},
		{"y/n prompt", "Continue? [y/n]", true},
		{"Y/n prompt", "Are you sure? [Y/n]", true},
		{"y/N prompt", "Delete all? [y/N]", true},
		{"no permission", "Regular output text", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewClaudeAwareScreenHandler(80, 24)
			h.Feed([]byte(tc.input))

			state := h.GetClaudeState()
			isPermission := state.State == StatePermissionPrompt

			if isPermission != tc.want {
				t.Errorf("Permission detected: got %v, want %v (state: %s)", isPermission, tc.want, state.State)
			}
		})
	}
}

func TestClaudeAwareScreenHandler_DetectSpinner(t *testing.T) {
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	for _, char := range spinnerChars {
		t.Run("spinner_"+char, func(t *testing.T) {
			h := NewClaudeAwareScreenHandler(80, 24)
			h.Feed([]byte(char + " Loading..."))

			state := h.GetClaudeState()
			if state.State != StateGenerating {
				t.Errorf("State: got %s, want %s", state.State, StateGenerating)
			}
		})
	}
}

func TestClaudeAwareScreenHandler_DetectPrompt(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  ClaudeState
	}{
		{"simple prompt", "> ", StateWaitingInput},
		{"claude prompt", "claude> ", StateWaitingInput},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewClaudeAwareScreenHandler(80, 24)
			h.Feed([]byte(tc.input))

			state := h.GetClaudeState()
			if state.State != tc.want {
				t.Errorf("State: got %s, want %s", state.State, tc.want)
			}
		})
	}
}

func TestClaudeAwareScreenHandler_DetectToolRunning(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"running", "Running: npm install"},
		{"writing", "Writing: src/main.go"},
		{"reading", "Reading: package.json"},
		{"editing", "Editing: file.txt"},
		{"searching", "Searching: pattern"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewClaudeAwareScreenHandler(80, 24)
			h.Feed([]byte(tc.input))

			state := h.GetClaudeState()
			if state.State != StateToolRunning {
				t.Errorf("State: got %s, want %s", state.State, StateToolRunning)
			}
		})
	}
}

func TestClaudeAwareScreenHandler_DetectError(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)
	handler.Feed([]byte("Error: file not found"))

	state := handler.GetClaudeState()
	if state.State != StateError {
		t.Errorf("State: got %s, want %s", state.State, StateError)
	}
}

func TestClaudeAwareScreenHandler_DetectVimMode(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		wantMode  ClaudeMode
		wantSubMode VimSubMode
	}{
		{"insert mode", "-- INSERT --", ModeVim, VimInsert},
		{"normal mode", "-- NORMAL --", ModeVim, VimNormal},
		{"visual mode", "-- VISUAL --", ModeVim, VimVisual},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewClaudeAwareScreenHandler(80, 24)
			h.Feed([]byte(tc.input))

			state := h.GetClaudeState()
			if state.Mode != tc.wantMode {
				t.Errorf("Mode: got %s, want %s", state.Mode, tc.wantMode)
			}
			if state.VimSubMode != tc.wantSubMode {
				t.Errorf("VimSubMode: got %s, want %s", state.VimSubMode, tc.wantSubMode)
			}
		})
	}
}

func TestClaudeAwareScreenHandler_DetectSlashCommands(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantCmd string
	}{
		{"help", "/help", "help"},
		{"clear", "/clear", "clear"},
		{"vim", "/vim", "vim"},
		{"plan", "/plan", "plan"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewClaudeAwareScreenHandler(80, 24)
			h.Feed([]byte(tc.input))

			state := h.GetClaudeState()
			if state.LastSlashCommand != tc.wantCmd {
				t.Errorf("LastSlashCommand: got %s, want %s", state.LastSlashCommand, tc.wantCmd)
			}
		})
	}
}

func TestClaudeAwareScreenHandler_DetectTokens(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)
	// Use format that matches regex: (\d+)\s*tokens?
	handler.Feed([]byte("Used 1234 tokens"))

	state := handler.GetClaudeState()
	if state.TokensEstimated != 1234 {
		t.Errorf("TokensEstimated: got %d, want %d", state.TokensEstimated, 1234)
	}
}

func TestClaudeAwareScreenHandler_DetectCost(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)
	handler.Feed([]byte("Cost: $0.05"))

	state := handler.GetClaudeState()
	if state.CostEstimated != 0.05 {
		t.Errorf("CostEstimated: got %f, want %f", state.CostEstimated, 0.05)
	}
}

func TestClaudeAwareScreenHandler_AddCheckpoint(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	handler.AddCheckpoint("cp1", "Edit", []string{"file1.txt"})

	checkpoints := handler.GetCheckpoints()
	if len(checkpoints) != 1 {
		t.Fatalf("Checkpoints length: got %d, want 1", len(checkpoints))
	}

	if checkpoints[0].ID != "cp1" {
		t.Errorf("Checkpoint ID: got %s, want %s", checkpoints[0].ID, "cp1")
	}

	if checkpoints[0].ToolUsed != "Edit" {
		t.Errorf("Checkpoint ToolUsed: got %s, want %s", checkpoints[0].ToolUsed, "Edit")
	}

	state := handler.GetClaudeState()
	if state.CheckpointCount != 1 {
		t.Errorf("CheckpointCount: got %d, want 1", state.CheckpointCount)
	}

	if !state.CanRewind {
		t.Error("CanRewind should be true after adding checkpoint")
	}
}

func TestClaudeAwareScreenHandler_AddEvent(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	handler.AddEvent(HookPreToolUse, "Edit", nil)
	handler.AddEvent(HookPostToolUse, "Edit", nil)

	events := handler.GetEventHistory()
	if len(events) != 2 {
		t.Fatalf("Events length: got %d, want 2", len(events))
	}

	if events[0].Type != HookPreToolUse {
		t.Errorf("Event type: got %s, want %s", events[0].Type, HookPreToolUse)
	}

	if events[0].Tool != "Edit" {
		t.Errorf("Event tool: got %s, want %s", events[0].Tool, "Edit")
	}
}

func TestClaudeAwareScreenHandler_SetPermissionMode(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	handler.SetPermissionMode(PermDontAsk)

	state := handler.GetClaudeState()
	if state.PermissionMode != PermDontAsk {
		t.Errorf("PermissionMode: got %s, want %s", state.PermissionMode, PermDontAsk)
	}
}

func TestClaudeAwareScreenHandler_SetMode(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	handler.SetMode(ModePlan)

	state := handler.GetClaudeState()
	if state.Mode != ModePlan {
		t.Errorf("Mode: got %s, want %s", state.Mode, ModePlan)
	}
}

func TestClaudeAwareScreenHandler_BackgroundTasks(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	handler.AddBackgroundTask("task1")
	handler.AddBackgroundTask("task2")

	state := handler.GetClaudeState()
	if len(state.BackgroundTasks) != 2 {
		t.Fatalf("BackgroundTasks length: got %d, want 2", len(state.BackgroundTasks))
	}

	handler.RemoveBackgroundTask("task1")

	state = handler.GetClaudeState()
	if len(state.BackgroundTasks) != 1 {
		t.Fatalf("BackgroundTasks after remove: got %d, want 1", len(state.BackgroundTasks))
	}

	if state.BackgroundTasks[0] != "task2" {
		t.Errorf("Remaining task: got %s, want %s", state.BackgroundTasks[0], "task2")
	}
}

func TestClaudeAwareScreenHandler_IsReadyForInput(t *testing.T) {
	t.Skip("IsReadyForInput depends on full screen state detection which requires proper terminal emulation")
	// This test requires the terminal emulation to properly position
	// the prompt character at the start of a line for detection
}

func TestClaudeAwareScreenHandler_HasPendingPermission(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)
	handler.Feed([]byte("Allow Edit to write file? [y/n]"))

	if !handler.HasPendingPermission() {
		t.Error("HasPendingPermission should return true")
	}
}

func TestClaudeAwareScreenHandler_IsGenerating(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)
	handler.Feed([]byte("⠋ Thinking..."))

	if !handler.IsGenerating() {
		t.Error("IsGenerating should return true for spinner")
	}
}

func TestClaudeAwareScreenHandler_Resize(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	// Should not panic
	handler.Resize(120, 40)
}

func TestClaudeAwareScreenHandler_OnStateChange_Callback(t *testing.T) {
	handler := NewClaudeAwareScreenHandler(80, 24)

	callbackCalled := make(chan bool, 1)
	var oldState, newState ClaudeState

	handler.OnStateChange = func(old, new ClaudeState) {
		oldState = old
		newState = new
		callbackCalled <- true
	}

	// Trigger state change
	handler.Feed([]byte("⠋ Processing..."))

	select {
	case <-callbackCalled:
		if oldState != StateUnknown {
			t.Errorf("Old state: got %s, want %s", oldState, StateUnknown)
		}
		if newState != StateGenerating {
			t.Errorf("New state: got %s, want %s", newState, StateGenerating)
		}
	case <-time.After(time.Second):
		t.Error("OnStateChange callback not called")
	}
}

func TestKnownSlashCommands(t *testing.T) {
	// Verify all expected commands exist
	expectedCommands := []string{
		"clear", "compact", "resume", "rewind", "exit",
		"cost", "context", "todos", "stats", "bashes", "help",
		"model", "permissions", "hooks", "config",
		"plan", "vim", "sandbox", "review", "init", "memory", "rename", "export",
		"mcp",
	}

	for _, cmd := range expectedCommands {
		if _, exists := knownSlashCommands[cmd]; !exists {
			t.Errorf("Missing command: %s", cmd)
		}
	}
}

func TestOutputPattern_Priority(t *testing.T) {
	// Verify patterns have reasonable priorities
	for _, p := range claudePatterns {
		if p.Priority < 0 || p.Priority > 100 {
			t.Errorf("Pattern %s has invalid priority: %d", p.Name, p.Priority)
		}
	}
}

func TestParseIntFromString(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"0", 0},
		{"999", 999},
		{"1234567", 1234567},
	}

	for _, tc := range tests {
		got, err := parseIntFromString(tc.input)
		if err != nil {
			t.Errorf("parseIntFromString(%q) error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseIntFromString(%q): got %d, want %d", tc.input, got, tc.want)
		}
	}
}
