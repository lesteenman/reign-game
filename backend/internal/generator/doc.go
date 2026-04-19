// Package generator produces Reign puzzles (Queens-style row/column/region
// grids with an adjacency constraint). It is the Phase 5 replacement for the
// legacy strategy-matrix pipeline.
//
// The package is self-contained per INV-GEN-1: it does not import from
// backend/internal/model, backend/internal/repository, backend/internal/queue,
// backend/internal/handler, or backend/internal/worker. Consumers translate
// generator.Puzzle into storage and transport shapes themselves.
//
// A *Generator is not safe for concurrent use. Each goroutine that generates
// puzzles must construct and own its own *Generator.
package generator
