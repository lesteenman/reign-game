// Package pool is the application service for the admin pool status view.
// It aggregates all CONFIG rows with per-combo ready-puzzle counts and
// emits per-step DDB timing observability, keeping the handler a thin
// HTTP shell.
package pool

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// countConcurrency bounds how many per-combo CountReady queries run in
// parallel during a LoadPool. Small enough to stay well under DynamoDB
// connection limits, large enough to collapse the admin pool's cold-start
// fan-out from O(combos) serial round-trips to one bounded wave.
const countConcurrency = 8

// Store is the persistence surface used by Service.
type Store interface {
	GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
	CountReady(ctx context.Context, size int, mode string) (int, error)
}

// ComboEntry is the service-level result for one size+mode combination.
// The handler maps this to its ComboStatus DTO.
type ComboEntry struct {
	Config     repository.ConfigRecord
	ReadyCount int
}

// Service aggregates config rows with ready counts for the admin pool view.
type Service struct {
	store Store
}

// New constructs a Service.
func New(store Store) *Service {
	return &Service{store: store}
}

// LoadPool fetches all configs and, for each enabled combo, the ready
// puzzle count. Disabled combos are included with ReadyCount=0.
//
// Logs per-step DDB timing on every call:
//
//	admin pool: total_ms=N configs_ms=N combos=N count_breakdown=[...]
//
// Each count_breakdown entry is "<size>#<mode>=<ms>ms"; disabled combos
// show -1ms as a sentinel for "skipped".
func (s *Service) LoadPool(ctx context.Context) ([]ComboEntry, error) {
	totalStart := time.Now()

	configsStart := time.Now()
	configs, err := s.store.GetAllConfigs(ctx)
	configsMs := time.Since(configsStart).Milliseconds()
	if err != nil {
		log.Printf("admin pool: failed to get configs: %v configs_ms=%d", err, configsMs)
		return nil, fmt.Errorf("loading configs: %w", err)
	}

	// Count each enabled combo concurrently (bounded), writing results by
	// index so the entries + timing breakdown stay in config order
	// regardless of goroutine completion order. Disabled combos are filled
	// in directly with ReadyCount=0 and a -1ms "skipped" sentinel — no query.
	entries := make([]ComboEntry, len(configs))
	countMsByIdx := make([]int64, len(configs))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(countConcurrency)
	for i := range configs {
		cfg := configs[i]
		entries[i] = ComboEntry{Config: cfg, ReadyCount: 0}
		if !cfg.Enabled {
			countMsByIdx[i] = -1 // sentinel for "skipped (config disabled)"
			continue
		}
		g.Go(func() error {
			countStart := time.Now()
			readyCount, cerr := s.store.CountReady(gctx, cfg.Size, cfg.Mode)
			countMsByIdx[i] = time.Since(countStart).Milliseconds()
			if cerr != nil {
				log.Printf("admin pool: failed to count ready for %d#%s: %v count_%d#%s_ms=%d", cfg.Size, cfg.Mode, cerr, cfg.Size, cfg.Mode, countMsByIdx[i])
				return fmt.Errorf("counting ready puzzles for %d#%s: %w", cfg.Size, cfg.Mode, cerr)
			}
			entries[i].ReadyCount = readyCount
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Each entry: "<size>#<mode>=<ms>ms" — terse so a single log line
	// shows the per-combo breakdown without bloating the format.
	countTimings := make([]string, 0, len(configs))
	for i, cfg := range configs {
		countTimings = append(countTimings, fmt.Sprintf("%d#%s=%dms", cfg.Size, cfg.Mode, countMsByIdx[i]))
	}

	log.Printf(
		"admin pool: total_ms=%d configs_ms=%d combos=%d count_breakdown=[%s]",
		time.Since(totalStart).Milliseconds(),
		configsMs,
		len(configs),
		strings.Join(countTimings, " "),
	)

	return entries, nil
}
