package replenish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/message"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
	"github.com/eriksteenman/reign-game/backend/internal/service/replenish"
)

// fakeConfigReader stubs ConfigReader (both GetAllConfigs + GetConfig).
type fakeConfigReader struct {
	all       []configsvc.ConfigView
	allErr    error
	byKey     map[string]*configsvc.ConfigView
	getCfgErr error
}

func (f *fakeConfigReader) GetAllConfigs(_ context.Context) ([]configsvc.ConfigView, error) {
	return f.all, f.allErr
}

func (f *fakeConfigReader) GetConfig(_ context.Context, size int, mode string) (*configsvc.ConfigView, error) {
	if f.getCfgErr != nil {
		return nil, f.getCfgErr
	}
	key := keyFor(size, mode)
	return f.byKey[key], nil
}

func keyFor(size int, mode string) string {
	return mode + "/" + sizeStr(size)
}

func sizeStr(size int) string {
	switch size {
	case 5:
		return "5"
	case 7:
		return "7"
	case 9:
		return "9"
	default:
		return "?"
	}
}

type fakePoolCounter struct {
	counts map[string]int
	err    error
}

func (f *fakePoolCounter) CountReady(_ context.Context, size int, mode string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.counts[keyFor(size, mode)], nil
}

type fakePublisher struct {
	published []message.GenerationRequest
	err       error
}

func (f *fakePublisher) PublishGenerationRequest(_ context.Context, req *message.GenerationRequest) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, *req)
	return nil
}

type fakeClaimer struct {
	claim    bool
	claimErr error
	calls    int
}

func (f *fakeClaimer) TryClaimAutoReplenish(_ context.Context, _ int, _ string, _ time.Time, _ time.Duration) (bool, error) {
	f.calls++
	return f.claim, f.claimErr
}

func enabledConfig(size int, mode string, threshold, maxAttempts int) configsvc.ConfigView {
	return configsvc.ConfigView{
		Size:        size,
		Mode:        mode,
		Threshold:   threshold,
		Enabled:     true,
		MaxAttempts: maxAttempts,
	}
}

// --- Sweep ----------------------------------------------------------------

func TestSweep_EmptyConfigs_NothingTriggered(t *testing.T) {
	// Arrange
	deps := replenish.SweepDeps{
		Configs:   &fakeConfigReader{all: []configsvc.ConfigView{}},
		Counter:   &fakePoolCounter{counts: map[string]int{}},
		Publisher: &fakePublisher{},
	}

	// Act
	got, err := replenish.Sweep(context.Background(), deps, replenish.Filter{})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Triggered) != 0 || len(got.Skipped) != 0 {
		t.Errorf("expected empty result, got triggered=%d skipped=%d", len(got.Triggered), len(got.Skipped))
	}
}

func TestSweep_MixedBelowAndAbove(t *testing.T) {
	// Arrange — 5#standard at threshold (skipped), 7#standard below (triggered).
	pub := &fakePublisher{}
	deps := replenish.SweepDeps{
		Configs: &fakeConfigReader{all: []configsvc.ConfigView{
			enabledConfig(5, "standard", 3, 0),
			enabledConfig(7, "standard", 3, 10),
		}},
		Counter: &fakePoolCounter{counts: map[string]int{
			keyFor(5, "standard"): 3,
			keyFor(7, "standard"): 1,
		}},
		Publisher: pub,
	}

	// Act
	got, err := replenish.Sweep(context.Background(), deps, replenish.Filter{})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Triggered) != 1 || got.Triggered[0].Size != 7 || got.Triggered[0].Count != 2 {
		t.Errorf("triggered = %+v", got.Triggered)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Size != 5 || got.Skipped[0].Ready != 3 {
		t.Errorf("skipped = %+v", got.Skipped)
	}
	if len(pub.published) != 2 {
		t.Errorf("published = %d, want 2", len(pub.published))
	}
	if pub.published[0].MaxAttempts != 10 {
		t.Errorf("MaxAttempts = %d, want 10", pub.published[0].MaxAttempts)
	}
}

func TestSweep_FilterBySize(t *testing.T) {
	// Arrange
	pub := &fakePublisher{}
	deps := replenish.SweepDeps{
		Configs: &fakeConfigReader{all: []configsvc.ConfigView{
			enabledConfig(5, "standard", 3, 0),
			enabledConfig(7, "standard", 3, 0),
			enabledConfig(9, "standard", 3, 0),
		}},
		Counter: &fakePoolCounter{counts: map[string]int{
			keyFor(7, "standard"): 0,
		}},
		Publisher: pub,
	}

	// Act
	got, err := replenish.Sweep(context.Background(), deps, replenish.Filter{Size: 7})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Triggered) != 1 || got.Triggered[0].Size != 7 {
		t.Errorf("triggered = %+v", got.Triggered)
	}
	if len(pub.published) != 3 {
		t.Errorf("published = %d, want 3", len(pub.published))
	}
}

