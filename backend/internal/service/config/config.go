// Package config is the application service for managing puzzle
// generation configs (the CONFIG row family) — admin Create/Update
// and the public modes-listing read path.
package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ErrNotFound is returned by Service.Update when the addressed config
// row does not exist. Handlers translate it to 404 without ever
// importing repository internals.
var ErrNotFound = errors.New("config not found")

// ErrAlreadyExists is returned by Service.Create when the addressed
// (size, mode) pair is already present. Handlers translate it to 409.
var ErrAlreadyExists = errors.New("config already exists")

// Store is the persistence surface used by Service.
type Store interface {
	GetConfig(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
	PutConfig(ctx context.Context, config *repository.ConfigRecord) error
	CreateConfig(ctx context.Context, config *repository.ConfigRecord) error
	GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
}

// ConfigView is the read-side projection of a (size, mode) configuration
// returned by ListEnabledModes.
type ConfigView struct {
	Size        int
	Mode        string
	Threshold   int
	Enabled     bool
	MaxAttempts int
}

// CreateInput is the plain-field input to Service.Create.
type CreateInput struct {
	Size        int
	Mode        string
	Threshold   int
	Enabled     bool
	MaxAttempts int
}

// UpdateInput is the plain-field input to Service.Update. Identity
// (size, mode) is supplied separately from the URL path.
type UpdateInput struct {
	Size        int
	Mode        string
	Threshold   int
	Enabled     bool
	MaxAttempts int
}

// Service handles config reads, creates, and updates.
type Service struct {
	store Store
}

// New constructs a Service.
func New(store Store) *Service {
	return &Service{store: store}
}

// Update writes the given config record. Returns ErrNotFound when the
// addressed (size, mode) pair has no existing row — the admin "edit"
// flow requires an existing target.
func (s *Service) Update(ctx context.Context, in UpdateInput) error {
	existing, err := s.store.GetConfig(ctx, in.Size, in.Mode)
	if err != nil {
		return fmt.Errorf("checking existing config: %w", err)
	}
	if existing == nil {
		return ErrNotFound
	}
	record := &repository.ConfigRecord{
		Size:        in.Size,
		Mode:        in.Mode,
		Threshold:   in.Threshold,
		Enabled:     in.Enabled,
		MaxAttempts: in.MaxAttempts,
	}
	if err := s.store.PutConfig(ctx, record); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// Create writes a new config record. Returns ErrAlreadyExists when
// the addressed (size, mode) pair is already present.
func (s *Service) Create(ctx context.Context, in CreateInput) error {
	record := &repository.ConfigRecord{
		Size:        in.Size,
		Mode:        in.Mode,
		Threshold:   in.Threshold,
		Enabled:     in.Enabled,
		MaxAttempts: in.MaxAttempts,
	}
	if err := s.store.CreateConfig(ctx, record); err != nil {
		var alreadyExists *repository.ConfigAlreadyExistsError
		if errors.As(err, &alreadyExists) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("creating config: %w", err)
	}
	return nil
}

// ListEnabledModes returns every enabled config, ordered as returned
// by the store (the public modes endpoint forwards this verbatim).
func (s *Service) ListEnabledModes(ctx context.Context) ([]ConfigView, error) {
	all, err := s.store.GetAllConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing configs: %w", err)
	}
	enabled := make([]ConfigView, 0, len(all))
	for _, c := range all {
		if c.Enabled {
			enabled = append(enabled, ConfigView{
				Size:        c.Size,
				Mode:        c.Mode,
				Threshold:   c.Threshold,
				Enabled:     c.Enabled,
				MaxAttempts: c.MaxAttempts,
			})
		}
	}
	return enabled, nil
}
