package daily

import "testing"

func TestEvaluateCheatFlag(t *testing.T) {
	cases := []struct {
		name            string
		difficulty      int
		serverElapsedMs int64
		clientClaimedMs int64
		want            string
	}{
		// Below-floor: Medium floor is 8000ms.
		{"medium below floor", 2, 5000, 5000, cheatFlagBelowFloor},
		{"medium exactly at floor", 2, 8000, 8000, ""},
		{"medium just above floor", 2, 8001, 8001, ""},
		// Hard floor is 12000ms.
		{"hard below floor", 3, 11999, 11999, cheatFlagBelowFloor},
		{"hard at floor", 3, 12000, 12000, ""},
		// Unknown difficulty (0) has no floor → never flagged on floor basis.
		{"unknown difficulty fast solve not floored", 0, 100, 100, ""},
		// Divergence: tolerance = max(2000, 50% of serverElapsedMs).
		// For a 20000ms solve, tolerance = 10000ms.
		{"divergence within relative tolerance", 2, 20000, 11000, ""},
		{"divergence exactly at relative tolerance", 2, 20000, 10000, ""},
		{"divergence beyond relative tolerance", 2, 20000, 9000, cheatFlagTimeDivergence},
		// For a short solve (10000ms, above Medium floor 8000), 50% = 5000,
		// which exceeds the 2000 floor, so tolerance = 5000.
		{"short solve divergence within tolerance", 2, 10000, 6000, ""},
		{"short solve divergence beyond tolerance", 2, 10000, 4000, cheatFlagTimeDivergence},
		// Absolute floor governs when 50% would be below 2000. A 9000ms solve
		// → 50% = 4500 > 2000, tolerance = 4500. Use difficulty 0 to isolate
		// the divergence path from the floor. For difficulty 0, 3000ms solve:
		// 50% = 1500 < 2000 → tolerance = 2000.
		{"tiny solve uses absolute floor within", 0, 3000, 1500, ""},
		{"tiny solve uses absolute floor exactly", 0, 3000, 1000, ""},
		{"tiny solve uses absolute floor beyond", 0, 3000, 900, cheatFlagTimeDivergence},
		// clientClaimedMs == 0 → divergence check skipped entirely.
		{"client claimed zero skips divergence", 0, 20000, 0, ""},
		// Floor takes precedence over divergence.
		{"below floor wins over divergence", 2, 5000, 100, cheatFlagBelowFloor},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange / Act
			got := evaluateCheatFlag(tc.difficulty, tc.serverElapsedMs, tc.clientClaimedMs)

			// Assert
			if got != tc.want {
				t.Errorf("evaluateCheatFlag(diff=%d, server=%d, client=%d) = %q, want %q",
					tc.difficulty, tc.serverElapsedMs, tc.clientClaimedMs, got, tc.want)
			}
		})
	}
}
