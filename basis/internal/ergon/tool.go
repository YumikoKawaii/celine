package ergon

import (
	"context"
	"encoding/json"
)

// Tool is the interface every capability must implement.
// Register with a Registry and Claude sees it automatically.
type Tool interface {
	Name() string
	Description() string
	// Schema returns the JSON schema for the tool's input object.
	// Must include at minimum "type": "object" and "properties".
	Schema() map[string]any
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}
