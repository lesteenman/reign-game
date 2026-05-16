package handler

import (
	"fmt"
	"net/http"

	"github.com/eriksteenman/reign-game/backend/internal/mode"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// The handler layer speaks explicit DTOs for CONFIG items.
// repository.ConfigRecord is the domain shape stored in DynamoDB; these
// DTOs shape the HTTP request and response surfaces. Mappers at the
// boundary translate between them so no HTTP concern leaks into the
// repo and no DynamoDB tag leaks into the API.

// ConfigBody is the subset of a CONFIG row that carries configuration
// rather than identity (size, mode). Shared by every response and
// request payload that carries config values — flat views, nested
// combo entries, and update bodies.
type ConfigBody struct {
	Threshold   int  `json:"threshold"`
	Enabled     bool `json:"enabled"`
	MaxAttempts int  `json:"maxAttempts,omitempty"`
}

// ConfigView is the flat JSON shape returned by PUT /api/admin/config
// and POST /api/admin/config. Identity fields (size, mode) live
// alongside the ConfigBody so callers get a complete picture.
type ConfigView struct {
	Size int    `json:"size"`
	Mode string `json:"mode"`
	ConfigBody
}

// ConfigCreateRequest is the JSON body of POST /api/admin/config.
// Carries size and mode in the body because the endpoint has no URL
// path parameters.
type ConfigCreateRequest struct {
	Size int    `json:"size"`
	Mode string `json:"mode"`
	ConfigBody
}

// ConfigUpdateRequest is the JSON body of PUT /api/admin/config/{size}/{mode}.
// Size and mode live in the URL path, not the body.
type ConfigUpdateRequest struct {
	ConfigBody
}

// configBodyFrom builds a ConfigBody from a repository.ConfigRecord.
// Used by any response that nests config (e.g., ComboStatus).
func configBodyFrom(rec *repository.ConfigRecord) ConfigBody {
	return ConfigBody{
		Threshold:   rec.Threshold,
		Enabled:     rec.Enabled,
		MaxAttempts: rec.MaxAttempts,
	}
}

// configViewFrom builds a ConfigView from a repository.ConfigRecord.
// The view carries size and mode as top-level fields because the
// repository stores them as DynamoDB keys, not record attributes.
func configViewFrom(rec *repository.ConfigRecord) ConfigView {
	return ConfigView{
		Size:       rec.Size,
		Mode:       rec.Mode,
		ConfigBody: configBodyFrom(rec),
	}
}

// toRecord maps a ConfigCreateRequest to a repository.ConfigRecord.
// Size and mode come from the request body for this endpoint.
func (req *ConfigCreateRequest) toRecord() *repository.ConfigRecord {
	return &repository.ConfigRecord{
		Size:        req.Size,
		Mode:        req.Mode,
		Threshold:   req.Threshold,
		Enabled:     req.Enabled,
		MaxAttempts: req.MaxAttempts,
	}
}

// toRecord maps a ConfigUpdateRequest to a repository.ConfigRecord
// using size and mode supplied by the URL path.
func (req *ConfigUpdateRequest) toRecord(size int, modeName string) *repository.ConfigRecord {
	return &repository.ConfigRecord{
		Size:        size,
		Mode:        modeName,
		Threshold:   req.Threshold,
		Enabled:     req.Enabled,
		MaxAttempts: req.MaxAttempts,
	}
}

// validateConfigBody checks the fields shared by both create and update.
// Returns (0, "", "") on success.
func validateConfigBody(body *ConfigBody) (status int, errCode, errMsg string) {
	if body.Threshold < 1 || body.Threshold > maxConfigThreshold {
		return http.StatusBadRequest, "invalid_params",
			fmt.Sprintf("threshold must be between 1 and %d", maxConfigThreshold)
	}
	if body.MaxAttempts < 0 {
		return http.StatusBadRequest, "invalid_params",
			"maxAttempts must be >= 0"
	}
	return 0, "", ""
}

// validateSize rejects sizes outside the handler's accepted range.
func validateSize(size int) (status int, errCode, errMsg string) {
	if size < 3 || size > 15 {
		return http.StatusBadRequest, "invalid_params",
			"size must be between 3 and 15"
	}
	return 0, "", ""
}

// validateMode rejects modes outside {standard, double}.
func validateMode(modeName string) (status int, errCode, errMsg string) {
	if !mode.IsValid(modeName) {
		return http.StatusBadRequest, "invalid_params",
			"mode must be 'standard' or 'double'"
	}
	return 0, "", ""
}
