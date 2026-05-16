// Package replenish coordinates the puzzle-pool top-up logic shared by
// the admin sweep endpoint and the reactive triggers on read paths.
//
// Two entry points:
//
//   - Sweep: full-iteration "scan every enabled combo, publish deficit
//     where below threshold." Used by POST /api/admin/replenish. The
//     caller (HTTP handler) is synchronous and serialises the per-combo
//     results back to JSON.
//
//   - TryReactiveTopUp: single-combo "try to claim a per-combo debounce
//     window, publish a fixed batch on success." Used by drain-site
//     goroutines (practice serve, daily candidate, daily sync-fallback).
//     The caller is async; first publisher error returns up so the
//     goroutine can log it once.
//
// See openspec/changes/auto-replenish-puzzle-pool/design.md for the
// locked design decisions behind this surface.
package replenish

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/queue"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// DefaultWindow is the default per-combo debounce duration for reactive
// top-ups. Picked to match the order-of-magnitude of one generator
// cycle for a 9x9 Standard puzzle.
const DefaultWindow = 60 * time.Second

// DefaultGoroutineTimeout caps the SQS publish goroutine launched per
// drain-site invocation, so the goroutine doesn't sit idle when the
// Lambda runtime is about to freeze.
const DefaultGoroutineTimeout = 2 * time.Second

// ErrSkippedDebounced is the sentinel returned by TryReactiveTopUp when
// another concurrent request already claimed the per-combo debounce
// window. Callers (drain-site goroutines) treat it as a quiet skip and
// do not log it as an error.
var ErrSkippedDebounced = errors.New("replenish: debounced")

// AllConfigsLister enumerates every CONFIG record. Used by Sweep.
type AllConfigsLister interface {
	GetAllConfigs(ctx context.Context) ([]configsvc.ConfigView, error)
}

// ConfigGetter looks up a single CONFIG record by size and mode. Used
// by TryReactiveTopUp on the reactive hot path so we don't pay the
// list-all cost on every drain.
type ConfigGetter interface {
	GetConfig(ctx context.Context, size int, mode string) (*configsvc.ConfigView, error)
}

// PoolCounter counts ready puzzles for a given size+mode partition.
// Used only by Sweep — TryReactiveTopUp deliberately skips the count.
type PoolCounter interface {
	CountReady(ctx context.Context, size int, mode string) (int, error)
}

// MessagePublisher publishes generation requests to the SQS queue.
type MessagePublisher interface {
	PublishGenerationRequest(ctx context.Context, req *queue.GenerationRequest) error
}

// AutoReplenishClaimer coordinates concurrent reactive top-ups via a
// per-combo debounce window held on the matching CONFIG record.
// Implemented by *repository.PuzzleRepository.TryClaimAutoReplenish.
type AutoReplenishClaimer interface {
	TryClaimAutoReplenish(ctx context.Context, size int, mode string, now time.Time, window time.Duration) (bool, error)
}

// SweepDeps is the dependency bundle for Sweep.
type SweepDeps struct {
	Configs   AllConfigsLister
	Counter   PoolCounter
	Publisher MessagePublisher
}

// ReactiveDeps is the dependency bundle for TryReactiveTopUp. Distinct
// from SweepDeps because the reactive path needs Claimer but not Counter,
// and Sweep needs Counter but not Claimer.
type ReactiveDeps struct {
	Configs   ConfigGetter
	Claimer   AutoReplenishClaimer
	Publisher MessagePublisher
	// Window is the per-combo debounce duration. Default DefaultWindow
	// (60s) when zero; tunable at construction time.
	Window time.Duration
	// Clock returns the current time. Injectable for tests.
	Clock func() time.Time
}

// Filter narrows a Sweep to a specific size and/or mode. Zero values
// match everything (matching the existing handler semantics).
type Filter struct {
	Size int
	Mode string
}

// TriggeredEntry is one combo where Sweep published generation requests.
type TriggeredEntry struct {
	Size  int    `json:"size"`
	Mode  string `json:"mode"`
	Count int    `json:"count"`
}

// SkippedEntry is one combo whose ready count is at or above threshold.
type SkippedEntry struct {
	Size  int    `json:"size"`
	Mode  string `json:"mode"`
	Ready int    `json:"ready"`
}

// Result is the per-combo outcome of a Sweep.
type Result struct {
	Triggered []TriggeredEntry `json:"triggered"`
	Skipped   []SkippedEntry   `json:"skipped"`
}

