package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/config"
)

type fakeStore struct {
	getConfig     func(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
	putConfig     func(ctx context.Context, c *repository.ConfigRecord) error
	createConfig  func(ctx context.Context, c *repository.ConfigRecord) error
	getAllConfigs func(ctx context.Context) ([]repository.ConfigRecord, error)
	lastPutRecord *repository.ConfigRecord
	lastCreateRec *repository.ConfigRecord
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
	in := config.UpdateInput{Size: 7, Mode: "standard", Threshold: 10, Enabled: true}

	// Act
	err := svc.Update(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastPutRecord == nil {
		t.Fatal("PutConfig was not called")
	}
	if store.lastPutRecord.Size != 7 || store.lastPutRecord.Mode != "standard" || store.lastPutRecord.Threshold != 10 {
		t.Errorf("PutConfig record = %+v, want {Size:7 Mode:standard Threshold:10 Enabled:true}", store.lastPutRecord)
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
	err := svc.Update(context.Background(), config.UpdateInput{Size: 7, Mode: "standard"})

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
	err := svc.Update(context.Background(), config.UpdateInput{Size: 7, Mode: "standard"})

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
	err := svc.Update(context.Background(), config.UpdateInput{Size: 7, Mode: "standard"})

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
	in := config.CreateInput{Size: 7, Mode: "standard", Threshold: 5, Enabled: true}

	// Act
	err := svc.Create(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastCreateRec == nil {
		t.Fatal("CreateConfig was not called")
	}
	if store.lastCreateRec.Size != 7 || store.lastCreateRec.Mode != "standard" || store.lastCreateRec.Threshold != 5 {
		t.Errorf("CreateConfig record = %+v, want {Size:7 Mode:standard Threshold:5 Enabled:true}", store.lastCreateRec)
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
	err := svc.Create(context.Background(), config.CreateInput{Size: 7, Mode: "standard"})

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
	err := svc.Create(context.Background(), config.CreateInput{Size: 7, Mode: "standard"})

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

func TestGetAllConfigs_ReturnsAllIncludingDisabled(t *testing.T) {
	// Arrange
	all := []repository.ConfigRecord{
		{Size: 7, Mode: "standard", Enabled: true, Threshold: 5, MaxAttempts: 10},
		{Size: 7, Mode: "double", Enabled: false, Threshold: 3, MaxAttempts: 5},
	}
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return all, nil },
	}
	svc := config.New(store)

	// Act
	got, err := svc.GetAllConfigs(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Size != 7 || got[0].Mode != "standard" || got[0].Enabled != true {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Size != 7 || got[1].Mode != "double" || got[1].Enabled != false {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestGetAllConfigs_WrapsErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb exploded")
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return nil, sentinel },
	}
	svc := config.New(store)

	// Act
	_, err := svc.GetAllConfigs(context.Background())

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
}

func TestGetConfig_ReturnsViewWhenFound(t *testing.T) {
	// Arrange
	rec := &repository.ConfigRecord{Size: 9, Mode: "standard", Threshold: 8, Enabled: true, MaxAttempts: 20}
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return rec, nil },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return nil, nil },
	}
	svc := config.New(store)

	// Act
	got, err := svc.GetConfig(context.Background(), 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil view")
	}
	if got.Size != 9 || got.Mode != "standard" || got.Threshold != 8 || got.MaxAttempts != 20 {
		t.Errorf("got = %+v", got)
	}
}

func TestGetConfig_ReturnsNilWhenMissing(t *testing.T) {
	// Arrange
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, nil },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return nil, nil },
	}
	svc := config.New(store)

	// Act
	got, err := svc.GetConfig(context.Background(), 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestGetConfig_WrapsErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb exploded")
	store := &fakeStore{
		getConfig:     func(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) { return nil, sentinel },
		putConfig:     func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		createConfig:  func(_ context.Context, _ *repository.ConfigRecord) error { return nil },
		getAllConfigs: func(_ context.Context) ([]repository.ConfigRecord, error) { return nil, nil },
	}
	svc := config.New(store)

	// Act
	_, err := svc.GetConfig(context.Background(), 9, "standard")

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
}
