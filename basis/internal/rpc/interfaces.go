package rpc

import (
	"context"

	"github.com/YumikoKawaii/celine/basis/internal/agent"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// chatAgent is used by the Celine handler for streaming chat.
type chatAgent interface {
	Chat(ctx context.Context, ownerSub string, userText string, sink agent.EventSink) (int64, error)
}

// prosoponStore is the full prosopon access used by the Hermes handler
// (includes Upsert for the OAuth exchange flow).
type prosoponStore interface {
	Upsert(ctx context.Context, p *mneme.Prosopon) error
	Get(ctx context.Context, parameters mneme.Scope) (mneme.Prosopon, error)
}

// convReader is used by the Hermes handler to create the conversation at auth time
// so its ID can be embedded in the JWT.
type convReader interface {
	GetOrCreate(ctx context.Context, filter mneme.KataProsopon) (*mneme.Conversation, error)
}

// msgReader is the message access used by the Celine handler.
type msgReader interface {
	List(ctx context.Context, scope mneme.Scope, pagination *mneme.Pagination) ([]mneme.Message, error)
}
