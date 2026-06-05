package graphe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	voyageURL   = "https://api.voyageai.com/v1/embeddings"
	voyageModel = "voyage-3-lite"
)

// Embedder produces a vector for a text string.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VoyageClient calls the Voyage AI embeddings API (voyage-3-lite, 512 dims).
type VoyageClient struct {
	apiKey string
	http   *http.Client
}

func NewVoyageClient(apiKey string) *VoyageClient {
	return &VoyageClient{apiKey: apiKey, http: &http.Client{}}
}

// Embed returns a 512-dim embedding for text.
// inputType should be "document" when indexing, "query" when recalling.
func (c *VoyageClient) embed(ctx context.Context, text, inputType string) ([]float32, error) {
	body, err := json.Marshal(map[string]any{
		"input":      []string{text},
		"model":      voyageModel,
		"input_type": inputType,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var e struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return nil, fmt.Errorf("voyage: status %d: %s", resp.StatusCode, e.Detail)
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("voyage: decode response: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("voyage: empty embedding in response")
	}
	return out.Data[0].Embedding, nil
}

// Embed satisfies the Embedder interface — used by the graphe worker for indexing.
func (c *VoyageClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text, "document")
}

// EmbedQuery produces a query-optimised embedding — used by the recall tier.
func (c *VoyageClient) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text, "query")
}
