package handler

import "testing"

// Internal test for the unexported validateDeviceID helper. Lives in
// package handler (not handler_test) so it can reach the helper
// directly. The handler-level integration test in daily_test.go
// covers the 400 emit path via the public DailyGetHandler surface.

func TestValidateDeviceID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short (15 chars)", "abcdef012345678", true},
		{"exactly min length (16)", "abcdef0123456789", false},
		{"typical UUID (36 chars)", "550e8400-e29b-41d4-a716-446655440000", false},
		{"with underscores", "device_id_test_1234", false},
		{"exactly max length (64)", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2", false},
		{"too long (65 chars)", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2X", true},
		{"5MB string", string(make([]byte, 5*1024*1024)), true},
		{"contains space", "device id with spaces", true},
		{"contains slash", "device/id/with/slash", true},
		{"contains plus", "device+id+with+plus", true},
		{"contains percent", "device%20encoded%20value", true},
		{"contains null byte", "deviceid\x00withnull1", true},
		{"contains newline", "deviceid\nwithnewline", true},
		{"contains unicode", "device-id-héllo", true},
		{"all dashes/underscores at min length", "----____----____", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange + Act
			err := validateDeviceID(tc.id)

			// Assert
			if tc.wantErr && err == nil {
				t.Errorf("expected error for input length %d, got nil", len(tc.id))
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected nil error for %q, got %v", tc.id, err)
			}
		})
	}
}
