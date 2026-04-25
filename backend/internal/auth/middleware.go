package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	gojwt "github.com/go-jose/go-jose/v3/jwt"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
)

// sessionCookieName is the cookie that Clerk's browser SDK sets for
// the session JWT when hosted on a first-party domain (same-origin or
// via CloudFront behavior forwarding cookies). BM-10 / CloudFront
// cookie forwarding is wired in the devops slice (R-089).
const sessionCookieName = "__session"

// userContextKey is the unexported type used as the request-context
// key for the authenticated user. A zero-sized struct type means no
// caller outside this package can collide with it, even by accident.
type userContextKey struct{}

// sessionVerifier is the narrow contract RequireAuth depends on. The
// production implementation (clerkSessionVerifier) wraps
// github.com/clerk/clerk-sdk-go/v2/jwt.Verify and
// github.com/clerk/clerk-sdk-go/v2/user.Get.
//
// Defined as an interface so middleware tests can substitute a fake
// without standing up a real Clerk SDK client.
type sessionVerifier interface {
	// Verify decodes and verifies a Clerk session JWT. Any non-nil
	// error is treated by the middleware as "session invalid" (401)
	// regardless of its underlying cause (expired, bad signature,
	// malformed, network failure). BM-09 mandates fail-closed.
	Verify(ctx context.Context, token string) (*clerk.SessionClaims, error)
	// GetUser fetches the full Clerk user record for a subject ID.
	// The middleware needs this to read publicMetadata.role — the
	// session JWT itself does not carry custom public metadata.
	GetUser(ctx context.Context, id string) (*clerk.User, error)
}

// clerkSessionVerifier is the production sessionVerifier. It relies
// on clerk.SetKey(...) having been called at startup — the
// SDK's package-level user client picks the key up from there, and
// jwt.Verify fetches JWKS lazily.
type clerkSessionVerifier struct{}

// NewClerkSessionVerifier returns a sessionVerifier backed by the
// real Clerk Go SDK v2. Call clerk.SetKey(secret) exactly once before
// handling requests so the SDK's default user client can authenticate.
func NewClerkSessionVerifier() sessionVerifier {
	return clerkSessionVerifier{}
}

func (clerkSessionVerifier) Verify(ctx context.Context, token string) (*clerk.SessionClaims, error) {
	return jwt.Verify(ctx, &jwt.VerifyParams{Token: token})
}

func (clerkSessionVerifier) GetUser(ctx context.Context, id string) (*clerk.User, error) {
	return user.Get(ctx, id)
}

