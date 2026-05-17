// Package message defines wire DTOs shared between the message
// producer (publisher) and consumer (worker). Carved out of internal/queue/
// so the layered architecture rules (handler/worker must not import
// queue) hold without losing a single source of truth for the wire
// shape.
package message

// GenerationRequest is the SQS message body the replenish service
// publishes and the generator worker consumes.
type GenerationRequest struct {
	Size        int    `json:"size"`
	Mode        string `json:"mode"`
	MaxAttempts int    `json:"maxAttempts,omitempty"`
}
