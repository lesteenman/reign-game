// Package httperr centralises the JSON error-response shape used across
// every HTTP handler and middleware. Both the handler package and the
// auth middleware need the same `{"error":"<code>","message":"<msg>"}`
// body with Content-Type application/json, so route through WriteError
// to keep the client contract consistent.
package httperr

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteError emits a JSON error response with the standard shape:
//
//	{"error":"<code>","message":"<message>"}
//
// It sets Content-Type: application/json and the status code. Body
// write failures are logged and otherwise swallowed — the status line
// has already been sent and there is no recovery path.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]string{"error": code, "message": message}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("WARN: httperr: failed to write error response: %v", err)
	}
}
