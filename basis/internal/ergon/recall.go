package ergon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YumikoKawaii/celine/basis/internal/arche"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// ownerKey carries the calling client's prosopon id into a tool's Execute.
// The tool loop stamps it (see agent.Chat); recall reads it to scope its
// vector search to that client's memories only.
type ownerKey struct{}

// WithOwner returns ctx carrying ownerProsoponId for owner-scoped tools.
func WithOwner(ctx context.Context, ownerProsoponId int64) context.Context {
	return context.WithValue(ctx, ownerKey{}, ownerProsoponId)
}

func ownerFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(ownerKey{}).(int64)
	return id, ok
}

// embedder turns the model's query text into a vector.
type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// store is the recall read path — a filtered vector search (§11.2, §12.5).
type store interface {
	Search(ctx context.Context, ownerProsoponId int64, embedding []float32, k int) ([]mneme.MemoryHit, error)
}

// Recall is the agentic tier-3 memory tool (§12.5): Claude calls it when it
// knows it's missing context, choosing the query and k. The result lands in
// history, so later turns within the cache TTL get it for free.
type Recall struct {
	embedder embedder
	store    store
}

func NewRecall(e embedder, s store) *Recall {
	return &Recall{embedder: e, store: s}
}

func (r *Recall) Name() string { return "recall" }

func (r *Recall) Description() string {
	return "Search your long-term memory of past conversations with this person. " +
		"Use when you need context that isn't in the current thread — preferences, " +
		"facts they told you before, earlier decisions. Refine the query and raise k " +
		"if the first results miss."
}

func (r *Recall) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "What to look for, phrased as the topic or fact you're recalling.",
			},
			"k": map[string]any{
				"type":        "integer",
				"description": "How many memories to retrieve (default 5, max 20).",
			},
		},
		"required": []string{"query"},
	}
}

func (r *Recall) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		K     int    `json:"k"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("recall: invalid input: %w", err)
	}
	if strings.TrimSpace(params.Query) == "" {
		return "", fmt.Errorf("recall: query is required")
	}
	owner, ok := ownerFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("recall: no caller identity in context")
	}
	switch {
	case params.K <= 0:
		params.K = 5
	case params.K > 20:
		params.K = 20
	}

	vec, err := r.embedder.Embed(ctx, params.Query)
	if err != nil {
		return "", fmt.Errorf("recall: embed: %w", err)
	}
	hits, err := r.store.Search(ctx, owner, vec, params.K)
	if err != nil {
		return "", fmt.Errorf("recall: search: %w", err)
	}
	if len(hits) == 0 {
		return "No relevant memories found.", nil
	}

	var sb strings.Builder
	for i, h := range hits {
		who := "them"
		if h.ProsoponId == arche.CelineProsoponID {
			who = "you"
		}
		fmt.Fprintf(&sb, "%d. (%s, similarity %.2f) %s\n", i+1, who, h.Similarity, strings.TrimSpace(h.Content))
	}
	return strings.TrimSpace(sb.String()), nil
}
