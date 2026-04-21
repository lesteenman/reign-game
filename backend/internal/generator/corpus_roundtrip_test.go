package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCorpusRoundtrip loads every committed testdata/puzzles/*.json and
// asserts the deductive solver still solves it and the brute solver
// still agrees on one unique solution. Runs in the default suite —
// any generator change that breaks a committed corpus member surfaces
// here, which is exactly the point: the corpus is the regression
// canary for generator behavior across difficulty tiers.
func TestCorpusRoundtrip(t *testing.T) {
	t.Parallel()

	corpusDir := filepath.Join("testdata", "puzzles")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("no corpus committed yet — run -tags=corpus to generate")
		}
		t.Fatalf("readdir: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(corpusDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var p Puzzle
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}

		sols, err := bruteSolveAll(p.Regions, p.N, p.MarksPerUnit, 2)
		if err != nil {
			t.Fatalf("%s: brute: %v", path, err)
		}
		if len(sols) != 1 {
			t.Fatalf("%s: brute returned %d solutions", path, len(sols))
		}
		if !marksEqualUnordered(sols[0], p.Solution) {
			t.Fatalf("%s: solution mismatch", path)
		}

		var s solverState
		if err := s.initFromRegionMap(p.Regions, p.N, p.MarksPerUnit); err != nil {
			t.Fatalf("%s: initFromRegionMap: %v", path, err)
		}
		if solve(&s) != OutcomeSolved {
			t.Fatalf("%s: deductive solver no longer solves", path)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no puzzles in testdata/puzzles — corpus not yet generated")
	}
	t.Logf("corpus: %d puzzles checked", checked)
}
