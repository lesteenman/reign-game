package pool_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/pool"
)

type fakeStore struct {
	getAllConfigs func(ctx context.Context) ([]repository.ConfigRecord, error)
	countReady    func(ctx context.Context, size int, mode string) (int, error)
}

func (f *fakeStore) GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error) {
	return f.getAllConfigs(ctx)
}

func (f *fakeStore) CountReady(ctx context.Context, size int, mode string) (int, error) {
	return f.countReady(ctx, size, mode)
}

func TestLoadPool_HappyPath_EnabledCombosGetCount_DisabledSkipped(t *testing.T) {
	// Arrange
	configs := []repository.ConfigRecord{
		{Size: 5, Mode: "standard", Enabled: true},
		{Size: 7, Mode: "double", Enabled: false},
		{Size: 9, Mode: "standard", Enabled: true},
	}
	var countReadyCalls atomic.Int32
	store := &fakeStore{
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) {
			return configs, nil
		},
		countReady: func(_ context.Context, size int, mode string) (int, error) {
			countReadyCalls.Add(1)
			// Return a distinct count per combo so we can verify routing.
			if size == 5 && mode == "standard" {
				return 10, nil
			}
			if size == 9 && mode == "standard" {
				return 3, nil
			}
			return 0, nil
		},
	}
	svc := pool.New(store)

	// Act
	entries, err := svc.LoadPool(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries length = %d, want 3", len(entries))
	}
	// Verify order is preserved.
	if entries[0].Config.Size != 5 || entries[0].Config.Mode != "standard" {
		t.Errorf("entries[0] = %+v, want 5/standard", entries[0].Config)
	}
	if entries[0].ReadyCount != 10 {
		t.Errorf("entries[0].ReadyCount = %d, want 10", entries[0].ReadyCount)
	}
	// Disabled combo: CountReady must NOT have been called, ReadyCount must be 0.
	if entries[1].Config.Size != 7 || entries[1].Config.Mode != "double" {
		t.Errorf("entries[1] = %+v, want 7/double", entries[1].Config)
	}
	if entries[1].ReadyCount != 0 {
		t.Errorf("entries[1].ReadyCount = %d, want 0 for disabled combo", entries[1].ReadyCount)
	}
	if entries[2].ReadyCount != 3 {
		t.Errorf("entries[2].ReadyCount = %d, want 3", entries[2].ReadyCount)
	}
	// Only 2 enabled combos → CountReady called exactly twice.
	if got := countReadyCalls.Load(); got != 2 {
		t.Errorf("CountReady called %d times, want 2 (disabled combo must be skipped)", got)
	}
}

func TestLoadPool_CountReadyCallsRunConcurrently(t *testing.T) {
	// Arrange — three enabled combos. Each CountReady blocks until it has
	// observed at least 2 callers in flight at once, proving the per-combo
	// counts are parallelized (the serial implementation would deadlock the
	// barrier and time out, so a bounded wait drives the assertion).
	configs := []repository.ConfigRecord{
		{Size: 5, Mode: "standard", Enabled: true},
		{Size: 7, Mode: "standard", Enabled: true},
		{Size: 9, Mode: "standard", Enabled: true},
	}
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var wg sync.WaitGroup
	wg.Add(3)
	store := &fakeStore{
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) {
			return configs, nil
		},
		countReady: func(_ context.Context, _ int, _ string) (int, error) {
			cur := inFlight.Add(1)
			for {
				prev := maxInFlight.Load()
				if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
					break
				}
			}
			// Let the other goroutines spin up before returning so the
			// concurrency window is observable.
			wg.Done()
			wg.Wait()
			inFlight.Add(-1)
			return 1, nil
		},
	}
	svc := pool.New(store)

	// Act — guard with a timeout: serial execution can never satisfy the
	// 3-way wg.Wait barrier, so it would hang rather than pass.
	done := make(chan struct{})
	var entries []pool.ComboEntry
	var err error
	go func() {
		entries, err = svc.LoadPool(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("LoadPool did not run CountReady concurrently (barrier timed out — counts are still serial)")
	}

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries length = %d, want 3", len(entries))
	}
	if got := maxInFlight.Load(); got < 2 {
		t.Errorf("max concurrent CountReady = %d, want >= 2 (counts should be parallel)", got)
	}
}

func TestLoadPool_AllDisabled_NoCountReadyCalled(t *testing.T) {
	// Arrange
	configs := []repository.ConfigRecord{
		{Size: 5, Mode: "standard", Enabled: false},
		{Size: 7, Mode: "double", Enabled: false},
	}
	var countReadyCalled atomic.Bool
	store := &fakeStore{
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) {
			return configs, nil
		},
		countReady: func(_ context.Context, _ int, _ string) (int, error) {
			countReadyCalled.Store(true)
			return 0, nil
		},
	}
	svc := pool.New(store)

	// Act
	entries, err := svc.LoadPool(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries length = %d, want 2", len(entries))
	}
	if countReadyCalled.Load() {
		t.Error("CountReady must not be called when all configs are disabled")
	}
	for i, e := range entries {
		if e.ReadyCount != 0 {
			t.Errorf("entries[%d].ReadyCount = %d, want 0", i, e.ReadyCount)
		}
	}
}

func TestLoadPool_GetAllConfigsError_ReturnsWrappedError(t *testing.T) {
	// Arrange
	sentinel := errors.New("dynamo exploded")
	store := &fakeStore{
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) {
			return nil, sentinel
		},
		countReady: func(_ context.Context, _ int, _ string) (int, error) {
			return 0, nil
		},
	}
	svc := pool.New(store)

	// Act
	entries, err := svc.LoadPool(context.Background())

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
	if entries != nil {
		t.Errorf("entries = %+v, want nil on error", entries)
	}
}

func TestLoadPool_CountReadyError_ReturnsWrappedError(t *testing.T) {
	// Arrange
	sentinel := errors.New("count query failed")
	store := &fakeStore{
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) {
			return []repository.ConfigRecord{
				{Size: 5, Mode: "standard", Enabled: true},
			}, nil
		},
		countReady: func(_ context.Context, _ int, _ string) (int, error) {
			return 0, sentinel
		},
	}
	svc := pool.New(store)

	// Act
	entries, err := svc.LoadPool(context.Background())

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
	if entries != nil {
		t.Errorf("entries = %+v, want nil on error", entries)
	}
}

func TestLoadPool_EmptyConfigList_ReturnsEmptySlice(t *testing.T) {
	// Arrange
	store := &fakeStore{
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) {
			return []repository.ConfigRecord{}, nil
		},
		countReady: func(_ context.Context, _ int, _ string) (int, error) {
			return 0, nil
		},
	}
	svc := pool.New(store)

	// Act
	entries, err := svc.LoadPool(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries length = %d, want 0", len(entries))
	}
}
