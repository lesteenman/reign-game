package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// TestDailyRoutesAreRegistered is a wiring smoke test. It does not
// exercise the handlers' real behavior (those are covered in
// internal/handler tests) — it confirms the chi router maps the
// expected paths to handlers that return non-404.
func TestDailyRoutesAreRegistered(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"GET daily by date", http.MethodGet, "/api/daily/2026-05-02"},
		{"POST daily result", http.MethodPost, "/api/daily/2026-05-02/result"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange — newRouter only mounts the per-repo route groups
			// (including /daily) when repo != nil, so the test must pass
			// a non-nil repository. The DynamoDB client is nil; any
			// handler that actually issues a DDB call panics, which the
			// deferred recover() below absorbs. The wiring assertion
			// (status != 404) only requires the chi mux to resolve.
			repo := repository.NewPuzzleRepository(nil, "test")
			r := newRouter(repo, nil)
			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			defer func() {
				// Recover from any nil-repo panic; the assertion below
				// only verifies the router reached past 404.
				_ = recover()
			}()
			r.ServeHTTP(rec, req)

			// Assert — anything other than 404 means routing worked.
			// Auth middleware may write 401 (no cookie/no header), date
			// validation may write 400, etc. — all of those are fine.
			if rec.Code == http.StatusNotFound {
				t.Fatalf("expected route to be registered, got 404 for %s %s", tc.method, tc.path)
			}
		})
	}
}
