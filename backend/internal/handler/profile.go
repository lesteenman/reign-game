package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/profile"
)

// clerkConflictCode is the Clerk API error code returned when an
// identifier (here, the username) already belongs to another user.
// Clerk reports username collisions under this code; we map it to a 409.
const clerkConflictCode = "form_identifier_exists"

// UsernameUpdater is the narrow Clerk surface the handler needs: set a
// user's native username. Defined as an interface so tests substitute a
// stub without calling the real Clerk API. The production implementation
// (clerkUsernameUpdater) wraps github.com/clerk/clerk-sdk-go/v2/user.Update.
type UsernameUpdater interface {
	SetUsername(ctx context.Context, userID, username string) error
}

// clerkUsernameUpdater is the production UsernameUpdater. It relies on
// clerk.SetKey(...) having been called at startup so the SDK's
// package-level user client can authenticate.
type clerkUsernameUpdater struct{}

// NewClerkUsernameUpdater returns a UsernameUpdater backed by the real
// Clerk Go SDK v2 user.Update call.
func NewClerkUsernameUpdater() UsernameUpdater {
	return clerkUsernameUpdater{}
}

func (clerkUsernameUpdater) SetUsername(ctx context.Context, userID, username string) error {
	_, err := user.Update(ctx, userID, &user.UpdateParams{Username: &username})
	return err
}

// usernameValidator is the format/profanity gate the handler runs before
// the Clerk write. Defined as an interface so tests can inject a stub,
// though the production wiring uses profile.NewUsernameValidator.
type usernameValidator interface {
	Validate(name string) error
}

// usernameRequest is the decoded JSON body for PUT /api/profile/username.
type usernameRequest struct {
	Username string `json:"username"`
}

// usernameResponse is the success envelope: the username now set on the
// Clerk user, echoed so the frontend can render it without a follow-up
// fetch.
type usernameResponse struct {
	Username string `json:"username"`
}

// ProfileUsernameHandler creates the HTTP handler for
// PUT /api/profile/username.
//
// Mounted inside the /api/profile route group so RequireAuth runs once
// at the group level (signed-in only; no admin gate). The handler:
//  1. reads the authenticated Clerk user from the request context,
//  2. validates the requested name's format (400) then content (422),
//  3. calls Clerk user.Update to set the native username,
//  4. maps Clerk's uniqueness conflict to 409 and other Clerk errors to
//     502.
//
// Validation runs BEFORE the Clerk write so profanity/reserved names
// never reach Clerk — Clerk's drop-in UI would bypass this gate, which
// is exactly why the set flow is backend-mediated.
func ProfileUsernameHandler(updater UsernameUpdater, validator usernameValidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		u := auth.UserFromContext(r.Context())
		if u == nil {
			// Defense-in-depth: RequireAuth should have returned 401
			// before this. Reaching here means the middleware chain is
			// misconfigured — surface a 500 rather than calling Clerk
			// with an empty user ID.
			log.Printf("profile: UserFromContext returned nil after RequireAuth")
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "authentication context missing")
			return
		}

		var req usernameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		if err := validator.Validate(req.Username); err != nil {
			switch {
			case errors.Is(err, profile.ErrInvalidFormat):
				httperr.WriteError(w, http.StatusBadRequest, "invalid_format", "Username must be 3-20 characters using letters, numbers, _ or -.")
			case errors.Is(err, profile.ErrNotAllowed):
				httperr.WriteError(w, http.StatusUnprocessableEntity, "not_allowed", "That username isn't allowed. Please choose another.")
			default:
				log.Printf("profile: unexpected validation error sub=%s err=%v", u.ID, err)
				httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to set username.")
			}
			return
		}

		if err := updater.SetUsername(r.Context(), u.ID, req.Username); err != nil {
			if isClerkConflict(err) {
				log.Printf("profile: username taken sub=%s", u.ID)
				httperr.WriteError(w, http.StatusConflict, "username_taken", "That username is already taken.")
				return
			}
			log.Printf("WARN: profile: clerk update failed sub=%s err=%q", u.ID, err.Error())
			httperr.WriteError(w, http.StatusBadGateway, "upstream_error", "Couldn't set username right now. Please try again.")
			return
		}

		log.Printf("profile: username set sub=%s", u.ID)
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(usernameResponse(req)); err != nil {
			log.Printf("profile: write response failed sub=%s err=%v", u.ID, err)
		}
	}
}

// isClerkConflict reports whether a Clerk SDK error represents a username
// uniqueness conflict. Clerk returns *clerk.APIErrorResponse with a
// per-error Code of "form_identifier_exists" (HTTP 422 upstream) when the
// requested username already belongs to another user. Matched by error
// code via errors.As so it survives error wrapping.
func isClerkConflict(err error) bool {
	var apiErr *clerk.APIErrorResponse
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, e := range apiErr.Errors {
		if e.Code == clerkConflictCode {
			return true
		}
	}
	return false
}
