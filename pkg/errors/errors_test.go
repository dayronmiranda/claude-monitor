package errors

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeNotFound, "test not found")

	if err.Code != ErrCodeNotFound {
		t.Errorf("Code: got %s, want %s", err.Code, ErrCodeNotFound)
	}

	if err.Message != "test not found" {
		t.Errorf("Message: got %s, want %s", err.Message, "test not found")
	}
}

func TestAPIError_Error(t *testing.T) {
	err := New(ErrCodeInternal, "internal error")

	if err.Error() != "internal error" {
		t.Errorf("Error(): got %s, want %s", err.Error(), "internal error")
	}
}

func TestAPIError_WithDetails(t *testing.T) {
	details := map[string]string{"field": "name", "reason": "required"}
	err := New(ErrCodeInvalidInput, "validation failed").WithDetails(details)

	if err.Details == nil {
		t.Fatal("Details should not be nil")
	}

	d, ok := err.Details.(map[string]string)
	if !ok {
		t.Fatal("Details should be map[string]string")
	}

	if d["field"] != "name" {
		t.Errorf("Details field: got %s, want %s", d["field"], "name")
	}
}

func TestAPIError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		{"not found", ErrCodeNotFound, http.StatusNotFound},
		{"unauthorized", ErrCodeUnauthorized, http.StatusUnauthorized},
		{"forbidden", ErrCodeForbidden, http.StatusForbidden},
		{"invalid input", ErrCodeInvalidInput, http.StatusBadRequest},
		{"bad request", ErrCodeBadRequest, http.StatusBadRequest},
		{"path not allowed", ErrCodePathNotAllowed, http.StatusBadRequest},
		{"conflict", ErrCodeConflict, http.StatusConflict},
		{"too many requests", ErrCodeTooManyRequests, http.StatusTooManyRequests},
		{"internal", ErrCodeInternal, http.StatusInternalServerError},
		{"unknown", ErrorCode("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, "test")
			if err.HTTPStatus() != tt.expected {
				t.Errorf("HTTPStatus(): got %d, want %d", err.HTTPStatus(), tt.expected)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("user")

	if err.Code != ErrCodeNotFound {
		t.Errorf("Code: got %s, want %s", err.Code, ErrCodeNotFound)
	}

	if err.Message != "user no encontrado" {
		t.Errorf("Message: got %s, want %s", err.Message, "user no encontrado")
	}
}

func TestInvalidInput(t *testing.T) {
	err := InvalidInput("email", "must be valid email")

	if err.Code != ErrCodeInvalidInput {
		t.Errorf("Code: got %s, want %s", err.Code, ErrCodeInvalidInput)
	}

	details, ok := err.Details.(map[string]string)
	if !ok {
		t.Fatal("Details should be map[string]string")
	}

	if details["field"] != "email" {
		t.Errorf("Details field: got %s, want %s", details["field"], "email")
	}

	if details["reason"] != "must be valid email" {
		t.Errorf("Details reason: got %s, want %s", details["reason"], "must be valid email")
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("resource already exists")

	if err.Code != ErrCodeConflict {
		t.Errorf("Code: got %s, want %s", err.Code, ErrCodeConflict)
	}

	if err.Message != "resource already exists" {
		t.Errorf("Message: got %s, want %s", err.Message, "resource already exists")
	}
}

func TestTooManyRequests(t *testing.T) {
	err := TooManyRequests("rate limit exceeded")

	if err.Code != ErrCodeTooManyRequests {
		t.Errorf("Code: got %s, want %s", err.Code, ErrCodeTooManyRequests)
	}
}

func TestWrap(t *testing.T) {
	original := errors.New("original error")
	wrapped := Wrap(original, "wrapped context")

	if wrapped == nil {
		t.Fatal("Wrap should not return nil for non-nil error")
	}

	if wrapped.Error() != "wrapped context: original error" {
		t.Errorf("Error(): got %s, want %s", wrapped.Error(), "wrapped context: original error")
	}

	// Test nil case
	nilWrapped := Wrap(nil, "context")
	if nilWrapped != nil {
		t.Error("Wrap(nil, ...) should return nil")
	}
}

func TestWrapWithCode(t *testing.T) {
	original := errors.New("db error")
	apiErr := WrapWithCode(original, ErrCodeInternal, "database operation failed")

	if apiErr == nil {
		t.Fatal("WrapWithCode should not return nil for non-nil error")
	}

	if apiErr.Code != ErrCodeInternal {
		t.Errorf("Code: got %s, want %s", apiErr.Code, ErrCodeInternal)
	}

	if apiErr.Message != "database operation failed" {
		t.Errorf("Message: got %s, want %s", apiErr.Message, "database operation failed")
	}

	details, ok := apiErr.Details.(map[string]interface{})
	if !ok {
		t.Fatal("Details should be map[string]interface{}")
	}

	if details["cause"] != "db error" {
		t.Errorf("Details cause: got %s, want %s", details["cause"], "db error")
	}

	// Test nil case
	nilApiErr := WrapWithCode(nil, ErrCodeInternal, "message")
	if nilApiErr != nil {
		t.Error("WrapWithCode(nil, ...) should return nil")
	}
}

func TestIsNotFound(t *testing.T) {
	notFoundErr := New(ErrCodeNotFound, "not found")
	otherErr := New(ErrCodeInternal, "internal")
	regularErr := errors.New("regular error")

	if !IsNotFound(notFoundErr) {
		t.Error("IsNotFound should return true for NOT_FOUND error")
	}

	if IsNotFound(otherErr) {
		t.Error("IsNotFound should return false for other API errors")
	}

	if IsNotFound(regularErr) {
		t.Error("IsNotFound should return false for regular errors")
	}
}

func TestIsUnauthorized(t *testing.T) {
	unauthorizedErr := New(ErrCodeUnauthorized, "unauthorized")
	otherErr := New(ErrCodeForbidden, "forbidden")

	if !IsUnauthorized(unauthorizedErr) {
		t.Error("IsUnauthorized should return true for UNAUTHORIZED error")
	}

	if IsUnauthorized(otherErr) {
		t.Error("IsUnauthorized should return false for other errors")
	}
}

func TestIsBadRequest(t *testing.T) {
	badRequestErr := New(ErrCodeBadRequest, "bad request")
	invalidInputErr := New(ErrCodeInvalidInput, "invalid input")
	otherErr := New(ErrCodeNotFound, "not found")

	if !IsBadRequest(badRequestErr) {
		t.Error("IsBadRequest should return true for BAD_REQUEST error")
	}

	if !IsBadRequest(invalidInputErr) {
		t.Error("IsBadRequest should return true for INVALID_INPUT error")
	}

	if IsBadRequest(otherErr) {
		t.Error("IsBadRequest should return false for other errors")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Status: got %d, want %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %s, want %s", w.Header().Get("Content-Type"), "application/json")
	}
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"result": "ok"}

	WriteSuccess(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("Status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWriteCreated(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"id": "123"}

	WriteCreated(w, data)

	if w.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	err := New(ErrCodeNotFound, "not found")

	WriteError(w, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestWriteErrorFromError(t *testing.T) {
	t.Run("with API error", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := New(ErrCodeForbidden, "forbidden")

		WriteErrorFromError(w, err)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status: got %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("with regular error", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("some error")

		WriteErrorFromError(w, err)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status: got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestWrappedError_Unwrap(t *testing.T) {
	original := errors.New("original")
	wrapped := Wrap(original, "context")

	unwrapped := errors.Unwrap(wrapped)
	if unwrapped != original {
		t.Error("Unwrap should return the original error")
	}
}
