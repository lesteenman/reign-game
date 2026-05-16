package handler

import (
	"fmt"
	"net/http"

	"github.com/eriksteenman/reign-game/backend/internal/mode"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// The handler layer speaks explicit DTOs for CONFIG items.
// configsvc.ConfigView is the service-level read projection; these
// DTOs shape the HTTP request and response surfaces. Mappers at the
// boundary translate between them so no HTTP concern leaks into the
// service and no DynamoDB tag leaks into the API.

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

// configBodyFrom builds a ConfigBody from a configsvc.ConfigView.
// Used by any response that nests config (e.g., ComboStatus).
func configBodyFrom(v configsvc.ConfigView) ConfigBody {
	return ConfigBody{
		Threshold:   v.Threshold,
		Enabled:     v.Enabled,
		MaxAttempts: v.MaxAttempts,
	}
}

// configViewFrom builds a handler ConfigView from a configsvc.ConfigView.
func configViewFrom(v configsvc.ConfigView) ConfigView {
	return ConfigView{
		Size:       v.Size,
		Mode:       v.Mode,
		ConfigBody: configBodyFrom(v),
	}
}

// toCreateInput maps a ConfigCreateRequest to a configsvc.CreateInput.
// Size and mode come from the request body for this endpoint.
func (req *ConfigCreateRequest) toCreateInput() configsvc.CreateInput {
	return configsvc.CreateInput{
		Size:        req.Size,
		Mode:        req.Mode,
		Threshold:   req.Threshold,
		Enabled:     req.Enabled,
		MaxAttempts: req.MaxAttempts,
	}
}

// toUpdateInput maps a ConfigUpdateRequest to a configsvc.UpdateInput
// using size and mode supplied by the URL path.
func (req *ConfigUpdateRequest) toUpdateInput(size int, modeName string) configsvc.UpdateInput {
	return configsvc.UpdateInput{
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
