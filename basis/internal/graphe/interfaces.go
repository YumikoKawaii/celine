package graphe

import (
	"context"

	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type messages interface {
	Get(ctx context.Context, scope mneme.Scope) (mneme.Message, error)
}

type memories interface {
	Insert(ctx context.Context, memory mneme.Memory, embedding []float32) error
}

type queue interface {
	Dequeue(ctx context.Context, topic string) (interface{}, error)
}
