package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// mockConfigRepo implements handler.ConfigRepo for testing.
type mockConfigRepo struct {
	getConfigFunc    func(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
	putConfigFunc    func(ctx context.Context, config *repository.ConfigRecord) error
	createConfigFunc func(ctx context.Context, config *repository.ConfigRecord) error
}

func (m *mockConfigRepo) GetConfig(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error) {
	return m.getConfigFunc(ctx, size, mode)
}

func (m *mockConfigRepo) PutConfig(ctx context.Context, config *repository.ConfigRecord) error {
	return m.putConfigFunc(ctx, config)
}

func (m *mockConfigRepo) CreateConfig(ctx context.Context, config *repository.ConfigRecord) error {
	return m.createConfigFunc(ctx, config)
}

// validConfigBody returns a valid config JSON body for tests.
func validConfigBody() string {
	return `{
		"pipeline": "iterative",
		"solver": "propagation",
		"regions": "bfs",
		"regionVariance": 0.5,
		"deducible": true,
		"concurrency": 2,
		"threshold": 10,
		"enabled": true
	}`
}

// validCreateBody returns a valid config JSON body with size and mode for POST tests.
func validCreateBody() string {
	return `{
		"size": 7,
		"mode": "standard",
		"pipeline": "iterative",
		"solver": "propagation",
		"regions": "bfs",
		"regionVariance": 0.5,
		"deducible": true,
		"concurrency": 2,
		"threshold": 10,
		"enabled": true
	}`
}

func TestUpdateConfigHandler(t *testing.T) {
	existingConfig := &repository.ConfigRecord{
		Size:           7,
		Mode:           "standard",
		Pipeline:       "iterative",
		Solver:         "propagation",
		Regions:        "bfs",
		RegionVariance: 0.3,
		Deducible:      true,
		Concurrency:    1,
		Threshold:      5,
		Enabled:        true,
	}

	tests := []struct {
		name           string
		size           string
		mode           string
		body           string
		getConfigFunc  func(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
		putConfigFunc  func(ctx context.Context, config *repository.ConfigRecord) error
		wantStatus     int
		wantError      string
		wantMessage    string
	}{
		{
			name: "valid update returns 200",
			size: "7",
			mode: "standard",
			body: validConfigBody(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "config not found returns 404",
			size: "7",
			mode: "standard",
			body: validConfigBody(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return nil, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus:  http.StatusNotFound,
			wantError:   "not_found",
			wantMessage: "config not found for 7x7 standard",
		},
		{
			name: "invalid size not a number returns 400",
			size: "abc",
			mode: "standard",
			body: validConfigBody(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "invalid mode returns 400",
			size: "7",
			mode: "triple",
			body: validConfigBody(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "invalid pipeline returns 400",
			size: "7",
			mode: "standard",
			body: `{"pipeline":"random","solver":"backtrack","regions":"bfs","regionVariance":0.5,"concurrency":2,"threshold":10}`,
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "regionVariance out of range returns 400",
			size: "7",
			mode: "standard",
			body: `{"pipeline":"iterative","solver":"backtrack","regions":"bfs","regionVariance":1.5,"concurrency":2,"threshold":10}`,
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "regionVariance NaN returns 400",
			size: "7",
			mode: "standard",
			body: func() string {
				type req struct {
					Pipeline       string  `json:"pipeline"`
					Solver         string  `json:"solver"`
					Regions        string  `json:"regions"`
					RegionVariance float64 `json:"regionVariance"`
					Concurrency    int     `json:"concurrency"`
					Threshold      int     `json:"threshold"`
				}
				b, _ := json.Marshal(req{
					Pipeline:       "iterative",
					Solver:         "backtrack",
					Regions:        "bfs",
					RegionVariance: math.NaN(),
					Concurrency:    2,
					Threshold:      10,
				})
				return string(b)
			}(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "concurrency out of range returns 400",
			size: "7",
			mode: "standard",
			body: `{"pipeline":"iterative","solver":"backtrack","regions":"bfs","regionVariance":0.5,"concurrency":10,"threshold":10}`,
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "threshold less than 1 returns 400",
			size: "7",
			mode: "standard",
			body: `{"pipeline":"iterative","solver":"backtrack","regions":"bfs","regionVariance":0.5,"concurrency":2,"threshold":0}`,
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "DynamoDB GetConfig error returns 500",
			size: "7",
			mode: "standard",
			body: validConfigBody(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return nil, errors.New("dynamodb error")
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
		{
			name: "DynamoDB PutConfig error returns 500",
			size: "7",
			mode: "standard",
			body: validConfigBody(),
			getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
				return existingConfig, nil
			},
			putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return errors.New("dynamodb error")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := &mockConfigRepo{
				getConfigFunc: tt.getConfigFunc,
				putConfigFunc: tt.putConfigFunc,
				createConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
					return nil
				},
			}

			r := chi.NewRouter()
			r.Put("/admin/config/{size}/{mode}", handler.UpdateConfigHandler(repo))

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

			// Verify response contains the config fields.
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp["pipeline"] != "iterative" {
				t.Errorf("pipeline = %v, want %q", resp["pipeline"], "iterative")
			}
			if resp["size"] != float64(7) {
				t.Errorf("size = %v, want %v", resp["size"], 7)
			}
			if resp["mode"] != "standard" {
				t.Errorf("mode = %v, want %q", resp["mode"], "standard")
			}
		})
	}
}

func TestCreateConfigHandler(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		createConfigFunc func(ctx context.Context, config *repository.ConfigRecord) error
		wantStatus       int
		wantError        string
		wantMessage      string
	}{
		{
			name: "valid create returns 201",
			body: validCreateBody(),
			createConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "duplicate returns 409",
			body: validCreateBody(),
			createConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return &repository.ConfigAlreadyExistsError{Size: 7, Mode: "standard"}
			},
			wantStatus:  http.StatusConflict,
			wantError:   "conflict",
			wantMessage: "config already exists for 7x7 standard",
		},
		{
			name: "invalid fields returns 400",
			body: `{
				"size": 7,
				"mode": "standard",
				"pipeline": "invalid",
				"solver": "backtrack",
				"regions": "bfs",
				"regionVariance": 0.5,
				"concurrency": 2,
				"threshold": 10
			}`,
			createConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return nil
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name: "DynamoDB CreateConfig error returns 500",
			body: validCreateBody(),
			createConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
				return errors.New("dynamodb error")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := &mockConfigRepo{
				getConfigFunc: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
					return nil, nil
				},
				putConfigFunc: func(_ context.Context, _ *repository.ConfigRecord) error {
					return nil
				},
				createConfigFunc: tt.createConfigFunc,
			}

			r := chi.NewRouter()
			r.Post("/admin/config", handler.CreateConfigHandler(repo))

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

			// Verify response contains the config fields.
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp["pipeline"] != "iterative" {
				t.Errorf("pipeline = %v, want %q", resp["pipeline"], "iterative")
			}
			if resp["size"] != float64(7) {
				t.Errorf("size = %v, want %v", resp["size"], 7)
			}
			if resp["mode"] != "standard" {
				t.Errorf("mode = %v, want %q", resp["mode"], "standard")
			}
		})
	}
}
