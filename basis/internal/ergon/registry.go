package ergon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/YumikoKawaii/celine/basis/internal/llm"
)

// Registry holds all registered tools and dispatches Execute calls.
type Registry struct {
	tools map[string]Tool
	order []string // preserves registration order for Claude's tool list
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; !exists {
		r.order = append(r.order, t.Name())
	}
	r.tools[t.Name()] = t
}

// Defs returns tool definitions in registration order for the Claude API.
func (r *Registry) Defs() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("ergon: unknown tool %q", name)
	}
	return t.Execute(ctx, input)
}
