package profile_test

import (
	"errors"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/profile"
)

func TestValidateUsername_Format(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid alphanumeric", input: "player42", wantErr: nil},
		{name: "valid with underscore", input: "cool_player", wantErr: nil},
		{name: "valid with hyphen", input: "cool-player", wantErr: nil},
		{name: "valid min length", input: "abc", wantErr: nil},
		{name: "valid max length", input: "abcdefghij0123456789", wantErr: nil}, // 20 chars
		{name: "too short", input: "ab", wantErr: profile.ErrInvalidFormat},
		{name: "too long", input: "abcdefghij0123456789X", wantErr: profile.ErrInvalidFormat}, // 21 chars
		{name: "empty", input: "", wantErr: profile.ErrInvalidFormat},
		{name: "space", input: "cool player", wantErr: profile.ErrInvalidFormat},
		{name: "at sign", input: "player@home", wantErr: profile.ErrInvalidFormat},
		{name: "dot", input: "player.one", wantErr: profile.ErrInvalidFormat},
		{name: "unicode", input: "plåyer42", wantErr: profile.ErrInvalidFormat},
	}

	validator := profile.NewUsernameValidator()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			// (validator + tc.input)

			// Act
			err := validator.Validate(tc.input)

			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestValidateUsername_Profanity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "clean name passes", input: "happyplayer", wantErr: nil},
		{name: "obvious profanity rejected", input: "shithead", wantErr: profile.ErrNotAllowed},
		// Leetspeak normalization: go-away maps 4->a, 3->e, etc. "4ssh0le" -> "asshole".
		{name: "leetspeak profanity rejected", input: "4ssh0le", wantErr: profile.ErrNotAllowed},
	}

	validator := profile.NewUsernameValidator()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			// (validator + tc.input)

			// Act
			err := validator.Validate(tc.input)

			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestValidateUsername_Reserved(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "admin reserved", input: "admin"},
		{name: "root reserved", input: "root"},
		{name: "reserved case-insensitive", input: "Admin"},
		{name: "reserved mixed case", input: "ROOT"},
		{name: "brand term reign", input: "reign"},
	}

	validator := profile.NewUsernameValidator()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			// (validator + tc.input)

			// Act
			err := validator.Validate(tc.input)

			// Assert
			if !errors.Is(err, profile.ErrNotAllowed) {
				t.Errorf("Validate(%q) error = %v, want ErrNotAllowed", tc.input, err)
			}
		})
	}
}

// TestValidateUsername_FormatBeforeContent asserts that a value failing
// the charset/length gate returns ErrInvalidFormat even if it would also
// trip the content checks — format is the cheaper, earlier gate.
func TestValidateUsername_FormatBeforeContent(t *testing.T) {
	// Arrange
	validator := profile.NewUsernameValidator()

	// Act
	err := validator.Validate("ab") // too short

	// Assert
	if !errors.Is(err, profile.ErrInvalidFormat) {
		t.Errorf("Validate(too-short) error = %v, want ErrInvalidFormat", err)
	}
}
