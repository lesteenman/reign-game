package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	packsvc "github.com/eriksteenman/reign-game/backend/internal/service/pack"
)

// stubPackService implements handler.PackService for HTTP-boundary tests.
type stubPackService struct {
	createErr     error
	updateErr     error
	deleteErr     error
	getView       *packsvc.PackView
	getErr        error
	listViews     []packsvc.PackView
	listErr       error
	candidates    []packsvc.Candidate
	candidatesErr error
}

func (s *stubPackService) Create(_ context.Context, _ *packsvc.CreateInput) error {
	return s.createErr
}
func (s *stubPackService) Update(_ context.Context, _ string, _ packsvc.UpdateInput) error {
	return s.updateErr
}
func (s *stubPackService) Delete(_ context.Context, _ string) error { return s.deleteErr }
func (s *stubPackService) Get(_ context.Context, slug string) (*packsvc.PackView, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getView != nil {
		return s.getView, nil
	}
	return &packsvc.PackView{Slug: slug, Name: "P", Size: 7, Mode: "standard"}, nil
}
func (s *stubPackService) List(_ context.Context) ([]packsvc.PackView, error) {
	return s.listViews, s.listErr
}
func (s *stubPackService) ListCandidates(_ context.Context, _ int, _ string) ([]packsvc.Candidate, error) {
	return s.candidates, s.candidatesErr
}

func validPackCreateBody() string {
	return `{"slug":"starter-pack","name":"Starter","size":7,"mode":"standard","puzzleIds":["id-1"],"published":false}`
}

func TestCreatePackHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
		wantError  string
	}{
		{name: "valid create returns 201", body: validPackCreateBody(), wantStatus: http.StatusCreated},
		{name: "duplicate slug returns 409", body: validPackCreateBody(), createErr: packsvc.ErrAlreadyExists, wantStatus: http.StatusConflict, wantError: "conflict"},
		{name: "invalid slug returns 400", body: validPackCreateBody(), createErr: packsvc.ErrInvalidSlug, wantStatus: http.StatusBadRequest, wantError: "invalid_params"},
		{name: "invalid combo returns 400", body: validPackCreateBody(), createErr: packsvc.ErrInvalidCombo, wantStatus: http.StatusBadRequest, wantError: "invalid_params"},
		{name: "non-verdict-up puzzle returns 422", body: validPackCreateBody(), createErr: &packsvc.InvalidPuzzleError{PuzzleID: "id-1", Reason: "not verdict-up"}, wantStatus: http.StatusUnprocessableEntity, wantError: "unprocessable"},
		{name: "publish empty returns 422", body: validPackCreateBody(), createErr: packsvc.ErrEmptyPublish, wantStatus: http.StatusUnprocessableEntity, wantError: "unprocessable"},
		{name: "malformed body returns 400", body: `{bad`, wantStatus: http.StatusBadRequest, wantError: "invalid_params"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubPackService{createErr: tt.createErr}
			r := chi.NewRouter()
			r.Post("/admin/packs", handler.CreatePackHandler(svc))
			req := httptest.NewRequest(http.MethodPost, "/admin/packs", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantError != "" {
				var errResp struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("parse error response: %v", err)
				}
				if errResp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", errResp.Error, tt.wantError)
				}
			}
		})
	}
}

func TestUpdatePackHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		updateErr  error
		wantStatus int
		wantError  string
	}{
		{name: "valid update returns 200", body: `{"name":"New","puzzleIds":["id-1"],"published":true}`, wantStatus: http.StatusOK},
		{name: "not found returns 404", body: `{"name":"x"}`, updateErr: packsvc.ErrNotFound, wantStatus: http.StatusNotFound, wantError: "not_found"},
		{name: "invalid puzzle returns 422", body: `{"name":"x","puzzleIds":["bad"]}`, updateErr: &packsvc.InvalidPuzzleError{PuzzleID: "bad", Reason: "not found in this size+mode"}, wantStatus: http.StatusUnprocessableEntity, wantError: "unprocessable"},
		{name: "publish empty returns 422", body: `{"name":"x","published":true}`, updateErr: packsvc.ErrEmptyPublish, wantStatus: http.StatusUnprocessableEntity, wantError: "unprocessable"},
		{name: "malformed body returns 400", body: `{bad`, wantStatus: http.StatusBadRequest, wantError: "invalid_params"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubPackService{updateErr: tt.updateErr}
			r := chi.NewRouter()
			r.Put("/admin/packs/{slug}", handler.UpdatePackHandler(svc))
			req := httptest.NewRequest(http.MethodPut, "/admin/packs/starter-pack", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantError != "" {
				var errResp struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("parse error response: %v", err)
				}
				if errResp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", errResp.Error, tt.wantError)
				}
			}
		})
	}
}

