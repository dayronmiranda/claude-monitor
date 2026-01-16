package handlers

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	apierrors "claude-monitor/pkg/errors"
)

// URLParam obtiene un parámetro de URL de Chi
// Es un wrapper de chi.URLParam para uso en handlers
func URLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// URLParamDecoded obtiene un parámetro de URL decodificado
// Útil para paths que pueden contener caracteres especiales
func URLParamDecoded(r *http.Request, key string) string {
	param := chi.URLParam(r, key)
	decoded, err := url.PathUnescape(param)
	if err != nil {
		return param
	}
	return decoded
}

// APIMeta metadatos de respuesta
type APIMeta struct {
	Total  int `json:"total,omitempty"`
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// Alias para compatibilidad
type APIResponse = apierrors.APIResponse
type APIError = apierrors.APIError

// ErrorResponse respuesta de error (mantiene compatibilidad)
func ErrorResponse(err string) apierrors.APIResponse {
	return apierrors.APIResponse{
		Success: false,
		Error:   &apierrors.APIError{Code: apierrors.ErrCodeInternal, Message: err},
	}
}

// SuccessResponse respuesta exitosa
func SuccessResponse(data interface{}) apierrors.APIResponse {
	return apierrors.APIResponse{
		Success: true,
		Data:    data,
	}
}

// SuccessWithMeta respuesta exitosa con metadatos
func SuccessWithMeta(data interface{}, meta *APIMeta) map[string]interface{} {
	return map[string]interface{}{
		"success": true,
		"data":    data,
		"meta":    meta,
	}
}

// Helpers para escribir respuestas

// WriteSuccess escribe una respuesta de éxito
func WriteSuccess(w http.ResponseWriter, data any) {
	apierrors.WriteSuccess(w, data)
}

// WriteCreated escribe una respuesta de creación
func WriteCreated(w http.ResponseWriter, data any) {
	apierrors.WriteCreated(w, data)
}

// WriteError escribe un error tipado
func WriteError(w http.ResponseWriter, err *apierrors.APIError) {
	apierrors.WriteError(w, err)
}

// WriteErrorMsg escribe un error con mensaje simple
func WriteErrorMsg(w http.ResponseWriter, code apierrors.ErrorCode, message string) {
	apierrors.WriteError(w, apierrors.New(code, message))
}

// WriteNotFound escribe un error 404
func WriteNotFound(w http.ResponseWriter, resource string) {
	apierrors.WriteError(w, apierrors.NotFound(resource))
}

// WriteBadRequest escribe un error 400
func WriteBadRequest(w http.ResponseWriter, message string) {
	apierrors.WriteError(w, apierrors.New(apierrors.ErrCodeBadRequest, message))
}

// WriteInternalError escribe un error 500
func WriteInternalError(w http.ResponseWriter, message string) {
	apierrors.WriteError(w, apierrors.New(apierrors.ErrCodeInternal, message))
}

// WriteConflict escribe un error 409
func WriteConflict(w http.ResponseWriter, message string) {
	apierrors.WriteError(w, apierrors.Conflict(message))
}
