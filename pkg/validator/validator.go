package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	apierrors "claude-monitor/pkg/errors"
)

// ValidationError error de validación con detalles
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors lista de errores de validación
type ValidationErrors []ValidationError

// Error implementa la interfaz error
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s - %s", e[0].Field, e[0].Message)
}

// ToAPIError convierte a APIError
func (e ValidationErrors) ToAPIError() *apierrors.APIError {
	return apierrors.New(apierrors.ErrCodeInvalidInput, "validación fallida").WithDetails(e)
}

// Validator validador de requests
type Validator struct {
	errors ValidationErrors
}

// New crea un nuevo validador
func New() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// HasErrors verifica si hay errores
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors retorna los errores
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// AddError añade un error de validación
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, ValidationError{Field: field, Message: message})
}

// Required valida que un campo no esté vacío
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "es requerido")
	}
	return v
}

// MinLength valida longitud mínima
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.AddError(field, fmt.Sprintf("debe tener al menos %d caracteres", min))
	}
	return v
}

// MaxLength valida longitud máxima
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.AddError(field, fmt.Sprintf("debe tener máximo %d caracteres", max))
	}
	return v
}

// UUID valida formato UUID
func (v *Validator) UUID(field, value string) *Validator {
	if value == "" {
		return v // Vacío se valida con Required
	}
	pattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	if matched, _ := regexp.MatchString(pattern, value); !matched {
		v.AddError(field, "formato UUID inválido")
	}
	return v
}

// OneOf valida que el valor esté en una lista
func (v *Validator) OneOf(field, value string, allowed []string) *Validator {
	if value == "" {
		return v
	}
	for _, a := range allowed {
		if value == a {
			return v
		}
	}
	v.AddError(field, fmt.Sprintf("debe ser uno de: %s", strings.Join(allowed, ", ")))
	return v
}

// DirExists valida que el directorio exista
func (v *Validator) DirExists(field, path string) *Validator {
	if path == "" {
		return v
	}
	info, err := os.Stat(path)
	if err != nil {
		v.AddError(field, "directorio no existe")
		return v
	}
	if !info.IsDir() {
		v.AddError(field, "no es un directorio")
	}
	return v
}

// AbsolutePath valida que sea un path absoluto
func (v *Validator) AbsolutePath(field, path string) *Validator {
	if path == "" {
		return v
	}
	if !filepath.IsAbs(path) {
		v.AddError(field, "debe ser un path absoluto")
	}
	return v
}

// NoPathTraversal valida que no haya path traversal
func (v *Validator) NoPathTraversal(field, path string) *Validator {
	if strings.Contains(path, "..") {
		v.AddError(field, "path traversal no permitido")
	}
	return v
}

// Positive valida que un número sea positivo
func (v *Validator) Positive(field string, value int) *Validator {
	if value <= 0 {
		v.AddError(field, "debe ser positivo")
	}
	return v
}

// Range valida que un número esté en un rango
func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.AddError(field, fmt.Sprintf("debe estar entre %d y %d", min, max))
	}
	return v
}

// DecodeAndValidate decodifica JSON y valida
func DecodeAndValidate[T any](r *http.Request, validate func(*T, *Validator)) (*T, error) {
	var req T

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, apierrors.New(apierrors.ErrCodeBadRequest, "JSON inválido: "+err.Error())
	}

	v := New()
	validate(&req, v)

	if v.HasErrors() {
		return nil, v.Errors().ToAPIError()
	}

	return &req, nil
}

// TerminalConfig validación para config de terminal
type TerminalConfigRequest struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name,omitempty"`
	WorkDir         string   `json:"work_dir"`
	Type            string   `json:"type,omitempty"`
	Model           string   `json:"model,omitempty"`
	Resume          bool     `json:"resume,omitempty"`
	Continue        bool     `json:"continue,omitempty"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
}

// ValidateTerminalConfig valida configuración de terminal
func ValidateTerminalConfig(req *TerminalConfigRequest, v *Validator) {
	v.Required("work_dir", req.WorkDir)
	v.AbsolutePath("work_dir", req.WorkDir)
	v.NoPathTraversal("work_dir", req.WorkDir)
	v.DirExists("work_dir", req.WorkDir)

	if req.Name != "" {
		v.MaxLength("name", req.Name, 100)
	}

	if req.Type != "" {
		v.OneOf("type", req.Type, []string{"claude", "terminal"})
	}

	if req.ID != "" {
		v.UUID("id", req.ID)
	}
}

// SessionIDsRequest request para eliminar múltiples sesiones
type SessionIDsRequest struct {
	SessionIDs []string `json:"session_ids"`
}

// ValidateSessionIDs valida lista de session IDs
func ValidateSessionIDs(req *SessionIDsRequest, v *Validator) {
	if len(req.SessionIDs) == 0 {
		v.AddError("session_ids", "debe contener al menos un ID")
		return
	}

	for i, id := range req.SessionIDs {
		v.UUID(fmt.Sprintf("session_ids[%d]", i), id)
	}
}

// ResizeRequest request para redimensionar terminal
type ResizeRequest struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// ValidateResize valida request de resize
func ValidateResize(req *ResizeRequest, v *Validator) {
	v.Range("rows", int(req.Rows), 1, 500)
	v.Range("cols", int(req.Cols), 1, 500)
}
