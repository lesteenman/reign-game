package pack_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/pack"
)

type fakeStore struct {
	getPack       func(ctx context.Context, slug string) (*repository.PackRecord, error)
	listPacks     func(ctx context.Context) ([]repository.PackRecord, error)
	putPack       func(ctx context.Context, p *repository.PackRecord) error
	updatePack    func(ctx context.Context, p *repository.PackRecord) error
	deletePack    func(ctx context.Context, slug string) error
	getPuzzle     func(ctx context.Context, size int, mode, id string) (*repository.PuzzleRecord, error)
	batchGet      func(ctx context.Context, size int, mode string, ids []string) ([]repository.PuzzleRecord, error)
	listApproved  func(ctx context.Context, size int, mode string, excludeRecent bool, now time.Time) ([]repository.PuzzleRecord, error)
	lastPutRecord *repository.PackRecord
	lastUpdateRec *repository.PackRecord
}

func (f *fakeStore) GetPack(ctx context.Context, slug string) (*repository.PackRecord, error) {
	return f.getPack(ctx, slug)
}
func (f *fakeStore) ListPacks(ctx context.Context) ([]repository.PackRecord, error) {
	return f.listPacks(ctx)
}
func (f *fakeStore) PutPack(ctx context.Context, p *repository.PackRecord) error {
	f.lastPutRecord = p
	return f.putPack(ctx, p)
}
func (f *fakeStore) UpdatePack(ctx context.Context, p *repository.PackRecord) error {
	f.lastUpdateRec = p
	return f.updatePack(ctx, p)
}
func (f *fakeStore) DeletePack(ctx context.Context, slug string) error {
	return f.deletePack(ctx, slug)
}
func (f *fakeStore) GetPuzzle(ctx context.Context, size int, mode, id string) (*repository.PuzzleRecord, error) {
	return f.getPuzzle(ctx, size, mode, id)
}
func (f *fakeStore) ListApprovedPool(ctx context.Context, size int, mode string, excludeRecent bool, now time.Time) ([]repository.PuzzleRecord, error) {
	return f.listApproved(ctx, size, mode, excludeRecent, now)
}
func (f *fakeStore) BatchGetPuzzles(ctx context.Context, size int, mode string, ids []string) ([]repository.PuzzleRecord, error) {
	return f.batchGet(ctx, size, mode, ids)
}

// verdictUpPuzzle is a puzzle that passes the verdict-up gate.
func verdictUpPuzzle(id string) *repository.PuzzleRecord {
	return &repository.PuzzleRecord{
		ID:             id,
		VerdictSummary: repository.VerdictSummary{Up: 1, Down: 0},
	}
}

func baseStore() *fakeStore {
	return &fakeStore{
		getPack:    func(_ context.Context, _ string) (*repository.PackRecord, error) { return nil, nil },
		listPacks:  func(_ context.Context) ([]repository.PackRecord, error) { return nil, nil },
		putPack:    func(_ context.Context, _ *repository.PackRecord) error { return nil },
		updatePack: func(_ context.Context, _ *repository.PackRecord) error { return nil },
		deletePack: func(_ context.Context, _ string) error { return nil },
		getPuzzle: func(_ context.Context, _ int, _ string, id string) (*repository.PuzzleRecord, error) {
			return verdictUpPuzzle(id), nil
		},
		listApproved: func(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
			return nil, nil
		},
		batchGet: func(_ context.Context, _ int, _ string, _ []string) ([]repository.PuzzleRecord, error) {
			return nil, nil
		},
	}
}

// servePuzzle is a stored puzzle row carrying the fields ServePack
// projects (regionMap + metadata) plus a solution that must NOT leak.
func servePuzzle(id string) repository.PuzzleRecord {
	return repository.PuzzleRecord{
		ID:                   id,
		GridSize:             7,
		Mode:                 "standard",
		RegionMap:            [][]int{{0, 1}, {1, 0}},
		Solution:             [][]bool{{true, false}, {false, true}},
		Difficulty:           3,
		MaxTier:              2,
		TierCounts:           []int{0, 1, 2, 0, 0},
		TraceLen:             4,
		GenerationDurationMs: 120,
		CreatedAt:            "2026-06-14T00:00:00Z",
		Seed:                 42,
	}
}

