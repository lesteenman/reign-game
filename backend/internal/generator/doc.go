// Package generator is the Phase 5 replacement for backend/internal/generator.
//
// It is currently located under the v2/ subdirectory alongside the legacy
// package so the two can coexist while R-063..R-066 build out the algorithm.
// The v2 suffix vanishes at cutover (R-067), at which point this package moves
// up to backend/internal/generator/ and the old files are deleted.
//
// The package is self-contained per INV-GEN-1: it does not import from
// backend/internal/model, backend/internal/repository, backend/internal/queue,
// backend/internal/handler, or backend/internal/worker. Consumers translate
// generator.Puzzle into storage and transport shapes themselves.
//
// A *Generator is not safe for concurrent use. Each goroutine that generates
// puzzles must construct and own its own *Generator.
package generator
