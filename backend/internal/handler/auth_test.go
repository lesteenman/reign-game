package handler_test

// This file hosts shared test helpers for exercising admin routes
// behind the auth middleware. The helpers live here (not in the auth
// package) because the handler tests are the only callers, and
// keeping them in _test.go files means production binaries never
// link them.
//
// The helpers mount a chi router group under /api/admin with
// auth.RequireAuth + auth.RequireAdmin, using a fake sessionVerifier
// to stand in for the real Clerk SDK. Each test picks the session
// state (anonymous / user / admin) by choosing which mount function
// to call.
//
// R-08B's tester can import this file's contracts if needed — they
// are deliberately exported via the test-package symbol table.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
)

const (
	// testSessionCookieName must match auth.sessionCookieName — the
	// middleware reads this exact cookie name. Duplicated here as a
	// constant because the auth package does not export it (correct:
	// the outside world has no business knowing which cookie name
	// Clerk uses, the middleware is the only abstraction that sees
	// the raw cookie).
	testSessionCookieName = "__session"
	testSessionToken      = "test-session-jwt"
	testAdminSub          = "user_admin_test"
	testUserSub           = "user_regular_test"
)

// fakeAuthState describes one of the three scenarios every admin
// route tests for.
type fakeAuthState int

const (
	fakeAuthAnonymous fakeAuthState = iota // no cookie on the request
	fakeAuthUserRole                       // cookie + session valid, role=user → 403
	fakeAuthAdminRole                      // cookie + session valid, role=admin → 200
)

// fakeAuthVerifier implements auth.sessionVerifier (via interface
// satisfaction — the auth package accepts any type with the two
// methods below). Returns a synthetic admin or regular user
// depending on which constant (testAdminSub / testUserSub) was
// requested.
type fakeAuthVerifier struct {
	role string // "admin" or "user"
	sub  string
}

func (f *fakeAuthVerifier) Verify(_ context.Context, _ string) (*clerk.SessionClaims, error) {
	claims := &clerk.SessionClaims{}
	claims.Subject = f.sub
	return claims, nil
}

func (f *fakeAuthVerifier) GetUser(_ context.Context, _ string) (*clerk.User, error) {
	u := &clerk.User{ID: f.sub}
	body, _ := json.Marshal(map[string]string{"role": f.role})
	u.PublicMetadata = body
	return u, nil
}

// mountAdminWithAuth mounts the provided admin route under
// /api/admin behind a RequireAuth(verifier)+RequireAdmin chain. The
// caller specifies how the route is registered via the `register`
// callback (mirroring how main.go's /api/admin group is populated).
func mountAdminWithAuth(register func(r chi.Router), role string) *chi.Mux {
	sub := testAdminSub
	if role != "admin" {
		sub = testUserSub
	}
	verifier := &fakeAuthVerifier{role: role, sub: sub}
	r := chi.NewRouter()
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(auth.RequireAuth(verifier))
		r.Use(auth.RequireAdmin)
		register(r)
	})
	return r
}

// newAdminRequest builds a request that the auth middleware treats
// according to `state`. The caller still provides the method, path,
// and body; this helper only decorates the request with the right
// cookie (or no cookie for the anonymous case).
func newAdminRequest(state fakeAuthState, method, path string, body io.Reader) *http.Request {
	if body == nil {
		body = http.NoBody
	}
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		// test setup error — propagate via panic so the test that
		// called this helper fails loudly rather than silently
		// producing a nil request.
		panic("newAdminRequest: " + err.Error())
	}
	if state != fakeAuthAnonymous {
		req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
	}
	return req
}

// adminAuthMatrix is the shared 3-state matrix that every
// /api/admin/* route's test file iterates over. One row per test
// file is boilerplate, but keeps each route's auth coverage
// co-located with its own test file for easier grep.
var adminAuthMatrix = []struct {
	name     string
	state    fakeAuthState
	wantCode int
}{
	{name: "anonymous returns 401", state: fakeAuthAnonymous, wantCode: http.StatusUnauthorized},
	{name: "user role returns 403", state: fakeAuthUserRole, wantCode: http.StatusForbidden},
	{name: "admin role returns 200", state: fakeAuthAdminRole, wantCode: http.StatusOK},
}

// roleForState returns the publicMetadata.role the fake verifier
// should report for the given session state. "anonymous" doubles as
// a sentinel; the mountAdminWithAuth caller only cares about "user"
// vs "admin" and the anonymous path never runs GetUser.
func roleForState(s fakeAuthState) string {
	switch s {
	case fakeAuthAdminRole:
		return "admin"
	case fakeAuthUserRole:
		return "user"
	default:
		return "user"
	}
}
