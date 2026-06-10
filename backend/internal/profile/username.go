// Package profile holds the user-profile domain rules. Its sole concern
// today is validating a player-chosen leaderboard username: format
// (length + charset) and content (profanity + reserved names). The
// package is pure — no I/O, no Clerk — so the rules are unit-testable in
// isolation and reused by the HTTP handler that performs the Clerk write.
package profile

import (
	"errors"
	"regexp"
	"strings"

	goaway "github.com/TwiN/go-away"
)

// Username length bounds. Aligned with Clerk's username constraints
// (Clerk allows 4–64 by default but is configurable); our 3–20 window
// sits inside any reasonable Clerk setting so a name we accept is never
// rejected by Clerk for length.
const (
	minUsernameLength = 3
	maxUsernameLength = 20
)

var (
	// ErrInvalidFormat means the value failed the length or charset
	// gate. Maps to HTTP 400.
	ErrInvalidFormat = errors.New("username format invalid")
	// ErrNotAllowed means the value is well-formed but blocked —
	// profanity or a reserved name. Maps to HTTP 422.
	ErrNotAllowed = errors.New("username not allowed")
)

// usernameCharset is the allowed charset: ASCII alphanumerics plus
// underscore and hyphen. Anchored so the whole string must match.
var usernameCharset = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// reservedNames is the in-repo override denylist: brand terms and
// privileged identifiers that must never become a public display name,
// independent of the profanity dictionary. Compared case-insensitively
// (entries are lowercase). This is the single tunable place for
// reserved names alongside the go-away custom dictionary below.
var reservedNames = map[string]struct{}{
	"admin":         {},
	"administrator": {},
	"root":          {},
	"moderator":     {},
	"mod":           {},
	"support":       {},
	"system":        {},
	"reign":         {},
}

// UsernameValidator validates player-chosen usernames against format,
// profanity, and reserved-name rules. Construct via NewUsernameValidator;
// the embedded profanity detector is configured once and is safe for
// concurrent use.
type UsernameValidator struct {
	detector *goaway.ProfanityDetector
}

// NewUsernameValidator builds a validator with leetspeak and
// special-character sanitization enabled so obfuscated profanities
// (e.g. "4ssh0le") are still caught. The profanity dictionary is the
// single tunable place for blocked content; reserved names live in the
// reservedNames map above.
func NewUsernameValidator() *UsernameValidator {
	detector := goaway.NewProfanityDetector().
		WithSanitizeLeetSpeak(true).
		WithSanitizeSpecialCharacters(true)
	return &UsernameValidator{detector: detector}
}

// Validate returns nil if the name is acceptable, ErrInvalidFormat if it
// fails the length/charset gate, or ErrNotAllowed if it is profane or
// reserved. Format is checked first (the cheap, earlier gate); content
// checks run on a normalized (lowercased) copy.
func (v *UsernameValidator) Validate(name string) error {
	if len(name) < minUsernameLength || len(name) > maxUsernameLength {
		return ErrInvalidFormat
	}
	if !usernameCharset.MatchString(name) {
		return ErrInvalidFormat
	}

	normalized := strings.ToLower(name)
	if _, reserved := reservedNames[normalized]; reserved {
		return ErrNotAllowed
	}
	if v.detector.IsProfane(normalized) {
		return ErrNotAllowed
	}
	return nil
}