func TestServePack_PublishedHappyPath(t *testing.T) {
	// Arrange — a published pack with two puzzles in a fixed order.
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{
			Slug: "starter", Name: "Starter", Size: 7, Mode: "standard",
			Published: true, PuzzleIDs: []string{"id-1", "id-2"},
		}, nil
	}
	store.batchGet = func(_ context.Context, _ int, _ string, _ []string) ([]repository.PuzzleRecord, error) {
		// Returned out of pack order — ServePack must reorder.
		return []repository.PuzzleRecord{servePuzzle("id-2"), servePuzzle("id-1")}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.ServePack(context.Background(), "starter")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != "starter" || got.Name != "Starter" || got.Size != 7 || got.Mode != "standard" {
		t.Errorf("manifest = %+v", got)
	}
	if got.PuzzleCount != 2 {
		t.Errorf("PuzzleCount = %d, want 2", got.PuzzleCount)
	}
	if len(got.Puzzles) != 2 {
		t.Fatalf("len(Puzzles) = %d, want 2", len(got.Puzzles))
	}
	// Order preserved per the pack's PuzzleIDs, not the BatchGet order.
	if got.Puzzles[0].ID != "id-1" || got.Puzzles[1].ID != "id-2" {
		t.Errorf("order = %q,%q, want id-1,id-2", got.Puzzles[0].ID, got.Puzzles[1].ID)
	}
	// Metadata is carried through.
	if got.Puzzles[0].Difficulty != 3 || got.Puzzles[0].Seed != 42 || got.Puzzles[0].TraceLen != 4 {
		t.Errorf("metadata not carried: %+v", got.Puzzles[0])
	}
}

