package httperr_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
)

func TestWriteError_SetsStatusAndContentTypeAndBody(t *testing.T) {
	// Arrange
	rec := httptest.NewRecorder()

	// Act
	httperr.WriteError(rec, 418, "teapot", "I'm a teapot")

	// Assert
	if got, want := rec.Code, 418; got != want {
		t.Errorf("status code = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	var decoded map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not valid JSON: %v (body=%q)", err, rec.Body.String())
	}
	if got, want := decoded["error"], "teapot"; got != want {
		t.Errorf("body.error = %q, want %q", got, want)
	}
	if got, want := decoded["message"], "I'm a teapot"; got != want {
		t.Errorf("body.message = %q, want %q", got, want)
	}
}