func TestGetPackHandler(t *testing.T) {
	tests := []struct {
		name       string
		getErr     error
		wantStatus int
	}{
		{name: "found returns 200", wantStatus: http.StatusOK},
		{name: "not found returns 404", getErr: packsvc.ErrNotFound, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubPackService{getErr: tt.getErr}
			r := chi.NewRouter()
			r.Get("/admin/packs/{slug}", handler.GetPackHandler(svc))
			req := httptest.NewRequest(http.MethodGet, "/admin/packs/starter-pack", http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestListPacksHandler(t *testing.T) {
	// Arrange
	svc := &stubPackService{listViews: []packsvc.PackView{{Slug: "a"}, {Slug: "b"}}}
	r := chi.NewRouter()
	r.Get("/admin/packs", handler.ListPacksHandler(svc))
	req := httptest.NewRequest(http.MethodGet, "/admin/packs", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var views []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(views) != 2 {
		t.Errorf("len = %d, want 2", len(views))
	}
}

func TestDeletePackHandler(t *testing.T) {
	tests := []struct {
		name       string
		deleteErr  error
		wantStatus int
	}{
		{name: "deletes returns 204", wantStatus: http.StatusNoContent},
		{name: "not found returns 404", deleteErr: packsvc.ErrNotFound, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubPackService{deleteErr: tt.deleteErr}
			r := chi.NewRouter()
			r.Delete("/admin/packs/{slug}", handler.DeletePackHandler(svc))
			req := httptest.NewRequest(http.MethodDelete, "/admin/packs/starter-pack", http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestListPackCandidatesHandler(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		candidates []packsvc.Candidate
		wantStatus int
	}{
		{name: "valid returns 200", query: "?size=7&mode=standard", candidates: []packsvc.Candidate{{ID: "id-1", RegionMap: [][]int{{0}}}}, wantStatus: http.StatusOK},
		{name: "bad size returns 400", query: "?size=abc&mode=standard", wantStatus: http.StatusBadRequest},
		{name: "bad mode returns 400", query: "?size=7&mode=triple", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubPackService{candidates: tt.candidates}
			r := chi.NewRouter()
			r.Get("/admin/packs/candidates", handler.ListPackCandidatesHandler(svc))
			req := httptest.NewRequest(http.MethodGet, "/admin/packs/candidates"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				var views []map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
					t.Fatalf("parse: %v", err)
				}
				if len(views) != 1 || views[0]["puzzleId"] != "id-1" {
					t.Errorf("views = %v", views)
				}
				if _, hasSolution := views[0]["solution"]; hasSolution {
					t.Error("candidate must not include solution")
				}
			}
		})
	}
}

// --- Auth matrix: one unauthenticated + one non-admin test per route ---

func TestListPacksHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Get("/packs", handler.ListPacksHandler(&stubPackService{}))
			}, roleForState(tc.state))
			req := newAdminRequest(tc.state, http.MethodGet, "/api/admin/packs", nil)
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestGetPackHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Get("/packs/{slug}", handler.GetPackHandler(&stubPackService{}))
			}, roleForState(tc.state))
			req := newAdminRequest(tc.state, http.MethodGet, "/api/admin/packs/starter-pack", nil)
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestCreatePackHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Post("/packs", handler.CreatePackHandler(&stubPackService{}))
			}, roleForState(tc.state))
			wantCode := tc.wantCode
			if wantCode == http.StatusOK {
				wantCode = http.StatusCreated
			}
			req := newAdminRequest(tc.state, http.MethodPost, "/api/admin/packs", strings.NewReader(validPackCreateBody()))
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != wantCode {
				t.Errorf("status = %d, want %d", rec.Code, wantCode)
			}
		})
	}
}

func TestUpdatePackHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Put("/packs/{slug}", handler.UpdatePackHandler(&stubPackService{}))
			}, roleForState(tc.state))
			req := newAdminRequest(tc.state, http.MethodPut, "/api/admin/packs/starter-pack", strings.NewReader(`{"name":"x","puzzleIds":["id-1"]}`))
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestDeletePackHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Delete("/packs/{slug}", handler.DeletePackHandler(&stubPackService{}))
			}, roleForState(tc.state))
			wantCode := tc.wantCode
			if wantCode == http.StatusOK {
				wantCode = http.StatusNoContent
			}
			req := newAdminRequest(tc.state, http.MethodDelete, "/api/admin/packs/starter-pack", nil)
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != wantCode {
				t.Errorf("status = %d, want %d", rec.Code, wantCode)
			}
		})
	}
}

func TestListPackCandidatesHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Get("/packs/candidates", handler.ListPackCandidatesHandler(&stubPackService{}))
			}, roleForState(tc.state))
			req := newAdminRequest(tc.state, http.MethodGet, "/api/admin/packs/candidates?size=7&mode=standard", nil)
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}
