package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// APIResponse estructura para respuestas de la API
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *APIError       `json:"error,omitempty"`
}

// APIError estructura de error de la API
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MakeRequest crea un request HTTP para testing
func MakeRequest(t *testing.T, method, path string, body string) *http.Request {
	t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	return req
}

// MakeRequestWithAuth crea un request con autenticación
func MakeRequestWithAuth(t *testing.T, method, path, body, username, password string) *http.Request {
	t.Helper()

	req := MakeRequest(t, method, path, body)
	req.SetBasicAuth(username, password)

	return req
}

// ExecuteRequest ejecuta un request y devuelve el recorder
func ExecuteRequest(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return rr
}

// ParseResponse parsea una respuesta JSON
func ParseResponse(t *testing.T, rr *httptest.ResponseRecorder) *APIResponse {
	t.Helper()

	var response APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Error parsing response: %v", err)
	}

	return &response
}

// AssertStatus verifica el código de estado HTTP
func AssertStatus(t *testing.T, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("Status code: got %d, want %d", got, want)
	}
}

// AssertSuccess verifica que la respuesta sea exitosa
func AssertSuccess(t *testing.T, response *APIResponse) {
	t.Helper()

	if !response.Success {
		t.Errorf("Expected success=true, got false")
		if response.Error != nil {
			t.Errorf("Error: %s - %s", response.Error.Code, response.Error.Message)
		}
	}
}

// AssertError verifica que la respuesta sea un error
func AssertError(t *testing.T, response *APIResponse, expectedCode string) {
	t.Helper()

	if response.Success {
		t.Error("Expected success=false, got true")
		return
	}

	if response.Error == nil {
		t.Error("Expected error, got nil")
		return
	}

	if response.Error.Code != expectedCode {
		t.Errorf("Error code: got %s, want %s", response.Error.Code, expectedCode)
	}
}

// AssertContains verifica que una string contenga un substring
func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Errorf("Expected %q to contain %q", haystack, needle)
	}
}

// AssertNotContains verifica que una string NO contenga un substring
func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if strings.Contains(haystack, needle) {
		t.Errorf("Expected %q to NOT contain %q", haystack, needle)
	}
}

// AssertEqual verifica igualdad de valores
func AssertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// AssertNotEqual verifica desigualdad de valores
func AssertNotEqual[T comparable](t *testing.T, got, notWant T) {
	t.Helper()

	if got == notWant {
		t.Errorf("got %v, did not want %v", got, notWant)
	}
}

// AssertNil verifica que un valor sea nil
func AssertNil(t *testing.T, got interface{}) {
	t.Helper()

	if got != nil {
		t.Errorf("Expected nil, got %v", got)
	}
}

// AssertNotNil verifica que un valor no sea nil
func AssertNotNil(t *testing.T, got interface{}) {
	t.Helper()

	if got == nil {
		t.Error("Expected non-nil value, got nil")
	}
}

// AssertTrue verifica que una condición sea verdadera
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()

	if !condition {
		t.Errorf("Expected true: %s", msg)
	}
}

// AssertFalse verifica que una condición sea falsa
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()

	if condition {
		t.Errorf("Expected false: %s", msg)
	}
}
