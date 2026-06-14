// Package pack is the application service for managing curated puzzle
// packs (the PACK row family) — admin create/update/delete/get/list and
// the verdict-up candidate listing the assembly UI picks from.
package pack

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/mode"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// maxSlugLength bounds a pack slug. Slugs are admin-set identities, not
// free text — a generous cap keeps the PK bounded.
const maxSlugLength = 64

// Size bounds for a pack's grid. Mirrors the handler's accepted range.
const (
	minSize = 3
	maxSize = 15
)

// slugPattern enforces lowercase kebab-case: lowercase alphanumerics in
// dash-separated groups, no leading/trailing/double dashes.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ErrInvalidSlug is returned when a slug is not lowercase kebab-case or
// exceeds the length bound. Handlers map it to 400.
var ErrInvalidSlug = errors.New("invalid pack slug")

// ErrInvalidCombo is returned when size+mode is not a valid combination.
// Handlers map it to 400.
var ErrInvalidCombo = errors.New("invalid size+mode combo")

// ErrAlreadyExists is returned by Create when the slug is taken.
// Handlers map it to 409.
var ErrAlreadyExists = errors.New("pack already exists")

// ErrNotFound is returned by Get/Update/Delete when the slug is absent.
// Handlers map it to 404.
var ErrNotFound = errors.New("pack not found")

// ErrEmptyPublish is returned when a pack with no puzzles is published.
// Handlers map it to 422.
var ErrEmptyPublish = errors.New("cannot publish an empty pack")

// InvalidPuzzleError names a puzzleId that failed write-time validation
// (does not exist in the pack's size+mode partition, or is not
// verdict-up). Handlers map it to 422.
type InvalidPuzzleError struct {
	PuzzleID string
	Reason   string
}

// Error implements the error interface for InvalidPuzzleError.
func (e *InvalidPuzzleError) Error() string {
	return fmt.Sprintf("puzzle %s is invalid: %s", e.PuzzleID, e.Reason)
}

// Store is the persistence surface used by Service.
type Store interface {
	GetPack(ctx context.Context, slug string) (*repository.PackRecord, error)
	ListPacks(ctx context.Context) ([]repository.PackRecord, error)
	PutPack(ctx context.Context, p *repository.PackRecord) error
	UpdatePack(ctx context.Context, p *repository.PackRecord) error
	DeletePack(ctx context.Context, slug string) error
	GetPuzzle(ctx context.Context, size int, mode, id string) (*repository.PuzzleRecord, error)
	BatchGetPuzzles(ctx context.Context, size int, mode string, ids []string) ([]repository.PuzzleRecord, error)
	ListApprovedPool(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]repository.PuzzleRecord, error)
}

// CreateInput is the plain-field input to Service.Create.
type CreateInput struct {
	Slug      string
	Name      string
	Size      int
	Mode      string
	PuzzleIDs []string
	Published bool
}

// UpdateInput is the plain-field input to Service.Update. Identity
// (slug) is supplied separately; size and mode are immutable and read
// from the existing record.
type UpdateInput struct {
	Name      string
	PuzzleIDs []string
	Published bool
}

// PackView is the read-side projection of a pack.
type PackView struct {
	Slug      string
	Name      string
	Size      int
	Mode      string
	Published bool
	PuzzleIDs []string
	CreatedAt string
	UpdatedAt string
}

// PackServeView is the public read projection of a pack ready to play:
// the manifest plus every puzzle's play data in pack order, no solution.
type PackServeView struct {
	Slug        string
	Name        string
	Size        int
	Mode        string
	PuzzleCount int
	Puzzles     []ServePuzzle
}

// ServePuzzle is one puzzle's play data within a served pack — the
// serve-side shape (no solution).
type ServePuzzle struct {
	ID                   string
	RegionMap            [][]int
	Difficulty           int
	MaxTier              int
	TierCounts           []int
	TraceLen             int
	GenerationDurationMs int64
	CreatedAt            string
	Seed                 int64
}

// PackSummary is the list-side projection of a published pack — manifest
// fields only, no puzzles.
type PackSummary struct {
	Slug        string
	Name        string
	Size        int
	Mode        string
	PuzzleCount int
}

// Candidate is a verdict-up puzzle eligible for pack assembly. Carries
// the serve-side shape (no solution).
type Candidate struct {
	ID         string
	RegionMap  [][]int
	Difficulty int
	MaxTier    int
	TierCounts []int
	TraceLen   int
	CreatedAt  string
}

// Service handles pack reads, creates, updates, and deletes.
type Service struct {
	store Store
	now   func() time.Time
}

