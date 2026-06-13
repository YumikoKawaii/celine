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

// embedder turns query text into a vector for recall search (§12.5 tier 2).
type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// memories is the recall read path — a filtered vector search over this
// client's indexed messages (§11.2, §12.5).
type memories interface {
	Search(ctx context.Context, ownerProsoponId int64, embedding []float32, k int) ([]mneme.MemoryHit, error)
}

type toolRunner interface {
	Defs() []llm.ToolDef
	Execute(ctx context.Context, name string, input json.RawMessage) (string, error)
}
