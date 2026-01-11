package validator

import (
	"testing"
)

func TestValidator_Required(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"non-empty value", "hello", false},
		{"empty value", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.Required("field", tt.value)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("Required() hasErrors = %v, want %v", v.HasErrors(), tt.wantErr)
			}
		})
	}
}

func TestValidator_UUID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", false},
		{"invalid UUID", "not-a-uuid", true},
		{"empty UUID", "", false}, // Empty is valid, use Required for mandatory
		{"short UUID", "550e8400", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.UUID("field", tt.value)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("UUID() hasErrors = %v, want %v", v.HasErrors(), tt.wantErr)
			}
		})
	}
}

func TestValidator_OneOf(t *testing.T) {
	allowed := []string{"claude", "terminal"}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid value claude", "claude", false},
		{"valid value terminal", "terminal", false},
		{"invalid value", "bash", true},
		{"empty value", "", false}, // Empty is valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.OneOf("field", tt.value, allowed)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("OneOf() hasErrors = %v, want %v", v.HasErrors(), tt.wantErr)
			}
		})
	}
}

func TestValidator_AbsolutePath(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"absolute path", "/root/project", false},
		{"relative path", "project/file", true},
		{"empty path", "", false}, // Empty is valid
		{"dot path", "./file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.AbsolutePath("field", tt.value)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("AbsolutePath() hasErrors = %v, want %v", v.HasErrors(), tt.wantErr)
			}
		})
	}
}

func TestValidator_NoPathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"normal path", "/root/project", false},
		{"path traversal", "/root/../etc/passwd", true},
		{"double dots", "/root/project/../file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.NoPathTraversal("field", tt.value)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("NoPathTraversal() hasErrors = %v, want %v", v.HasErrors(), tt.wantErr)
			}
		})
	}
}

func TestValidator_Range(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		min     int
		max     int
		wantErr bool
	}{
		{"in range", 50, 1, 100, false},
		{"at min", 1, 1, 100, false},
		{"at max", 100, 1, 100, false},
		{"below min", 0, 1, 100, true},
		{"above max", 101, 1, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			v.Range("field", tt.value, tt.min, tt.max)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("Range() hasErrors = %v, want %v", v.HasErrors(), tt.wantErr)
			}
		})
	}
}

func TestValidator_ChainedValidation(t *testing.T) {
	v := New()
	v.Required("name", "test").
		MaxLength("name", "test", 100).
		Required("path", "/root").
		AbsolutePath("path", "/root").
		NoPathTraversal("path", "/root")

	if v.HasErrors() {
		t.Errorf("Chained validation failed unexpectedly: %v", v.Errors())
	}
}

func TestValidator_MultipleErrors(t *testing.T) {
	v := New()
	v.Required("field1", "")
	v.Required("field2", "")
	v.UUID("field3", "invalid")

	if !v.HasErrors() {
		t.Error("Expected errors but got none")
	}

	if len(v.Errors()) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(v.Errors()))
	}
}

func TestValidateTerminalConfig(t *testing.T) {
	tests := []struct {
		name    string
		req     TerminalConfigRequest
		wantErr bool
	}{
		{
			name: "valid config",
			req: TerminalConfigRequest{
				WorkDir: "/tmp",
				Type:    "claude",
			},
			wantErr: false,
		},
		{
			name: "missing work_dir",
			req: TerminalConfigRequest{
				Type: "claude",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			req: TerminalConfigRequest{
				WorkDir: "/tmp",
				Type:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "non-existent dir",
			req: TerminalConfigRequest{
				WorkDir: "/nonexistent/path/12345",
				Type:    "claude",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()
			ValidateTerminalConfig(&tt.req, v)
			if v.HasErrors() != tt.wantErr {
				t.Errorf("ValidateTerminalConfig() hasErrors = %v, want %v, errors: %v",
					v.HasErrors(), tt.wantErr, v.Errors())
			}
		})
	}
}