// New constructs a Service. now supplies the clock for timestamps.
func New(store Store, now func() time.Time) *Service {
	return &Service{store: store, now: now}
}

// Create validates and writes a new pack. Returns ErrInvalidSlug /
// ErrInvalidCombo on malformed input, InvalidPuzzleError when a puzzleId
// is not verdict-up, ErrEmptyPublish when publishing an empty pack, and
// ErrAlreadyExists when the slug is taken.
func (s *Service) Create(ctx context.Context, in *CreateInput) error {
	if !validSlug(in.Slug) {
		return ErrInvalidSlug
	}
	if !validCombo(in.Size, in.Mode) {
		return ErrInvalidCombo
	}
	if in.Published && len(in.PuzzleIDs) == 0 {
		return ErrEmptyPublish
	}
	if err := s.validatePuzzles(ctx, in.Size, in.Mode, in.PuzzleIDs); err != nil {
		return err
	}

	now := s.now().UTC().Format(time.RFC3339)
	record := &repository.PackRecord{
		Slug:      in.Slug,
		Name:      in.Name,
		Size:      in.Size,
		Mode:      in.Mode,
		Published: in.Published,
		PuzzleIDs: in.PuzzleIDs,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.PutPack(ctx, record); err != nil {
		if errors.Is(err, repository.ErrPackAlreadyExists) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("creating pack: %w", err)
	}
	return nil
}

// Update mutates name, ordered puzzleIds, and published on an existing
// pack. Slug, size, and mode are immutable. Re-validates every puzzleId.
// Returns ErrNotFound when the slug is absent.
func (s *Service) Update(ctx context.Context, slug string, in UpdateInput) error {
	existing, err := s.store.GetPack(ctx, slug)
	if err != nil {
		return fmt.Errorf("checking existing pack: %w", err)
	}
	if existing == nil {
		return ErrNotFound
	}
	if in.Published && len(in.PuzzleIDs) == 0 {
		return ErrEmptyPublish
	}
	if err := s.validatePuzzles(ctx, existing.Size, existing.Mode, in.PuzzleIDs); err != nil {
		return err
	}

	record := &repository.PackRecord{
		Slug:      slug,
		Name:      in.Name,
		Size:      existing.Size,
		Mode:      existing.Mode,
		Published: in.Published,
		PuzzleIDs: in.PuzzleIDs,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: s.now().UTC().Format(time.RFC3339),
	}
	if err := s.store.UpdatePack(ctx, record); err != nil {
		if errors.Is(err, repository.ErrPackNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("updating pack: %w", err)
	}
	return nil
}

// Get returns a single pack. Returns ErrNotFound when absent.
func (s *Service) Get(ctx context.Context, slug string) (*PackView, error) {
	rec, err := s.store.GetPack(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("getting pack: %w", err)
	}
	if rec == nil {
		return nil, ErrNotFound
	}
	v := packViewFrom(rec)
	return &v, nil
}

// List returns every pack (drafts included).
func (s *Service) List(ctx context.Context) ([]PackView, error) {
	recs, err := s.store.ListPacks(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing packs: %w", err)
	}
	views := make([]PackView, len(recs))
	for i := range recs {
		views[i] = packViewFrom(&recs[i])
	}
	return views, nil
}

// Delete removes a pack. Returns ErrNotFound when the slug is absent.
func (s *Service) Delete(ctx context.Context, slug string) error {
	if err := s.store.DeletePack(ctx, slug); err != nil {
		if errors.Is(err, repository.ErrPackNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("deleting pack: %w", err)
	}
	return nil
}

// ListCandidates returns the verdict-up puzzles for a size+mode combo,
// the corpus the assembly UI picks pack entries from. Returns
// ErrInvalidCombo on a malformed combo.
func (s *Service) ListCandidates(ctx context.Context, size int, modeName string) ([]Candidate, error) {
	if !validCombo(size, modeName) {
		return nil, ErrInvalidCombo
	}
	recs, err := s.store.ListApprovedPool(ctx, size, modeName, false, s.now())
	if err != nil {
		return nil, fmt.Errorf("listing candidates: %w", err)
	}
	candidates := make([]Candidate, len(recs))
	for i := range recs {
		candidates[i] = Candidate{
			ID:         recs[i].ID,
			RegionMap:  recs[i].RegionMap,
			Difficulty: recs[i].Difficulty,
			MaxTier:    recs[i].MaxTier,
			TierCounts: recs[i].TierCounts,
			TraceLen:   recs[i].TraceLen,
			CreatedAt:  recs[i].CreatedAt,
		}
	}
	return candidates, nil
}

// ServePack returns a published pack's manifest plus every puzzle's
// play data in pack order — the public read for playing a pack. It is a
// pure read: no status transition, no replenish. A draft or absent pack
// returns ErrNotFound (indistinguishable, so drafts are not
// discoverable). Puzzles are fetched in one BatchGetItem round-trip; a
// referenced id with no row is skipped with a WARN and the rest serve
// in pack order (served as-packed; verdict drift is ignored).
func (s *Service) ServePack(ctx context.Context, slug string) (*PackServeView, error) {
	rec, err := s.store.GetPack(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("getting pack: %w", err)
	}
	if rec == nil || !rec.Published {
		return nil, ErrNotFound
	}

	view := &PackServeView{
		Slug:        rec.Slug,
		Name:        rec.Name,
		Size:        rec.Size,
		Mode:        rec.Mode,
		PuzzleCount: len(rec.PuzzleIDs),
		Puzzles:     []ServePuzzle{},
	}
	if len(rec.PuzzleIDs) == 0 {
		return view, nil
	}

	puzzles, err := s.store.BatchGetPuzzles(ctx, rec.Size, rec.Mode, rec.PuzzleIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching pack puzzles: %w", err)
	}

	// Index the fetched rows by id, then walk the pack's ordered ids so
	// the response follows pack order regardless of BatchGetItem's
	// unspecified return order. An id with no row is genuinely absent.
	byID := make(map[string]repository.PuzzleRecord, len(puzzles))
	for i := range puzzles {
		byID[puzzles[i].ID] = puzzles[i]
	}
	for _, id := range rec.PuzzleIDs {
		p, ok := byID[id]
		if !ok {
			log.Printf("WARN: packs serve: pack=%s references absent puzzle id=%s (skipped)", slug, id)
			continue
		}
		view.Puzzles = append(view.Puzzles, ServePuzzle{
			ID:                   p.ID,
			RegionMap:            p.RegionMap,
			Difficulty:           p.Difficulty,
			MaxTier:              p.MaxTier,
			TierCounts:           p.TierCounts,
			TraceLen:             p.TraceLen,
			GenerationDurationMs: p.GenerationDurationMs,
			CreatedAt:            p.CreatedAt,
			Seed:                 p.Seed,
		})
	}
	return view, nil
}

// ListPublished returns the summary of every published pack — manifest
// fields only, no puzzles. Drafts are excluded so they stay
// undiscoverable.
func (s *Service) ListPublished(ctx context.Context) ([]PackSummary, error) {
	recs, err := s.store.ListPacks(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing packs: %w", err)
	}
	summaries := make([]PackSummary, 0, len(recs))
	for i := range recs {
		if !recs[i].Published {
			continue
		}
		summaries = append(summaries, PackSummary{
			Slug:        recs[i].Slug,
			Name:        recs[i].Name,
			Size:        recs[i].Size,
			Mode:        recs[i].Mode,
			PuzzleCount: len(recs[i].PuzzleIDs),
		})
	}
	return summaries, nil
}

// validatePuzzles asserts every puzzleId exists in the pack's size+mode
// partition and is verdict-up. Returns an InvalidPuzzleError naming the
// first offending id.
func (s *Service) validatePuzzles(ctx context.Context, size int, modeName string, ids []string) error {
	for _, id := range ids {
		puzzle, err := s.store.GetPuzzle(ctx, size, modeName, id)
		if err != nil {
			return fmt.Errorf("validating puzzle %s: %w", id, err)
		}
		if puzzle == nil {
			return &InvalidPuzzleError{PuzzleID: id, Reason: "not found in this size+mode"}
		}
		if puzzle.VerdictSummary.Up < 1 || puzzle.VerdictSummary.Down > 0 {
			return &InvalidPuzzleError{PuzzleID: id, Reason: "not verdict-up"}
		}
	}
	return nil
}

// validSlug reports whether s is lowercase kebab-case within bounds.
func validSlug(s string) bool {
	if s == "" || len(s) > maxSlugLength {
		return false
	}
	return slugPattern.MatchString(s)
}

// validCombo reports whether size+mode is a valid combination.
func validCombo(size int, modeName string) bool {
	return size >= minSize && size <= maxSize && mode.IsValid(modeName)
}

// packViewFrom builds a PackView from a repository record.
func packViewFrom(r *repository.PackRecord) PackView {
	return PackView{
		Slug:      r.Slug,
		Name:      r.Name,
		Size:      r.Size,
		Mode:      r.Mode,
		Published: r.Published,
		PuzzleIDs: r.PuzzleIDs,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
