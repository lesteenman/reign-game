package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// stubConfigService implements handler.ConfigService for HTTP-boundary tests.
type stubConfigService struct {
	updateErr error
	createErr error
}

func (s *stubConfigService) Update(_ context.Context, _ configsvc.UpdateInput) error {
	return s.updateErr
}

func (s *stubConfigService) Create(_ context.Context, _ configsvc.CreateInput) error {
	return s.createErr
}

// validConfigBody returns a valid config JSON body for tests.
func validConfigBody() string {
	return `{
		"threshold": 10,
		"enabled": true
	}`
}

// validCreateBody returns a valid config JSON body with size and mode for POST tests.
func validCreateBody() string {
	return `{
		"size": 7,
		"mode": "standard",
		"threshold": 10,
		"enabled": true
	}`
}

func TestUpdateConfigHandler(t *testing.T) {
	tests := []struct {
		name        string
		size        string
		mode        string
		body        string
		updateErr   error
		wantStatus  int
		wantError   string
		wantMessage string
	}{
		{
			name:       "valid update returns 200",
			size:       "7",
			mode:       "standard",
			body:       validConfigBody(),
			wantStatus: http.StatusOK,
		},
		{
			name:       "update with maxAttempts returns 200",
			size:       "7",
			mode:       "standard",
			body:       `{"threshold":10,"enabled":true,"maxAttempts":30}`,
			wantStatus: http.StatusOK,
		},
		{
			name:        "config not found returns 404",
			size:        "7",
			mode:        "standard",
			body:        validConfigBody(),
			updateErr:   configsvc.ErrNotFound,
			wantStatus:  http.StatusNotFound,
			wantError:   "not_found",
			wantMessage: "config not found for 7x7 standard",
		},
		{
			name:       "invalid size not a number returns 400",
			size:       "abc",
			mode:       "standard",
			body:       validConfigBody(),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid mode returns 400",
			size:       "7",
			mode:       "triple",
			body:       validConfigBody(),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "threshold less than 1 returns 400",
			size:       "7",
			mode:       "standard",
			body:       `{"threshold":0,"enabled":true}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "threshold above cap returns 400",
			size:       "7",
			mode:       "standard",
			body:       `{"threshold":51,"enabled":true}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "negative maxAttempts returns 400",
			size:       "7",
			mode:       "standard",
			body:       `{"threshold":10,"enabled":true,"maxAttempts":-1}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "service error returns 500",
			size:       "7",
			mode:       "standard",
			body:       validConfigBody(),
			updateErr:  errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubConfigService{updateErr: tt.updateErr}

			r := chi.NewRouter()
			r.Put("/admin/config/{size}/{mode}", handler.UpdateConfigHandler(svc))

			req := httptest.NewRequest(http.MethodPut,
				"/admin/config/"+tt.size+"/"+tt.mode,
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantError != "" {
				var errResp struct {
					Error   string `json:"error"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if errResp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", errResp.Error, tt.wantError)
				}
				if tt.wantMessage != "" && errResp.Message != tt.wantMessage {
					t.Errorf("message = %q, want %q", errResp.Message, tt.wantMessage)
				}
				return
			}

			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp["size"] != float64(7) {
				t.Errorf("size = %v, want %v", resp["size"], 7)
			}
			if resp["mode"] != "standard" {
				t.Errorf("mode = %v, want %q", resp["mode"], "standard")
			}
			if resp["threshold"] != float64(10) {
				t.Errorf("threshold = %v, want 10", resp["threshold"])
			}
		})
	}
}

func TestCreateConfigHandler(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		createErr   error
		wantStatus  int
		wantError   string
		wantMessage string
	}{
		{
			name:       "valid create returns 201",
			body:       validCreateBody(),
			wantStatus: http.StatusCreated,
		},
		{
			name:        "duplicate returns 409",
			body:        validCreateBody(),
			createErr:   configsvc.ErrAlreadyExists,
			wantStatus:  http.StatusConflict,
			wantError:   "conflict",
			wantMessage: "config already exists for 7x7 standard",
		},
		{
			name:       "invalid fields returns 400",
			body:       `{"size":7,"mode":"standard","threshold":0,"enabled":true}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "service error returns 500",
			body:       validCreateBody(),
			createErr:  errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubConfigService{createErr: tt.createErr}

			r := chi.NewRouter()
			r.Post("/admin/config", handler.CreateConfigHandler(svc))

			req := httptest.NewRequest(http.MethodPost,
				"/admin/config",
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantError != "" {
				var errResp struct {
					Error   string `json:"error"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if errResp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", errResp.Error, tt.wantError)
				}
				if tt.wantMessage != "" && errResp.Message != tt.wantMessage {
					t.Errorf("message = %q, want %q", errResp.Message, tt.wantMessage)
				}
				return
			}

			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp["size"] != float64(7) {
				t.Errorf("size = %v, want %v", resp["size"], 7)
			}
			if resp["mode"] != "standard" {
				t.Errorf("mode = %v, want %q", resp["mode"], "standard")
			}
			if resp["threshold"] != float64(10) {
				t.Errorf("threshold = %v, want 10", resp["threshold"])
			}
		})
	}
}

// TestCreateConfigHandler_AuthMatrix exercises POST /api/admin/config
// behind the auth middleware chain. The admin row's success code is
// 201 Created — adjusted inside the loop.
func TestCreateConfigHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubConfigService{}
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Post("/config", handler.CreateConfigHandler(svc))
			}, roleForState(tc.state))

			wantCode := tc.wantCode
			if wantCode == http.StatusOK {
				wantCode = http.StatusCreated
			}
			req := newAdminRequest(tc.state, http.MethodPost, "/api/admin/config", strings.NewReader(validCreateBody()))
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != wantCode {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, wantCode, rec.Body.String())
			}
		})
	}
}

// TestUpdateConfigHandler_AuthMatrix exercises PUT
// /api/admin/config/{size}/{mode} behind the auth middleware chain.
func TestUpdateConfigHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubConfigService{}
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Put("/config/{size}/{mode}", handler.UpdateConfigHandler(svc))
			}, roleForState(tc.state))

			req := newAdminRequest(tc.state, http.MethodPut, "/api/admin/config/7/standard", strings.NewReader(validConfigBody()))
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}
