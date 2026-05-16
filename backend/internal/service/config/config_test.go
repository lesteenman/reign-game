package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/config"
)

type fakeStore struct {
	getConfig      func(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
	putConfig      func(ctx context.Context, c *repository.ConfigRecord) error
	createConfig   func(ctx context.Context, c *repository.ConfigRecord) error
	getAllConfigs  func(ctx context.Context) ([]repository.ConfigRecord, error)
	lastPutRecord  *repository.ConfigRecord
	lastCreateRec  *repository.ConfigRecord
}

func (f *fakeStore) GetConfig(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error) {
	return f.getConfig(ctx, size, mode)
}
func (f *fakeStore) PutConfig(ctx context.Context, c *repository.ConfigRecord) error {
	f.lastPutRecord = c
	return f.putConfig(ctx, c)
}
func (f *fakeStore) CreateConfig(ctx context.Context, c *repository.ConfigRecord) error {
	f.lastCreateRec = c
	return f.createConfig(ctx, c)
}
func (f *fakeStore) GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error) {
	return f.getAllConfigs(ctx)
}

func TestUpdate_HappyPathForwardsToPut(t *testing.T) {
	// Arrange
	existing := &repository.ConfigRecord{Size: 7, Mode: "standard", Threshold: 5, Enabled: true}
	store := &fakeStore{
		getConfig:    func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return existing, nil },
		putConfig:    func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
	}
	svc := config.New(store)
	updated := &repository.ConfigRecord{Size: 7, Mode: "standard", Threshold: 10, Enabled: true}

	// Act
	err := svc.Update(context.Background(), updated)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastPutRecord != updated {
		t.Errorf("PutConfig record = %+v, want pointer-equal to updated", store.lastPutRecord)
	}
}

func TestUpdate_ReturnsNotFoundWhenMissing(t *testing.T) {
	// Arrange
	store := &fakeStore{
		getConfig:    func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:    func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
	}
	svc := config.New(store)

	// Act
	err := svc.Update(context.Background(), &repository.ConfigRecord{Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("err = %v, want config.ErrNotFound", err)
	}
	if store.lastPutRecord != nil {
		t.Error("PutConfig must not be called when GetConfig returns nil")
	}
}

func TestUpdate_WrapsGetConfigError(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb get exploded")
	store := &fakeStore{
		getConfig:    func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, sentinel },
		putConfig:    func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
	}
	svc := config.New(store)

	// Act
	err := svc.Update(context.Background(), &repository.ConfigRecord{Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
}

func TestUpdate_WrapsPutConfigError(t *testing.T) {
	// Arrange
	existing := &repository.ConfigRecord{Size: 7, Mode: "standard"}
	sentinel := errors.New("ddb put exploded")
	store := &fakeStore{
		getConfig:    func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return existing, nil },
		putConfig:    func(_ context.Context, _ *repository.ConfigRecord) error { return sentinel },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
	}
	svc := config.New(store)

	// Act
	err := svc.Update(context.Background(), &repository.ConfigRecord{Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
}

func TestCreate_HappyPathForwards(t *testing.T) {
	// Arrange
	store := &fakeStore{
		getConfig:    func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:    func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
	}
	svc := config.New(store)
	rec := &repository.ConfigRecord{Size: 7, Mode: "standard", Threshold: 5, Enabled: true}

	// Act
	err := svc.Create(context.Background(), rec)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastCreateRec != rec {
		t.Errorf("CreateConfig record = %+v, want pointer-equal to input", store.lastCreateRec)
	}
}

func TestCreate_TranslatesAlreadyExists(t *testing.T) {
	// Arrange
	store := &fakeStore{
		getConfig: func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error {
			return &repository.ConfigAlreadyExistsError{Size: 7, Mode: "standard"}
		},
	}
	svc := config.New(store)

	// Act
	err := svc.Create(context.Background(), &repository.ConfigRecord{Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, config.ErrAlreadyExists) {
		t.Errorf("err = %v, want config.ErrAlreadyExists", err)
	}
}

func TestCreate_WrapsOtherErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb create exploded")
	store := &fakeStore{
		getConfig:    func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:    func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig: func(_ context.Context, _ *repository.ConfigRecord) error { return sentinel },
	}
	svc := config.New(store)

	// Act
	err := svc.Create(context.Background(), &repository.ConfigRecord{Size: 7, Mode: "standard"})

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
}

func TestListEnabledModes_FiltersOutDisabled(t *testing.T) {
	// Arrange
	all := []repository.ConfigRecord{
		{Size: 7, Mode: "standard", Enabled: true, Threshold: 5},
		{Size: 7, Mode: "double", Enabled: false, Threshold: 5},
		{Size: 9, Mode: "standard", Enabled: true, Threshold: 8},
	}
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return all, nil },
	}
	svc := config.New(store)

	// Act
	enabled, err := svc.ListEnabledModes(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enabled) != 2 {
		t.Fatalf("len = %d, want 2", len(enabled))
	}
	if enabled[0].Size != 7 || enabled[0].Mode != "standard" {
		t.Errorf("enabled[0] = %+v", enabled[0])
	}
	if enabled[1].Size != 9 || enabled[1].Mode != "standard" {
		t.Errorf("enabled[1] = %+v", enabled[1])
	}
}

func TestListEnabledModes_WrapsErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb list exploded")
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return nil, sentinel },
	}
	svc := config.New(store)

	// Act
	_, err := svc.ListEnabledModes(context.Background())

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
}
