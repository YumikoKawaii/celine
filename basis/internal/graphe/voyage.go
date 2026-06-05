package graphe

import "context"

// Embedder produces a vector for a text string.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VoyageClient calls the Voyage AI embeddings API (voyage-3-lite, 512 dims).
// Stubbed until VOYAGE_API_KEY is wired — returns a zero vector so the worker
// pipeline and DB writes can be validated end-to-end first.
type VoyageClient struct {
	apiKey string
}

func NewVoyageClient(apiKey string) *VoyageClient {
	return &VoyageClient{apiKey: apiKey}
}

func (c *VoyageClient) Embed(_ context.Context, _ string) ([]float32, error) {
	// TODO: POST https://api.voyageai.com/v1/embeddings, model=voyage-3-lite
	return make([]float32, 512), nil
}
