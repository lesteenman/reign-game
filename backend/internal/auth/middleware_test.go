package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	gojwt "github.com/go-jose/go-jose/v3/jwt"
)

// fakeVerifier implements sessionVerifier for middleware tests. Every
// branch of RequireAuth corresponds to a configuration of this fake.
type fakeVerifier struct {
	verifyFn  func(ctx context.Context, token string) (*clerk.SessionClaims, error)
	getUserFn func(ctx context.Context, id string) (*clerk.User, error)
}

func (f *fakeVerifier) Verify(ctx context.Context, token string) (*clerk.SessionClaims, error) {
	return f.verifyFn(ctx, token)
}

func (f *fakeVerifier) GetUser(ctx context.Context, id string) (*clerk.User, error) {
	return f.getUserFn(ctx, id)
}

// claimsFor builds a SessionClaims with only Subject populated,
// mimicking a minimally decoded Clerk session JWT.
func claimsFor(sub string) *clerk.SessionClaims {
	c := &clerk.SessionClaims{}
	c.Subject = sub
	return c
}

// userWithRole builds a *clerk.User whose PublicMetadata JSON contains
// the given role (or no role at all if role == "").
func userWithRole(id, role string) *clerk.User {
	u := &clerk.User{ID: id}
	if role == "" {
		u.PublicMetadata = json.RawMessage(`{}`)
	} else {
		// json.Marshal so role values with quotes or Unicode escape
		// cleanly; keeps the table tests honest.
		body, _ := json.Marshal(map[string]string{"role": role})
		u.PublicMetadata = body
	}
	return u
}

// addSessionCookie attaches the Clerk session cookie to the request.
func addSessionCookie(r *http.Request, token string) {
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
}

// captureLogs redirects the stdlib logger to a bytes.Buffer for the
// duration of the test. Returned cleanup restores the previous
// writer.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return buf
}

// passThroughHandler sets a sentinel status so tests can distinguish
// "middleware let the request through" from the middleware's own
// 200-default when there is no next handler.
type sentinelHandler struct {
	called bool
	user   *clerk.User
}

func (s *sentinelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.called = true
	if u, ok := UserFromContextOK(r.Context()); ok {
		s.user = u
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// ---------------------------------------------------------------------------
// BM-01: RequireAuth rejection modes
// ---------------------------------------------------------------------------

func TestRequireAuth_NoCookie_Returns401(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	mw := RequireAuth(&fakeVerifier{})(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertErrorBody(t, rec, "unauthorized", "authentication required")
	if next.called {
		t.Error("next handler was called despite missing cookie")
	}
}

func TestRequireAuth_InvalidCookie_Returns401(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return nil, errors.New("signature invalid")
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, "not-a-real-jwt")
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertErrorBody(t, rec, "unauthorized", "invalid session")
	if next.called {
		t.Error("next handler was called despite verify failure")
	}
}

func TestRequireAuth_ExpiredSession_Returns401WithExpiredMessage(t *testing.T) {
	// BM-01: an expired-but-otherwise-valid session must surface as
	// "session expired" so the client can prompt re-sign-in, not a
	// generic "invalid session" that looks like tampering.

	// Arrange
	next := &sentinelHandler{}
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return nil, gojwt.ErrExpired
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, "expired-jwt")
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertErrorBody(t, rec, "unauthorized", "session expired")
	if next.called {
		t.Error("next handler was called despite expired session")
	}
}

func TestRequireAuth_ExpiredWrappedError_StillDetected(t *testing.T) {
	// Verify errors.Is traversal works: a caller might wrap the
	// underlying expiry error. Ensure we still detect expiry
	// regardless of wrapping depth.

	// Arrange
	next := &sentinelHandler{}
	wrapped := fmt.Errorf("clerk: verify failed: %w", gojwt.ErrExpired)
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return nil, wrapped
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, "expired-jwt")
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	assertErrorBody(t, rec, "unauthorized", "session expired")
}

func TestRequireAuth_UserNotFound_Returns401(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return claimsFor("user_123"), nil
		},
		getUserFn: func(context.Context, string) (*clerk.User, error) {
			return nil, errors.New("404 user not found")
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, "valid-jwt")
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertErrorBody(t, rec, "unauthorized", "user not found")
	if next.called {
		t.Error("next handler was called despite GetUser failure")
	}
}

// BM-09: Clerk SDK errors fail closed — they produce 401, never 500.
func TestRequireAuth_ClerkSDKError_LogsWarnAnd401(t *testing.T) {
	// Arrange
	buf := captureLogs(t)
	next := &sentinelHandler{}
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return nil, errors.New("network timeout")
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, "some-jwt")
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("status = 500, auth middleware must fail closed (401), not 500")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(buf.String(), "WARN: auth: clerk SDK error") {
		t.Errorf("expected WARN log for SDK error, got:\n%s", buf.String())
	}
}

func TestRequireAuth_ValidSession_CallsNextWithUser(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	user := userWithRole("user_abc", "admin")
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return claimsFor("user_abc"), nil
		},
		getUserFn: func(_ context.Context, id string) (*clerk.User, error) {
			if id != "user_abc" {
				t.Errorf("GetUser called with id=%q, want %q", id, "user_abc")
			}
			return user, nil
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, "valid-jwt")
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !next.called {
		t.Fatal("next handler was not called")
	}
	if next.user == nil || next.user.ID != "user_abc" {
		t.Errorf("user in context = %+v, want user_abc", next.user)
	}
}