func TestServePack_Draft_NotFound(t *testing.T) {
	// Arrange — an unpublished pack must be indistinguishable from missing.
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{Slug: "draft", Size: 7, Mode: "standard", Published: false, PuzzleIDs: []string{"id-1"}}, nil
	}
	batchCalled := false
	store.batchGet = func(_ context.Context, _ int, _ string, _ []string) ([]repository.PuzzleRecord, error) {
		batchCalled = true
		return nil, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	_, err := svc.ServePack(context.Background(), "draft")

	// Assert
	if !errors.Is(err, pack.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if batchCalled {
		t.Error("must not read puzzles for a draft pack")
	}
}

func TestServePack_Unknown_NotFound(t *testing.T) {
	// Arrange — absent slug.
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) { return nil, nil }
	svc := pack.New(store, time.Now)

	// Act
	_, err := svc.ServePack(context.Background(), "missing")

	// Assert
	if !errors.Is(err, pack.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestServePack_SkipsAbsentPuzzle_PreservesOrder(t *testing.T) {
	// Arrange — pack references three ids; the middle one is absent.
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{
			Slug: "p", Size: 7, Mode: "standard", Published: true,
			PuzzleIDs: []string{"id-1", "gone", "id-3"},
		}, nil
	}
	store.batchGet = func(_ context.Context, _ int, _ string, _ []string) ([]repository.PuzzleRecord, error) {
		return []repository.PuzzleRecord{servePuzzle("id-3"), servePuzzle("id-1")}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.ServePack(context.Background(), "p")

	// Assert — absent id dropped, remaining served in pack order.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Puzzles) != 2 {
		t.Fatalf("len = %d, want 2", len(got.Puzzles))
	}
	if got.Puzzles[0].ID != "id-1" || got.Puzzles[1].ID != "id-3" {
		t.Errorf("order = %q,%q, want id-1,id-3", got.Puzzles[0].ID, got.Puzzles[1].ID)
	}
	// PuzzleCount reflects the manifest count, not the served count.
	if got.PuzzleCount != 3 {
		t.Errorf("PuzzleCount = %d, want 3 (manifest size)", got.PuzzleCount)
	}
}

func TestServePack_EmptyPublished(t *testing.T) {
	// Arrange — published pack with no puzzles.
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{Slug: "p", Size: 7, Mode: "standard", Published: true, PuzzleIDs: nil}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.ServePack(context.Background(), "p")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Puzzles == nil {
		t.Error("Puzzles must be a non-nil empty slice")
	}
	if len(got.Puzzles) != 0 {
		t.Errorf("len = %d, want 0", len(got.Puzzles))
	}
}

func TestServePack_PureRead(t *testing.T) {
	// Arrange — a published pack. The fake store records any mutation
	// call; ServePack must never trigger one. (MarkServed / UpdateStatus
	// / replenish are not even on the Store interface — assert via the
	// methods that exist: only GetPack + BatchGetPuzzles run.)
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{Slug: "p", Size: 7, Mode: "standard", Published: true, PuzzleIDs: []string{"id-1"}}, nil
	}
	store.getPuzzle = func(_ context.Context, _ int, _ string, _ string) (*repository.PuzzleRecord, error) {
		t.Error("ServePack must not call GetPuzzle (use BatchGetPuzzles)")
		return nil, nil
	}
	store.batchGet = func(_ context.Context, _ int, _ string, _ []string) ([]repository.PuzzleRecord, error) {
		return []repository.PuzzleRecord{servePuzzle("id-1")}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	_, err := svc.ServePack(context.Background(), "p")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPublished_ExcludesDrafts(t *testing.T) {
	// Arrange — mix of published and draft packs.
	store := baseStore()
	store.listPacks = func(_ context.Context) ([]repository.PackRecord, error) {
		return []repository.PackRecord{
			{Slug: "pub-a", Name: "A", Size: 7, Mode: "standard", Published: true, PuzzleIDs: []string{"x", "y"}},
			{Slug: "draft-b", Name: "B", Size: 9, Mode: "double", Published: false, PuzzleIDs: []string{"z"}},
			{Slug: "pub-c", Name: "C", Size: 9, Mode: "double", Published: true, PuzzleIDs: nil},
		}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.ListPublished(context.Background())

	// Assert — only the two published packs, in summary shape.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (drafts excluded)", len(got))
	}
	for _, s := range got {
		if s.Slug == "draft-b" {
			t.Error("draft pack leaked into ListPublished")
		}
	}
	// PuzzleCount derived from the manifest length.
	if got[0].Slug == "pub-a" && got[0].PuzzleCount != 2 {
		t.Errorf("pub-a PuzzleCount = %d, want 2", got[0].PuzzleCount)
	}
}

func TestCreate_HappyPath(t *testing.T) {
	// Arrange
	store := baseStore()
	svc := pack.New(store, func() time.Time { return time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC) })

	// Act
	err := svc.Create(context.Background(), &pack.CreateInput{
		Slug: "starter-pack", Name: "Starter", Size: 7, Mode: "standard",
		PuzzleIDs: []string{"id-1", "id-2"}, Published: false,
	})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastPutRecord == nil {
		t.Fatal("PutPack was not called")
	}
	if store.lastPutRecord.Slug != "starter-pack" || store.lastPutRecord.Size != 7 {
		t.Errorf("record = %+v", store.lastPutRecord)
	}
	if store.lastPutRecord.CreatedAt == "" || store.lastPutRecord.UpdatedAt == "" {
		t.Error("timestamps not stamped")
	}
}

func TestCreate_RejectsInvalidSlug(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"uppercase", "Starter"},
		{"spaces", "my pack"},
		{"leading dash", "-pack"},
		{"trailing dash", "pack-"},
		{"double dash", "my--pack"},
		{"empty", ""},
		{"underscore", "my_pack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := baseStore()
			svc := pack.New(store, time.Now)

			// Act
			err := svc.Create(context.Background(), &pack.CreateInput{
				Slug: tt.slug, Name: "X", Size: 7, Mode: "standard",
			})

			// Assert
			if !errors.Is(err, pack.ErrInvalidSlug) {
				t.Errorf("err = %v, want ErrInvalidSlug", err)
			}
			if store.lastPutRecord != nil {
				t.Error("PutPack must not be called on invalid slug")
			}
		})
	}
}