func TestSweep_FilterByMode(t *testing.T) {
	// Arrange
	pub := &fakePublisher{}
	deps := replenish.SweepDeps{
		Configs: &fakeConfigReader{all: []configsvc.ConfigView{
			enabledConfig(7, "standard", 3, 0),
			enabledConfig(7, "double", 3, 0),
		}},
		Counter: &fakePoolCounter{counts: map[string]int{
			keyFor(7, "standard"): 0,
		}},
		Publisher: pub,
	}

	// Act
	got, err := replenish.Sweep(context.Background(), deps, replenish.Filter{Mode: "standard"})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Triggered) != 1 || got.Triggered[0].Mode != "standard" {
		t.Errorf("triggered = %+v", got.Triggered)
	}
}

func TestSweep_DisabledConfigsSkipped(t *testing.T) {
	// Arrange
	disabled := enabledConfig(7, "standard", 3, 0)
	disabled.Enabled = false
	pub := &fakePublisher{}
	deps := replenish.SweepDeps{
		Configs:   &fakeConfigReader{all: []configsvc.ConfigView{disabled}},
		Counter:   &fakePoolCounter{counts: map[string]int{}},
		Publisher: pub,
	}

	// Act
	got, err := replenish.Sweep(context.Background(), deps, replenish.Filter{})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Triggered) != 0 || len(got.Skipped) != 0 {
		t.Errorf("disabled config must not appear in either list; got %+v", got)
	}
}

func TestSweep_PublisherErrorMidSweep(t *testing.T) {
	// Arrange
	pub := &fakePublisher{err: errors.New("sqs down")}
	deps := replenish.SweepDeps{
		Configs: &fakeConfigReader{all: []configsvc.ConfigView{
			enabledConfig(5, "standard", 3, 0),
		}},
		Counter: &fakePoolCounter{counts: map[string]int{
			keyFor(5, "standard"): 0,
		}},
		Publisher: pub,
	}

	// Act
	_, err := replenish.Sweep(context.Background(), deps, replenish.Filter{})

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSweep_ConfigReadError(t *testing.T) {
	// Arrange
	deps := replenish.SweepDeps{
		Configs:   &fakeConfigReader{allErr: errors.New("ddb down")},
		Counter:   &fakePoolCounter{},
		Publisher: &fakePublisher{},
	}

	// Act
	_, err := replenish.Sweep(context.Background(), deps, replenish.Filter{})

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSweep_CountReadyError(t *testing.T) {
	// Arrange
	deps := replenish.SweepDeps{
		Configs: &fakeConfigReader{all: []configsvc.ConfigView{
			enabledConfig(5, "standard", 3, 0),
		}},
		Counter:   &fakePoolCounter{err: errors.New("ddb down")},
		Publisher: &fakePublisher{},
	}

	// Act
	_, err := replenish.Sweep(context.Background(), deps, replenish.Filter{})

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- TryReactiveTopUp ---------------------------------------------------

func TestTryReactiveTopUp_ClaimWins_PublishesThreshold(t *testing.T) {
	// Arrange
	pub := &fakePublisher{}
	cfg := enabledConfig(9, "standard", 5, 12)
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(9, "standard"): &cfg}},
		Claimer:   &fakeClaimer{claim: true},
		Publisher: pub,
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.published) != 5 {
		t.Errorf("published = %d, want 5", len(pub.published))
	}
	if pub.published[0].Size != 9 || pub.published[0].Mode != "standard" || pub.published[0].MaxAttempts != 12 {
		t.Errorf("first message = %+v", pub.published[0])
	}
}

func TestTryReactiveTopUp_ClaimLoses_NoPublish(t *testing.T) {
	// Arrange
	pub := &fakePublisher{}
	cfg := enabledConfig(9, "standard", 5, 0)
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(9, "standard"): &cfg}},
		Claimer:   &fakeClaimer{claim: false},
		Publisher: pub,
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if !errors.Is(err, replenish.ErrSkippedDebounced) {
		t.Fatalf("expected ErrSkippedDebounced, got %v", err)
	}
	if len(pub.published) != 0 {
		t.Errorf("published = %d, want 0", len(pub.published))
	}
}

