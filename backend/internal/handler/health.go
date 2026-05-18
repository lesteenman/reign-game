package handler

import (
	"log"
	"net/http"
)

// HealthCheck handles GET and HEAD /health.
//
// For GET it writes a JSON status body. For HEAD it returns the same
// headers and status code with no body (per RFC 9110 § 9.3.2). The
// frontend's PWA connectivity probe (useConnectivity) issues HEAD so
// the body wouldn't be read anyway; explicit branching keeps the
// behaviour testable via httptest, which does NOT auto-suppress body
// writes on HEAD the way net/http's production ResponseWriter does.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}

	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("health check write failed: %v", err)
	}
}
