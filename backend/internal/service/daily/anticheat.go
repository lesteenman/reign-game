package daily

// Anti-cheat heuristics for daily submissions (#171). Two cheap,
// conservative checks run server-side after serverElapsedMs is computed:
//
//  1. Min-time floor — a per-difficulty lower bound on plausible human
//     solve time. A solve faster than the floor is implausible.
//  2. Client/server time divergence — when the client-claimed playTimeMs
//     diverges far from the server-measured elapsed, the client clock or
//     the submission is suspect.
//
// A flagged solve still counts (PLAY outcome=solved) but is excluded from
// the leaderboard and carries the reason on the PLAY row. The player-facing
// response is unchanged — exclusion is silent so as not to tip off cheaters.

// Cheat-flag reason strings. Stored verbatim in the PLAY row's cheatFlag
// attribute and asserted on in tests, so they are the contract.
const (
	cheatFlagBelowFloor     = "belowFloor"
	cheatFlagTimeDivergence = "timeDivergence"
)

// minTimeFloorMs is the per-difficulty minimum plausible solve time in
// milliseconds. Keyed by PuzzleRecord.Difficulty (0 unknown, 1 Easy,
// 2 Medium, 3 Hard, 4 Expert). These are conservative STARTING values
// chosen low enough never to catch a genuine fast human; they are tunable
// once real player data exists (relates to #137's corpus). Difficulty 0
// (unknown) has no entry → no floor, so older / untiered puzzles are
// never flagged on this basis.
var minTimeFloorMs = map[int]int64{
	1: 4000,  // Easy
	2: 8000,  // Medium
	3: 12000, // Hard
	4: 16000, // Expert
}

// divergenceFloorMs is the absolute lower bound of the divergence
// tolerance — divergences at or below this are always accepted regardless
// of solve length. Tunable starting value.
const divergenceFloorMs int64 = 2000

// divergenceFraction is the relative component of the divergence tolerance:
// a divergence is tolerated up to this fraction of serverElapsedMs. The
// effective tolerance is max(divergenceFloorMs, divergenceFraction *
// serverElapsedMs). Tunable starting value (50%).
const divergenceFraction = 0.5

// evaluateCheatFlag returns the cheat-flag reason for a solve, or "" when
// the solve is plausible. The floor check takes precedence over the
// divergence check. The divergence check is skipped entirely when
// clientClaimedMs is 0/absent (older clients don't send it — not a signal).
func evaluateCheatFlag(difficulty int, serverElapsedMs, clientClaimedMs int64) string {
	if floor, ok := minTimeFloorMs[difficulty]; ok && serverElapsedMs < floor {
		return cheatFlagBelowFloor
	}

	if clientClaimedMs > 0 {
		divergence := serverElapsedMs - clientClaimedMs
		if divergence < 0 {
			divergence = -divergence
		}
		tolerance := divergenceFloorMs
		if frac := int64(float64(serverElapsedMs) * divergenceFraction); frac > tolerance {
			tolerance = frac
		}
		if divergence > tolerance {
			return cheatFlagTimeDivergence
		}
	}

	return ""
}
