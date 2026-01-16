package errors

import (
	"encoding/json"
	"net/http"
)

// ErrorCode códigos de error de la API
type ErrorCode string

const (
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrCodeUnauthorized  ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden     ErrorCode = "FORBIDDEN"
	ErrCodeInternal      ErrorCode = "INTERNAL_ERROR"
	ErrCodeConflict      ErrorCode = "CONFLICT"
	ErrCodeBadRequest    ErrorCode = "BAD_REQUEST"
	ErrCodePathNotAllowed ErrorCode = "PATH_NOT_ALLOWED"
)

// APIError estructura de error para la API
type APIError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

// Error implementa la interfaz error
func (e *APIError) Error() string {
	return e.Message
}

// APIResponse respuesta estándar de la API
type APIResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// New crea un nuevo APIError
func New(code ErrorCode, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
	}
}

// WithDetails añade detalles al error
func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

// Errores predefinidos
var (
	ErrNotFound      = New(ErrCodeNotFound, "recurso no encontrado")
	ErrUnauthorized  = New(ErrCodeUnauthorized, "no autorizado")
	ErrForbidden     = New(ErrCodeForbidden, "acceso denegado")
	ErrInternal      = New(ErrCodeInternal, "error interno del servidor")
	ErrInvalidInput  = New(ErrCodeInvalidInput, "entrada inválida")
	ErrBadRequest    = New(ErrCodeBadRequest, "petición inválida")
	ErrPathNotAllowed = New(ErrCodePathNotAllowed, "path no permitido")
)

// HTTPStatus devuelve el código HTTP apropiado para el error
func (e *APIError) HTTPStatus() int {
	switch e.Code {
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeInvalidInput, ErrCodeBadRequest, ErrCodePathNotAllowed:
		return http.StatusBadRequest
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeTooManyRequests:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// WriteJSON escribe una respuesta JSON
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteSuccess escribe una respuesta de éxito
func WriteSuccess(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// WriteCreated escribe una respuesta de creación exitosa
func WriteCreated(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    data,
	})
}

// WriteError escribe una respuesta de error
func WriteError(w http.ResponseWriter, err *APIError) {
	WriteJSON(w, err.HTTPStatus(), APIResponse{
		Success: false,
		Error:   err,
	})
}

// WriteErrorFromError convierte un error estándar a APIError y lo escribe
func WriteErrorFromError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*APIError); ok {
		WriteError(w, apiErr)
		return
	}
	WriteError(w, New(ErrCodeInternal, err.Error()))
}

// NotFound crea un error de recurso no encontrado personalizado
func NotFound(resource string) *APIError {
	return New(ErrCodeNotFound, resource+" no encontrado")
}

// InvalidInput crea un error de entrada inválida con detalles
func InvalidInput(field, reason string) *APIError {
	return New(ErrCodeInvalidInput, "campo inválido: "+field).WithDetails(map[string]string{
		"field":  field,
		"reason": reason,
	})
}

// Conflict crea un error de conflicto
func Conflict(message string) *APIError {
	return New(ErrCodeConflict, message)
}

// Wrap envuelve un error con contexto adicional
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return &wrappedError{
		err:     err,
		message: message,
	}
}

// WrapWithCode envuelve un error con código API
func WrapWithCode(err error, code ErrorCode, message string) *APIError {
	if err == nil {
		return nil
	}
	return &APIError{
		Code:    code,
		Message: message,
		Details: map[string]interface{}{
			"cause": err.Error(),
		},
	}
}

// wrappedError implementa error wrapping
type wrappedError struct {
	err     error
	message string
}

func (w *wrappedError) Error() string {
	return w.message + ": " + w.err.Error()
}

func (w *wrappedError) Unwrap() error {
	return w.err
}

// IsNotFound verifica si es un error NotFound
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == ErrCodeNotFound
	}
	return false
}

// IsUnauthorized verifica si es un error Unauthorized
func IsUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == ErrCodeUnauthorized
	}
	return false
}

// IsBadRequest verifica si es un error BadRequest
func IsBadRequest(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == ErrCodeBadRequest || apiErr.Code == ErrCodeInvalidInput
	}
	return false
}

// TooManyRequests para rate limiting
const ErrCodeTooManyRequests ErrorCode = "TOO_MANY_REQUESTS"

// TooManyRequests crea un error de rate limiting
func TooManyRequests(message string) *APIError {
	return New(ErrCodeTooManyRequests, message)
}