func TestCreate_RejectsTooLongSlug(t *testing.T) {
	// Arrange
	store := baseStore()
	svc := pack.New(store, time.Now)
	long := ""
	for i := 0; i < 80; i++ {
		long += "a"
	}

	// Act
	err := svc.Create(context.Background(), &pack.CreateInput{Slug: long, Name: "X", Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, pack.ErrInvalidSlug) {
		t.Errorf("err = %v, want ErrInvalidSlug", err)
	}
}

func TestCreate_RejectsInvalidCombo(t *testing.T) {
	tests := []struct {
		name string
		size int
		mode string
	}{
		{"bad mode", 7, "triple"},
		{"size too small", 2, "standard"},
		{"size too large", 99, "standard"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := baseStore()
			svc := pack.New(store, time.Now)

			// Act
			err := svc.Create(context.Background(), &pack.CreateInput{
				Slug: "p", Name: "X", Size: tt.size, Mode: tt.mode,
			})

			// Assert
			if !errors.Is(err, pack.ErrInvalidCombo) {
				t.Errorf("err = %v, want ErrInvalidCombo", err)
			}
		})
	}
}

func TestCreate_RejectsNonVerdictUpPuzzle(t *testing.T) {
	tests := []struct {
		name   string
		puzzle *repository.PuzzleRecord
	}{
		{"down-voted", &repository.PuzzleRecord{ID: "bad", VerdictSummary: repository.VerdictSummary{Up: 2, Down: 1}}},
		{"no up votes", &repository.PuzzleRecord{ID: "bad", VerdictSummary: repository.VerdictSummary{Up: 0, Down: 0}}},
		{"missing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			store := baseStore()
			store.getPuzzle = func(_ context.Context, _ int, _ string, id string) (*repository.PuzzleRecord, error) {
				if id == "bad" {
					return tt.puzzle, nil
				}
				return verdictUpPuzzle(id), nil
			}
			svc := pack.New(store, time.Now)

			// Act
			err := svc.Create(context.Background(), &pack.CreateInput{
				Slug: "p", Name: "X", Size: 7, Mode: "standard",
				PuzzleIDs: []string{"good", "bad"},
			})

			// Assert
			var invalid *pack.InvalidPuzzleError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v (%T), want InvalidPuzzleError", err, err)
			}
			if invalid.PuzzleID != "bad" {
				t.Errorf("offending id = %q, want %q", invalid.PuzzleID, "bad")
			}
			if store.lastPutRecord != nil {
				t.Error("PutPack must not be called when validation fails")
			}
		})
	}
}

func TestCreate_AllowsEmptyDraft(t *testing.T) {
	// Arrange
	store := baseStore()
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Create(context.Background(), &pack.CreateInput{
		Slug: "empty", Name: "X", Size: 7, Mode: "standard", PuzzleIDs: nil, Published: false,
	})

	// Assert
	if err != nil {
		t.Fatalf("empty draft should be allowed, got %v", err)
	}
}

func TestCreate_RejectsPublishingEmptyPack(t *testing.T) {
	// Arrange
	store := baseStore()
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Create(context.Background(), &pack.CreateInput{
		Slug: "empty", Name: "X", Size: 7, Mode: "standard", PuzzleIDs: nil, Published: true,
	})

	// Assert
	if !errors.Is(err, pack.ErrEmptyPublish) {
		t.Errorf("err = %v, want ErrEmptyPublish", err)
	}
}

func TestCreate_MapsAlreadyExists(t *testing.T) {
	// Arrange
	store := baseStore()
	store.putPack = func(_ context.Context, _ *repository.PackRecord) error { return repository.ErrPackAlreadyExists }
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Create(context.Background(), &pack.CreateInput{Slug: "dup", Name: "X", Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, pack.ErrAlreadyExists) {
		t.Errorf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestUpdate_HappyPath(t *testing.T) {
	// Arrange
	store := baseStore()
	existing := &repository.PackRecord{Slug: "p", Name: "Old", Size: 7, Mode: "standard", CreatedAt: "2026-01-01T00:00:00Z"}
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) { return existing, nil }
	svc := pack.New(store, func() time.Time { return time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC) })

	// Act
	err := svc.Update(context.Background(), "p", pack.UpdateInput{
		Name: "New", PuzzleIDs: []string{"id-1"}, Published: true,
	})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastUpdateRec == nil {
		t.Fatal("UpdatePack was not called")
	}
	if store.lastUpdateRec.Name != "New" || !store.lastUpdateRec.Published {
		t.Errorf("record = %+v", store.lastUpdateRec)
	}
	// Size+mode resolved from the existing record, not the input.
	if store.lastUpdateRec.Size != 7 || store.lastUpdateRec.Mode != "standard" {
		t.Errorf("size/mode = %d/%s, want 7/standard", store.lastUpdateRec.Size, store.lastUpdateRec.Mode)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	// Arrange
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) { return nil, nil }
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Update(context.Background(), "missing", pack.UpdateInput{Name: "x"})

	// Assert
	if !errors.Is(err, pack.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if store.lastUpdateRec != nil {
		t.Error("UpdatePack must not be called when pack is absent")
	}
}

func TestUpdate_RevalidatesPuzzles(t *testing.T) {
	// Arrange
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{Slug: "p", Size: 7, Mode: "standard"}, nil
	}
	store.getPuzzle = func(_ context.Context, _ int, _ string, id string) (*repository.PuzzleRecord, error) {
		if id == "bad" {
			return &repository.PuzzleRecord{ID: "bad", VerdictSummary: repository.VerdictSummary{Down: 1}}, nil
		}
		return verdictUpPuzzle(id), nil
	}
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Update(context.Background(), "p", pack.UpdateInput{PuzzleIDs: []string{"bad"}})

	// Assert
	var invalid *pack.InvalidPuzzleError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want InvalidPuzzleError", err)
	}
}

