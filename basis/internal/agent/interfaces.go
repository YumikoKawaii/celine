package agent

import (
	"context"
	"encoding/json"

	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// EventSink receives all events produced during a single Chat turn.
type EventSink interface {
	Bubble(seq int32, text string) error
	ToolCall(id, name, inputJSON string) error
	ToolResult(id, output string, isError bool) error
}

type brain interface {
	Chat(ctx context.Context, system string, history []llm.Message, tools []llm.ToolDef, emit func(bubble string)) (llm.Turn, error)
}

type prosopons interface {
	Get(ctx context.Context, parameters mneme.Scope) (mneme.Prosopon, error)
}

type conversations interface {
	GetOrCreate(ctx context.Context, filter mneme.KataProsopon) (*mneme.Conversation, error)
}

type messages interface {
	Create(ctx context.Context, message *mneme.Message) error
	List(ctx context.Context, parameters mneme.Scope, pagination *mneme.Pagination) ([]mneme.Message, error)
}

type queue interface {
	Enqueue(ctx context.Context, topic string, message interface{}) error
}

type toolRunner interface {
	Defs() []llm.ToolDef
	Execute(ctx context.Context, name string, input json.RawMessage) (string, error)
}
