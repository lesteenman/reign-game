//go:build corpus

package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestCorpusGenerate produces testdata/puzzles/*.json — ~10 puzzles per
// (difficulty, N, k) at the committed grid. Build-tagged because it
// overwrites files and takes several minutes.
//
// Re-run whenever generator behavior changes intentionally:
//
//	go test -tags=corpus -run TestCorpusGenerate -v -timeout=60m \
//	  ./internal/generator/
//
// The generated JSONs are deterministic (seeded) — running twice
// produces byte-identical files. Human verification: play a handful
// per tier to confirm the difficulty label feels right; if an Expert
// is actually trivial, open a follow-up to tune the classifier.
//
// The corpus is consumed by TestCorpusRoundtrip (default suite) which
// asserts the committed puzzles still solve and their difficulty
// label stays stable.
func TestCorpusGenerate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping corpus generation under -short")
	}

	perBucket := 10

	buckets := []struct {
		n, k int
		diff Difficulty
	}{
		{n: 9, k: 1, diff: Easy},
		{n: 9, k: 1, diff: Medium},
		{n: 9, k: 1, diff: Hard},
		{n: 12, k: 1, diff: Easy},
		{n: 12, k: 1, diff: Medium},
		{n: 12, k: 1, diff: Hard},
		{n: 12, k: 1, diff: Expert},
		{n: 9, k: 2, diff: Easy},
		{n: 9, k: 2, diff: Medium},
		{n: 9, k: 2, diff: Hard},
		{n: 12, k: 2, diff: Easy},
		{n: 12, k: 2, diff: Medium},
		{n: 12, k: 2, diff: Hard},
	}

	dir := filepath.Join("testdata", "puzzles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	for _, b := range buckets {
		seed := int64(9000*b.n + 100*b.k + 10*int(b.diff))
		g, err := New(b.n, b.k,
			WithSeed(seed),
			WithMaxAttempts(200),
			WithDifficulty(b.diff),
		)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		collected := 0
		attempts := 0
		const maxAttempts = 1000
		for collected < perBucket && attempts < maxAttempts {
			attempts++
			p, err := g.Generate(context.Background())
			if err != nil {
				if errors.Is(err, ErrMaxAttemptsExhausted) {
					continue
				}
				t.Fatalf("N=%d k=%d diff=%v: %v", b.n, b.k, b.diff, err)
			}
			path := filepath.Join(dir,
				fmt.Sprintf("n%d_k%d_%s_%02d.json", b.n, b.k,
					difficultyTag(b.diff), collected))
			data, err := json.MarshalIndent(p, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			collected++
		}
		if collected < perBucket {
			t.Logf("N=%d k=%d diff=%v: collected %d/%d (Expert is naturally rare)",
				b.n, b.k, b.diff, collected, perBucket)
		}
	}
}