func TestUpdate_RejectsPublishingEmptyPack(t *testing.T) {
	// Arrange
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) {
		return &repository.PackRecord{Slug: "p", Size: 7, Mode: "standard"}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Update(context.Background(), "p", pack.UpdateInput{PuzzleIDs: nil, Published: true})

	// Assert
	if !errors.Is(err, pack.ErrEmptyPublish) {
		t.Errorf("err = %v, want ErrEmptyPublish", err)
	}
}

func TestGet(t *testing.T) {
	// Arrange
	store := baseStore()
	store.getPack = func(_ context.Context, slug string) (*repository.PackRecord, error) {
		return &repository.PackRecord{Slug: slug, Name: "P", Size: 7, Mode: "standard"}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.Get(context.Background(), "p")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Slug != "p" {
		t.Errorf("got = %+v", got)
	}
}

func TestGet_NotFound(t *testing.T) {
	// Arrange
	store := baseStore()
	store.getPack = func(_ context.Context, _ string) (*repository.PackRecord, error) { return nil, nil }
	svc := pack.New(store, time.Now)

	// Act
	_, err := svc.Get(context.Background(), "missing")

	// Assert
	if !errors.Is(err, pack.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	// Arrange
	store := baseStore()
	store.listPacks = func(_ context.Context) ([]repository.PackRecord, error) {
		return []repository.PackRecord{{Slug: "a"}, {Slug: "b"}}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.List(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestDelete(t *testing.T) {
	// Arrange
	var deletedSlug string
	store := baseStore()
	store.deletePack = func(_ context.Context, slug string) error { deletedSlug = slug; return nil }
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Delete(context.Background(), "p")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedSlug != "p" {
		t.Errorf("deleted slug = %q, want p", deletedSlug)
	}
}

func TestDelete_NotFound(t *testing.T) {
	// Arrange
	store := baseStore()
	store.deletePack = func(_ context.Context, _ string) error { return repository.ErrPackNotFound }
	svc := pack.New(store, time.Now)

	// Act
	err := svc.Delete(context.Background(), "missing")

	// Assert
	if !errors.Is(err, pack.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListCandidates(t *testing.T) {
	// Arrange
	store := baseStore()
	store.listApproved = func(_ context.Context, size int, mode string, excludeRecent bool, _ time.Time) ([]repository.PuzzleRecord, error) {
		if excludeRecent {
			t.Error("candidates must call ListApprovedPool with excludeRecentlyDailied=false")
		}
		return []repository.PuzzleRecord{
			{ID: "id-1", GridSize: size, Mode: mode, RegionMap: [][]int{{0}}},
		}, nil
	}
	svc := pack.New(store, time.Now)

	// Act
	got, err := svc.ListCandidates(context.Background(), 7, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "id-1" {
		t.Errorf("got = %+v", got)
	}
}

func TestListCandidates_RejectsInvalidCombo(t *testing.T) {
	// Arrange
	store := baseStore()
	svc := pack.New(store, time.Now)

	// Act
	_, err := svc.ListCandidates(context.Background(), 7, "triple")

	// Assert
	if !errors.Is(err, pack.ErrInvalidCombo) {
		t.Errorf("err = %v, want ErrInvalidCombo", err)
	}
}
