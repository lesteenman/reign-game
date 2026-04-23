// Command genfixtures produces deterministic puzzle fixtures for the
// R-06B Playwright e2e suite. Fixtures are written as DynamoDB-JSON
// Item payloads so `awslocal dynamodb put-item --item file://<path>`
// ships them unchanged into the puzzle-pool-e2e table.
//
// Determinism: each fixture uses a fixed seed and a fixed puzzle ID.
// Re-running this command produces byte-identical output. If the
// generator's behavior changes intentionally, re-run and commit.
//
// Usage (via Taskfile):
//
//	task e2e:genfixtures
//
// Direct:
//
//	go run ./cmd/genfixtures
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
)

// fixture describes one committed puzzle. The ID is the DynamoDB SK
// and also the value the frontend sees in `puzzle.puzzleId`.
type fixture struct {
	ID   string
	Size int
	K    int
	Mode string
	Seed int64
}

// fixtures enumerates every committed fixture. Keep this list short —
// each added fixture is a test-maintenance line item.
//
// Two 7x7 Standard puzzles are emitted from ONE generator run
// (seed=1). Same content, different SKs. React StrictMode's dev-mode
// double-mount of GamePage fires fetchNextPuzzle twice; the first
// call marks a fixture served before the cleanup cancels the client
// state update. Without a second available fixture the second mount
// gets 404. With two identical fixtures, whichever mount survives
// interacts with an identical puzzle — tests' SOLUTION constant
// stays a single array.
//
// Duplicates is a test-infra workaround. The real fix (split serve-
// and-mark into two endpoints, or make the backend honor a client
// cancel before marking served) is out of scope for R-06B.
var fixtures = []fixture{
	{
		ID:   "e2e0000-0000-4000-a000-000000000001",
		Size: 7,
		K:    1,
		Mode: "standard",
		Seed: 1,
	},
	{
		// Same seed => identical puzzle. Different SK so DynamoDB
		// treats them as two rows.
		ID:   "e2e0000-0000-4000-a000-000000000002",
		Size: 7,
		K:    1,
		Mode: "standard",
		Seed: 1,
	},
}

func main() {
	outDir := "../frontend/playwright/e2e/fixtures/puzzles"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", outDir, err)
		os.Exit(1)
	}

	for _, f := range fixtures {
		if err := writeFixture(outDir, f); err != nil {
			fmt.Fprintf(os.Stderr, "fixture %s (size=%d mode=%s seed=%d): %v\n",
				f.ID, f.Size, f.Mode, f.Seed, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (size=%d mode=%s seed=%d)\n", f.ID, f.Size, f.Mode, f.Seed)
	}
}

func writeFixture(dir string, f fixture) error {
	g, err := generator.New(f.Size, f.K,
		generator.WithSeed(f.Seed),
		generator.WithMaxAttempts(50),
	)
	if err != nil {
		return fmt.Errorf("new generator: %w", err)
	}
	p, err := g.Generate(context.Background())
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	solution := make([][]bool, p.N)
	for i := range solution {
		solution[i] = make([]bool, p.N)
	}
	for _, m := range p.Solution {
		solution[m.Row][m.Col] = true
	}

	pk := fmt.Sprintf("%d#%s", f.Size, f.Mode)
	item := puzzleItem(pk, f.ID, f.Seed, &p, solution)

	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	// Short ID suffix (last 6 chars of the SK UUID). Two fixtures with
	// the same seed but different IDs write to different files.
	idSuffix := f.ID
	if len(idSuffix) > 6 {
		idSuffix = idSuffix[len(idSuffix)-6:]
	}
	base := fmt.Sprintf("%d_%s_seed%d_%s", f.Size, f.Mode, f.Seed, idSuffix)
	if err := os.WriteFile(filepath.Join(dir, base+".json"), data, 0o644); err != nil {
		return fmt.Errorf("write %s.json: %w", base, err)
	}

	// Sibling "*.solution.json": `[[row, col], ...]` — one entry per
	// marked cell. Playwright specs import this instead of hardcoding
	// the positions, so regenerating the fixture also updates the test
	// input in lockstep.
	solPath := filepath.Join(dir, base+".solution.json")
	solPairs := make([][2]int, 0, len(p.Solution))
	for _, m := range p.Solution {
		solPairs = append(solPairs, [2]int{m.Row, m.Col})
	}
	solData, err := json.MarshalIndent(solPairs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal solution: %w", err)
	}
	solData = append(solData, '\n')
	if err := os.WriteFile(solPath, solData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", solPath, err)
	}

	return nil
}

// puzzleItem builds the DynamoDB-JSON Item shape. The shape mirrors
// repository.PuzzleRecord's dynamodbav tags so the file drops into
// `awslocal dynamodb put-item --item file://<path>` unchanged.
//
// Kept as a hand-written builder (rather than going through the
// aws-sdk-go-v2 attributevalue package) because it's only ~20 fields,
// we control every one, and the resulting JSON is what a human reading
// the committed fixture will see.
func puzzleItem(pk, sk string, seed int64, p *generator.Puzzle, solution [][]bool) map[string]any {
	return map[string]any{
		"PK":                   s(pk),
		"SK":                   s(sk),
		"status":               s("ready"),
		"verdict":              s("none"),
		"regionMap":            listOfListOfInt(p.Regions),
		"solution":             listOfListOfBool(solution),
		"difficulty":           n(int(p.Difficulty)),
		"maxTier":              n(p.Metrics.MaxTier),
		"tierCounts":           listOfInt(p.Metrics.TierCounts),
		"traceLen":             n(p.Metrics.TraceLen),
		"generationDurationMs": n64(0), // fixtures ignore wall-clock
		"createdAt":            s("2026-01-01T00:00:00Z"),
		"seed":                 n64(seed),
	}
}

// DynamoDB-JSON helpers. Each value is wrapped in its type marker
// ({"S": ...}, {"N": ...}, {"BOOL": ...}, {"L": [...]}).

func s(v string) map[string]string { return map[string]string{"S": v} }
func n(v int) map[string]string    { return map[string]string{"N": strconv.Itoa(v)} }
func n64(v int64) map[string]string {
	return map[string]string{"N": strconv.FormatInt(v, 10)}
}
func b(v bool) map[string]bool { return map[string]bool{"BOOL": v} }

func listOfInt(xs []int) map[string]any {
	items := make([]map[string]string, len(xs))
	for i, x := range xs {
		items[i] = n(x)
	}
	return map[string]any{"L": items}
}

func listOfListOfInt(xs [][]int) map[string]any {
	items := make([]map[string]any, len(xs))
	for i, row := range xs {
		items[i] = listOfInt(row)
	}
	return map[string]any{"L": items}
}

func listOfBool(xs []bool) map[string]any {
	items := make([]map[string]bool, len(xs))
	for i, x := range xs {
		items[i] = b(x)
	}
	return map[string]any{"L": items}
}

func listOfListOfBool(xs [][]bool) map[string]any {
	items := make([]map[string]any, len(xs))
	for i, row := range xs {
		items[i] = listOfBool(row)
	}
	return map[string]any{"L": items}
}