func TestTryReactiveTopUp_MissingConfig_NoOp(t *testing.T) {
	// Arrange
	pub := &fakePublisher{}
	claimer := &fakeClaimer{claim: true}
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{}}, // not present
		Claimer:   claimer,
		Publisher: pub,
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimer.calls != 0 {
		t.Errorf("Claimer must not be called when config is missing")
	}
	if len(pub.published) != 0 {
		t.Errorf("published = %d, want 0", len(pub.published))
	}
}

func TestTryReactiveTopUp_DisabledConfig_NoOp(t *testing.T) {
	// Arrange
	pub := &fakePublisher{}
	claimer := &fakeClaimer{claim: true}
	cfg := enabledConfig(9, "standard", 5, 0)
	cfg.Enabled = false
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(9, "standard"): &cfg}},
		Claimer:   claimer,
		Publisher: pub,
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimer.calls != 0 {
		t.Errorf("Claimer must not be called when config is disabled")
	}
	if len(pub.published) != 0 {
		t.Errorf("published = %d, want 0", len(pub.published))
	}
}

func TestTryReactiveTopUp_ConfigReadError(t *testing.T) {
	// Arrange
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{getCfgErr: errors.New("ddb down")},
		Claimer:   &fakeClaimer{},
		Publisher: &fakePublisher{},
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, replenish.ErrSkippedDebounced) {
		t.Errorf("read errors must not surface as debounced")
	}
}

func TestTryReactiveTopUp_ClaimerError(t *testing.T) {
	// Arrange
	cfg := enabledConfig(9, "standard", 5, 0)
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(9, "standard"): &cfg}},
		Claimer:   &fakeClaimer{claimErr: errors.New("ddb down")},
		Publisher: &fakePublisher{},
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTryReactiveTopUp_PublisherErrorAfterClaim(t *testing.T) {
	// Arrange
	cfg := enabledConfig(9, "standard", 5, 0)
	deps := replenish.ReactiveDeps{
		Configs:   &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(9, "standard"): &cfg}},
		Claimer:   &fakeClaimer{claim: true},
		Publisher: &fakePublisher{err: errors.New("sqs down")},
		Window:    60 * time.Second,
		Clock:     time.Now,
	}

	// Act
	err := replenish.TryReactiveTopUp(context.Background(), deps, 9, "standard")

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- NewAsyncHook ---------------------------------------------------------

func TestNewAsyncHook_NilDepsReturnsNilHook(t *testing.T) {
	// Arrange — nil Configs makes the hook nil so local-dev without SQS works.
	deps := replenish.ReactiveDeps{Configs: nil, Claimer: &fakeClaimer{}, Publisher: &fakePublisher{}}

	// Act
	hook := replenish.NewAsyncHook(deps, "test")

	// Assert
	if hook != nil {
		t.Fatal("nil Configs should produce nil hook")
	}
}

func TestNewAsyncHook_DispatchesGoroutineThatPublishesOnClaim(t *testing.T) {
	// Arrange — claim wins; goroutine should publish Threshold messages.
	cfg := enabledConfig(9, "standard", 3, 100)
	cfgs := &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(9, "standard"): &cfg}}
	claimer := &fakeClaimer{claim: true}
	pub := &fakePublisher{}
	deps := replenish.ReactiveDeps{Configs: cfgs, Claimer: claimer, Publisher: pub}

	// Act
	hook := replenish.NewAsyncHook(deps, "test")
	if hook == nil {
		t.Fatal("hook unexpectedly nil")
	}
	hook(9, "standard")

	// Assert — goroutine completes well within the publisher fake's no-op latency.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(pub.published) == 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pub.published) != 3 {
		t.Fatalf("published = %d, want 3", len(pub.published))
	}
}

func TestNewAsyncHook_AppliesDefaultWindowAndClockWhenZero(t *testing.T) {
	// Arrange — zero Window/Clock should fall back to DefaultWindow/time.Now.
	cfg := enabledConfig(5, "standard", 1, 0)
	cfgs := &fakeConfigReader{byKey: map[string]*configsvc.ConfigView{keyFor(5, "standard"): &cfg}}
	claimer := &fakeClaimer{claim: true}
	pub := &fakePublisher{}
	deps := replenish.ReactiveDeps{Configs: cfgs, Claimer: claimer, Publisher: pub}

	// Act
	hook := replenish.NewAsyncHook(deps, "")
	hook(5, "standard")

	// Assert — claimer must have been invoked (defaults wired through).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if claimer.calls == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if claimer.calls != 1 {
		t.Fatalf("claimer calls = %d, want 1", claimer.calls)
	}
}