// RequireAuth returns HTTP middleware that enforces Clerk session
// validity on every non-OPTIONS request. See BM-01, BM-08, BM-09.
//
// The returned middleware:
//   - lets OPTIONS requests pass straight through (CORS preflight),
//   - reads the __session cookie from the request,
//   - verifies it via the supplied sessionVerifier,
//   - fetches the full user record and attaches it to the request
//     context under userContextKey{},
//   - responds 401 on any failure along the way.
//
// Failures never produce 5xx — Clerk SDK errors are logged at WARN
// and answered with 401 (fail-closed).
func RequireAuth(verifier sessionVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// BM-08: OPTIONS (CORS preflight) skips auth entirely.
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				log.Printf("auth: reject path=%s reason=no_session_cookie", r.URL.Path)
				writeUnauth(w, "authentication required")
				return
			}

			claims, err := verifier.Verify(r.Context(), cookie.Value)
			if err != nil {
				// BM-07: do NOT log the cookie value or the full
				// error object — the error string from jwt.Verify
				// doesn't include the token, but %v on deep SDK
				// errors could. Log only err.Error() which is the
				// verifier's human-readable reason.
				//
				// BM-01: distinguish an expired session from other
				// verification failures so the client can render
				// "session expired, please sign in again" rather
				// than a generic error. Clerk's jwt.Verify delegates
				// to go-jose's ValidateWithLeeway, which returns
				// gojwt.ErrExpired on expiry — matchable via
				// errors.Is. Every other verify error maps to
				// "invalid session".
				if errors.Is(err, gojwt.ErrExpired) {
					log.Printf("auth: reject path=%s reason=session_expired", r.URL.Path)
					writeUnauth(w, "session expired")
					return
				}
				log.Printf("WARN: auth: clerk SDK error path=%s reason=verify_failed err=%q", r.URL.Path, err.Error())
				writeUnauth(w, "invalid session")
				return
			}
			if claims == nil || claims.Subject == "" {
				log.Printf("auth: reject path=%s reason=claims_missing_subject", r.URL.Path)
				writeUnauth(w, "invalid session")
				return
			}

			u, err := verifier.GetUser(r.Context(), claims.Subject)
			if err != nil {
				log.Printf("WARN: auth: clerk SDK error path=%s reason=user_fetch_failed sub=%s err=%q", r.URL.Path, claims.Subject, err.Error())
				writeUnauth(w, "user not found")
				return
			}
			if u == nil {
				log.Printf("auth: reject path=%s reason=user_not_found sub=%s", r.URL.Path, claims.Subject)
				writeUnauth(w, "user not found")
				return
			}

			log.Printf("auth: allow path=%s sub=%s", r.URL.Path, u.ID)
			ctx := context.WithValue(r.Context(), userContextKey{}, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// publicMetadataRole is the minimal view into Clerk's publicMetadata
// JSON. Clerk returns PublicMetadata as json.RawMessage; we only care
// about the role field in this phase.
type publicMetadataRole struct {
	Role string `json:"role"`
}

// RequireAdmin is middleware that admits a request only if the user
// attached to the request context has publicMetadata.role == "admin".
// Runs after RequireAuth (see BM-02, BM-03, BM-04).
//
// Case sensitive. String equality. No fallback. "Admin",
// "administrator", and missing role all produce 403.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BM-04: UserFromContext panics on a request that was not
		// preceded by RequireAuth. The panic surfaces a programmer
		// error (wrong middleware order) at development time.
		u := UserFromContext(r.Context())

		var meta publicMetadataRole
		// len==0 guards against empty PublicMetadata which json.Unmarshal
		// would reject — an empty/missing metadata blob means "no role."
		if len(u.PublicMetadata) > 0 {
			// Ignore unmarshal errors deliberately: malformed metadata
			// should not produce 500s. It just means "no admin role."
			_ = json.Unmarshal(u.PublicMetadata, &meta)
		}
		if meta.Role != "admin" {
			log.Printf("auth: reject path=%s reason=role_not_admin sub=%s", r.URL.Path, u.ID)
			writeForbidden(w, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext returns the authenticated Clerk user from the
// request context. Panics when the context has no user — this is a
// programmer error (RequireAuth was not wired before the handler),
// never a runtime condition. See BM-04.
func UserFromContext(ctx context.Context) *clerk.User {
	u, ok := UserFromContextOK(ctx)
	if !ok {
		panic("auth.UserFromContext: RequireAuth middleware did not run on this request")
	}
	return u
}

// UserFromContextOK is the non-panicking variant. Returns the user
// and true when present; nil and false otherwise. Primarily useful
// in tests that want to assert presence or absence.
func UserFromContextOK(ctx context.Context) (*clerk.User, bool) {
	u, ok := ctx.Value(userContextKey{}).(*clerk.User)
	return u, ok
}

// WithUserForTest returns a new context with the given user
// installed under the package-private context key. Intended for
// handler tests that exercise routes behind RequireAdmin without
// standing up a full sessionVerifier.
//
// This is the sanctioned way to bypass the auth layer in tests —
// nothing outside this package's tests should import it from
// non-test code, and the function's name makes that expectation
// obvious at call sites.
func WithUserForTest(ctx context.Context, u *clerk.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

// writeUnauth emits the canonical 401 response shape expected by BM-01.
// All auth rejections go through this one function so the client sees
// a single body shape: {"error":"unauthorized","message":"..."}.
func writeUnauth(w http.ResponseWriter, message string) {
	httperr.WriteError(w, http.StatusUnauthorized, "unauthorized", message)
}

// writeForbidden emits the canonical 403 response shape expected by
// BM-02.
func writeForbidden(w http.ResponseWriter, message string) {
	httperr.WriteError(w, http.StatusForbidden, "forbidden", message)
}
