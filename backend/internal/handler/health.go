package handler

import (
	"log"
	"net/http"
)

// HealthCheck handles GET /health and returns a JSON status response.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("health check write failed: %v", err)
	}
}