// BM-08: OPTIONS skips auth regardless of cookie state. The browser
// sends preflight without cookies, so rejecting it would kill the
// flow before the real request ever goes out.
func TestRequireAuth_Options_BypassesAuthEntirely(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	// A verifier whose Verify would fail if called. The test checks
	// that the middleware never reaches it.
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			t.Fatal("RequireAuth invoked Verify on an OPTIONS request")
			return nil, nil
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/pool", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	if !next.called {
		t.Fatal("next handler was not called for OPTIONS request")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// BM-02: RequireAdmin role check
// ---------------------------------------------------------------------------

func TestRequireAdmin_RoleAdmin_CallsNext(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	user := userWithRole("user_1", "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	ctx := context.WithValue(req.Context(), userContextKey{}, user)
	rec := httptest.NewRecorder()

	// Act
	RequireAdmin(next).ServeHTTP(rec, req.WithContext(ctx))

	// Assert
	if !next.called {
		t.Fatal("next handler was not called for role=admin")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAdmin_NonAdminRoles_Return403(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{"role=user", "user"},
		{"role missing", ""},
		{"role wrong case Admin", "Admin"},
		{"role wrong value administrator", "administrator"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			next := &sentinelHandler{}
			user := userWithRole("user_1", tt.role)
			req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
			ctx := context.WithValue(req.Context(), userContextKey{}, user)
			rec := httptest.NewRecorder()

			// Act
			RequireAdmin(next).ServeHTTP(rec, req.WithContext(ctx))

			// Assert
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
			assertErrorBody(t, rec, "forbidden", "admin role required")
			if next.called {
				t.Error("next handler was called despite non-admin role")
			}
		})
	}
}

// BM-04: RequireAdmin panics when RequireAuth did not run. The panic
// is a programmer-error signal, not a runtime path.
func TestRequireAdmin_WithoutRequireAuth_Panics(t *testing.T) {
	// Arrange
	next := &sentinelHandler{}
	mw := RequireAdmin(next)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	rec := httptest.NewRecorder()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when RequireAdmin runs without RequireAuth")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "RequireAuth") {
			t.Errorf("panic message = %v, want to mention RequireAuth", r)
		}
	}()

	// Act (should panic)
	mw.ServeHTTP(rec, req)
}

// BM-04 inverse: UserFromContext panics when called without the
// middleware having attached a user.
func TestUserFromContext_NoUser_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when UserFromContext runs without user in context")
		}
	}()
	_ = UserFromContext(context.Background())
}

func TestUserFromContext_WithUser_Returns(t *testing.T) {
	// Arrange
	user := userWithRole("user_1", "admin")
	ctx := context.WithValue(context.Background(), userContextKey{}, user)

	// Act
	got := UserFromContext(ctx)

	// Assert
	if got == nil || got.ID != "user_1" {
		t.Errorf("UserFromContext = %+v, want user_1", got)
	}
}

// ---------------------------------------------------------------------------
// BM-07: Logs must not leak tokens, cookies, or full user objects.
// ---------------------------------------------------------------------------

func TestRequireAuth_DoesNotLogSessionTokenOrCookie(t *testing.T) {
	// Arrange
	const tokenSecret = "eyJSESSIONTOKENSECRET"
	buf := captureLogs(t)
	next := &sentinelHandler{}
	user := userWithRole("user_secret_id", "admin")
	verifier := &fakeVerifier{
		verifyFn: func(context.Context, string) (*clerk.SessionClaims, error) {
			return claimsFor("user_secret_id"), nil
		},
		getUserFn: func(context.Context, string) (*clerk.User, error) {
			return user, nil
		},
	}
	mw := RequireAuth(verifier)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pool", http.NoBody)
	addSessionCookie(req, tokenSecret)
	rec := httptest.NewRecorder()

	// Act
	mw.ServeHTTP(rec, req)

	// Assert
	out := buf.String()
	if strings.Contains(out, tokenSecret) {
		t.Errorf("session token appeared in log output:\n%s", out)
	}
	// Clerk user IDs are fine to log (BM-07 explicitly allows
	// the `sub` claim). We just want to make sure raw cookie
	// bodies and verbose %+v dumps stay out.
	if strings.Contains(out, "PublicMetadata") || strings.Contains(out, "EmailAddresses") {
		t.Errorf("user struct fields leaked into log output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// BM-05/BM-06: covered in the handler-package tests that wire the chi
// router group with real middleware (admin_pool_test.go,
// admin_config_test.go, replenish_test.go).
// ---------------------------------------------------------------------------

// assertErrorBody checks the 401/403 JSON shape from writeUnauth /
// writeForbidden.
func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, wantError, wantMsg string) {
	t.Helper()
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v (body=%q)", err, rec.Body.String())
	}
	if body.Error != wantError {
		t.Errorf("error = %q, want %q", body.Error, wantError)
	}
	if body.Message != wantMsg {
		t.Errorf("message = %q, want %q", body.Message, wantMsg)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
