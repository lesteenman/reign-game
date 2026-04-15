package generator

import (
	"context"
	"fmt"
	"sync"

	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// GenerateConcurrent runs pipeline.Generate on N goroutines in parallel,
// returning the first successful result. Cancels remaining goroutines via
// context. If opts.Concurrency <= 1, it calls pipeline.Generate directly.
func GenerateConcurrent(pipeline PipelineStrategy, gridSize, markersPerUnit int, opts GenerateOpts) (*model.Puzzle, error) {
	if opts.Concurrency <= 1 {
		return pipeline.Generate(gridSize, markersPerUnit, opts)
	}

	parentCtx := opts.Ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	type result struct {
		puzzle *model.Puzzle
		err    error
	}

	resultCh := make(chan result, opts.Concurrency)
	var wg sync.WaitGroup

	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			workerOpts := GenerateOpts{
				Timeout:     opts.Timeout,
				Ctx:         ctx,
				RegionOpts:  opts.RegionOpts,
				Deducible:   opts.Deducible,
				Concurrency: 1, // prevent nested concurrency
			}

			puzzle, err := pipeline.Generate(gridSize, markersPerUnit, workerOpts)
			select {
			case resultCh <- result{puzzle: puzzle, err: err}:
			case <-ctx.Done():
			}
		}()
	}

	// Close the channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var lastErr error
	for res := range resultCh {
		if res.err == nil {
			cancel() // stop remaining goroutines
			return res.puzzle, nil
		}
		lastErr = res.err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("generating puzzle concurrently: all %d goroutines failed after %v", opts.Concurrency, opts.Timeout)
}