// Sweep reads every enabled combo (optionally filtered by size and/or
// mode), counts each, and publishes (threshold - count) generation
// requests for combos below threshold. Returns the per-combo outcome.
//
// On the first publisher error mid-sweep the function returns the
// partial Result accumulated so far and the wrapped error — matches
// the prior handler behaviour, which surfaces a 500 to the admin.
func Sweep(ctx context.Context, deps SweepDeps, filter Filter) (Result, error) {
	allConfigs, err := deps.Configs.GetAllConfigs(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("listing configs: %w", err)
	}

	result := Result{
		Triggered: []TriggeredEntry{},
		Skipped:   []SkippedEntry{},
	}

	for i := range allConfigs {
		cfg := allConfigs[i]
		if !cfg.Enabled {
			continue
		}
		if filter.Size != 0 && cfg.Size != filter.Size {
			continue
		}
		if filter.Mode != "" && cfg.Mode != filter.Mode {
			continue
		}

		count, err := deps.Counter.CountReady(ctx, cfg.Size, cfg.Mode)
		if err != nil {
			return result, fmt.Errorf("counting ready for %d#%s: %w", cfg.Size, cfg.Mode, err)
		}

		if count >= cfg.Threshold {
			result.Skipped = append(result.Skipped, SkippedEntry{
				Size:  cfg.Size,
				Mode:  cfg.Mode,
				Ready: count,
			})
			continue
		}

		needed := cfg.Threshold - count
		for j := 0; j < needed; j++ {
			req := &queue.GenerationRequest{
				Size:        cfg.Size,
				Mode:        cfg.Mode,
				MaxAttempts: cfg.MaxAttempts,
			}
			if err := deps.Publisher.PublishGenerationRequest(ctx, req); err != nil {
				return result, fmt.Errorf("publishing generation request for %d#%s: %w", cfg.Size, cfg.Mode, err)
			}
		}

		result.Triggered = append(result.Triggered, TriggeredEntry{
			Size:  cfg.Size,
			Mode:  cfg.Mode,
			Count: needed,
		})
	}

	return result, nil
}

// TryReactiveTopUp performs the single-combo reactive top-up:
//  1. Look up the CONFIG row. Missing or disabled -> nil (no-op).
//  2. Try to claim the per-combo debounce window via Claimer.
//     - Loser -> ErrSkippedDebounced (caller treats as quiet skip).
//     - Error -> propagated.
//  3. Publish exactly Threshold generation requests. Generator workers'
//     consume-time threshold check bounds over-generation server-side
//     (skip-count, fixed-batch).
//  4. First publisher error -> return wrapped err. The caller is the
//     drain-site goroutine and logs it once.
func TryReactiveTopUp(ctx context.Context, deps ReactiveDeps, size int, mode string) error {
	cfg, err := deps.Configs.GetConfig(ctx, size, mode)
	if err != nil {
		return fmt.Errorf("loading config for %d#%s: %w", size, mode, err)
	}
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	claimed, err := deps.Claimer.TryClaimAutoReplenish(ctx, size, mode, deps.Clock(), deps.Window)
	if err != nil {
		return fmt.Errorf("claiming auto-replenish window for %d#%s: %w", size, mode, err)
	}
	if !claimed {
		return ErrSkippedDebounced
	}

	for j := 0; j < cfg.Threshold; j++ {
		req := &queue.GenerationRequest{
			Size:        size,
			Mode:        mode,
			MaxAttempts: cfg.MaxAttempts,
		}
		if err := deps.Publisher.PublishGenerationRequest(ctx, req); err != nil {
			return fmt.Errorf("publishing generation request for %d#%s: %w", size, mode, err)
		}
	}
	return nil
}

// NewAsyncHook returns the drain-site closure that dispatches a
// goroutine running TryReactiveTopUp under DefaultGoroutineTimeout.
// The goroutine logs every error except ErrSkippedDebounced, prefixed
// with `logPrefix` so operators can tell which binary fired the hook.
//
// Returns nil when any of Configs, Claimer, or Publisher is nil, which
// matches local-dev runs without SQS — drain handlers must be
// nil-tolerant so the player path keeps working.
//
// Window and Clock fall back to DefaultWindow / time.Now when zero.
func NewAsyncHook(deps ReactiveDeps, logPrefix string) func(size int, mode string) {
	if deps.Configs == nil || deps.Claimer == nil || deps.Publisher == nil {
		return nil
	}
	if deps.Window == 0 {
		deps.Window = DefaultWindow
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	if logPrefix == "" {
		logPrefix = "reactive replenish"
	}
	return func(size int, mode string) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), DefaultGoroutineTimeout)
			defer cancel()
			if err := TryReactiveTopUp(ctx, deps, size, mode); err != nil && !errors.Is(err, ErrSkippedDebounced) {
				log.Printf("%s: %d#%s: %v", logPrefix, size, mode, err)
			}
		}()
	}
}
