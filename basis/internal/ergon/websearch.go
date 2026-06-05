package ergon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const braveSearchURL = "https://api.search.brave.com/res/v1/web/search"

// WebSearch queries the Brave Search API and returns the top results
// as a numbered plaintext list for Claude to reason over.
type WebSearch struct {
	apiKey string
	http   *http.Client
}

func NewWebSearch(apiKey string) *WebSearch {
	return &WebSearch{apiKey: apiKey, http: &http.Client{}}
}

func (w *WebSearch) Name() string { return "web_search" }

func (w *WebSearch) Description() string {
	return "Search the web for current information. Use when you need up-to-date facts, recent events, or anything that may have changed."
}

func (w *WebSearch) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		"required": []string{"query"},
	}
}

func (w *WebSearch) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("web_search: invalid input: %w", err)
	}
	if params.Query == "" {
		return "", fmt.Errorf("web_search: query is required")
	}
	if w.apiKey == "" {
		return "", fmt.Errorf("web_search: BRAVE_API_KEY not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		braveSearchURL+"?q="+url.QueryEscape(params.Query)+"&count=5", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", w.apiKey)

	resp, err := w.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("web_search: status %d", resp.StatusCode)
	}

	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("web_search: decode: %w", err)
	}
	if len(result.Web.Results) == 0 {
		return "No results found.", nil
	}

	var sb strings.Builder
	for i, r := range result.Web.Results {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n", i+1, r.Title, r.URL, r.Description)
	}
	return strings.TrimSpace(sb.String()), nil
}
