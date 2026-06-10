package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/profile"
)

// stubUsernameUpdater is a hand-rolled handler.UsernameUpdater that
// records the call and returns a configurable error, keeping the handler
// tests off the real Clerk API.
type stubUsernameUpdater struct {
	err error

	called      bool
	gotUserID   string
	gotUsername string
}

func (s *stubUsernameUpdater) SetUsername(_ context.Context, userID, username string) error {
	s.called = true
	s.gotUserID = userID
	s.gotUsername = username
	return s.err
}

// clerkConflictErr returns an *clerk.APIErrorResponse shaped like Clerk's
// real username-taken response so isClerkConflict matches it.
func clerkConflictErr() error {
	return &clerk.APIErrorResponse{
		HTTPStatusCode: http.StatusUnprocessableEntity,
		Errors: []clerk.Error{
			{Code: "form_identifier_exists", Message: "That username is taken."},
		},
	}
}

// mountProfileWithAuth mounts the profile route under /api/profile behind
// a RequireAuth(verifier) chain only (no admin gate), mirroring main.go's
// /api/profile group. When signedIn is false no verifier user is wired,
// so requests without a cookie hit the 401 path.
func mountProfileWithAuth(register func(r chi.Router)) *chi.Mux {
	verifier := &fakeAuthVerifier{role: "user", sub: testUserSub}
	r := chi.NewRouter()
	r.Route("/api/profile", func(r chi.Router) {
		r.Use(auth.RequireAuth(verifier))
		register(r)
	})
	return r
}

func profileBody(username string) *bytes.Buffer {
	b, _ := json.Marshal(map[string]string{"username": username})
	return bytes.NewBuffer(b)
}

func TestProfileUsernameHandler_Success(t *testing.T) {
	// Arrange
	updater := &stubUsernameUpdater{}
	validator := profile.NewUsernameValidator()
	router := mountProfileWithAuth(func(r chi.Router) {
		r.Put("/username", handler.ProfileUsernameHandler(updater, validator))
	})
	req := httptest.NewRequest(http.MethodPut, "/api/profile/username", profileBody("happyplayer"))
	req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if !updater.called {
		t.Error("expected updater.SetUsername to be called")
	}
	if updater.gotUserID != testUserSub {
		t.Errorf("userID = %q, want %q", updater.gotUserID, testUserSub)
	}
	if updater.gotUsername != "happyplayer" {
		t.Errorf("username = %q, want happyplayer", updater.gotUsername)
	}
	var resp struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Username != "happyplayer" {
		t.Errorf("response username = %q, want happyplayer", resp.Username)
	}
}

func TestProfileUsernameHandler_Unauthenticated(t *testing.T) {
	// Arrange
	updater := &stubUsernameUpdater{}
	validator := profile.NewUsernameValidator()
	router := mountProfileWithAuth(func(r chi.Router) {
		r.Put("/username", handler.ProfileUsernameHandler(updater, validator))
	})
	// No session cookie -> RequireAuth returns 401.
	req := httptest.NewRequest(http.MethodPut, "/api/profile/username", profileBody("happyplayer"))
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}
	if updater.called {
		t.Error("updater must not be called on an unauthenticated request")
	}
}

func TestProfileUsernameHandler_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantCode int
	}{
		{name: "invalid format too short", username: "ab", wantCode: http.StatusBadRequest},
		{name: "invalid format bad charset", username: "bad name!", wantCode: http.StatusBadRequest},
		{name: "profane name", username: "shithead", wantCode: http.StatusUnprocessableEntity},
		{name: "reserved name", username: "admin", wantCode: http.StatusUnprocessableEntity},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			updater := &stubUsernameUpdater{}
			validator := profile.NewUsernameValidator()
			router := mountProfileWithAuth(func(r chi.Router) {
				r.Put("/username", handler.ProfileUsernameHandler(updater, validator))
			})
			req := httptest.NewRequest(http.MethodPut, "/api/profile/username", profileBody(tc.username))
			req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if updater.called {
				t.Error("updater must not be called when validation fails")
			}
		})
	}
}

func TestProfileUsernameHandler_UniquenessConflict(t *testing.T) {
	// Arrange
	updater := &stubUsernameUpdater{err: clerkConflictErr()}
	validator := profile.NewUsernameValidator()
	router := mountProfileWithAuth(func(r chi.Router) {
		r.Put("/username", handler.ProfileUsernameHandler(updater, validator))
	})
	req := httptest.NewRequest(http.MethodPut, "/api/profile/username", profileBody("happyplayer"))
	req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
}

func TestProfileUsernameHandler_UpstreamError(t *testing.T) {
	// Arrange
	updater := &stubUsernameUpdater{err: &clerk.APIErrorResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		Errors:         []clerk.Error{{Code: "internal_clerk_error"}},
	}}
	validator := profile.NewUsernameValidator()
	router := mountProfileWithAuth(func(r chi.Router) {
		r.Put("/username", handler.ProfileUsernameHandler(updater, validator))
	})
	req := httptest.NewRequest(http.MethodPut, "/api/profile/username", profileBody("happyplayer"))
	req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502; body = %s", rec.Code, rec.Body.String())
	}
}

func TestProfileUsernameHandler_BadBody(t *testing.T) {
	// Arrange
	updater := &stubUsernameUpdater{}
	validator := profile.NewUsernameValidator()
	router := mountProfileWithAuth(func(r chi.Router) {
		r.Put("/username", handler.ProfileUsernameHandler(updater, validator))
	})
	req := httptest.NewRequest(http.MethodPut, "/api/profile/username", bytes.NewBufferString("{not json"))
	req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if updater.called {
		t.Error("updater must not be called on a bad body")
	}
}
